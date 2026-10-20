package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"sort"
	"swaplook/internal/application"
	"swaplook/internal/domain"
	"swaplook/migrations"
	"time"
)

type queryer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}
type Repository struct {
	db   queryer
	pool *pgxpool.Pool
}

func Open(ctx context.Context, url string) (*Repository, error) {
	p, e := pgxpool.New(ctx, url)
	if e != nil {
		return nil, e
	}
	if e = p.Ping(ctx); e != nil {
		p.Close()
		return nil, e
	}
	return &Repository{db: p, pool: p}, nil
}
func (r *Repository) Close() { r.pool.Close() }
func (r *Repository) Migrate(ctx context.Context) error {
	return r.Transaction(ctx, func(port application.Repository) error {
		q := port.(*Repository).db
		if _, e := q.Exec(ctx, "SELECT pg_advisory_xact_lock(819541)"); e != nil {
			return e
		}
		if _, e := q.Exec(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations(version integer PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())"); e != nil {
			return e
		}
		entries, e := migrations.Files.ReadDir(".")
		if e != nil {
			return e
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			var version int
			if _, e := fmt.Sscanf(entry.Name(), "%d_", &version); e != nil {
				continue
			}
			var exists bool
			if e = q.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)", version).Scan(&exists); e != nil {
				return e
			}
			if exists {
				continue
			}
			b, e := migrations.Files.ReadFile(entry.Name())
			if e != nil {
				return e
			}
			if _, e = q.Exec(ctx, string(b)); e != nil {
				return fmt.Errorf("migration %d: %w", version, e)
			}
		}
		return nil
	})
}
func (r *Repository) Transaction(ctx context.Context, fn func(application.Repository) error) error {
	if r.pool == nil {
		return fn(r)
	}
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if e = fn(&Repository{db: tx}); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func missing(e error) error {
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return e
}
func (r *Repository) Catalog(ctx context.Context) ([]domain.Product, error) {
	rows, e := r.db.Query(ctx, `SELECT p.id,p.name,p.price,p.currency,p.color,p.material,p.cut,p.category,p.image,jsonb_object_agg(v.size,v.quantity) FROM products p JOIN variants v ON v.product_id=p.id GROUP BY p.id ORDER BY p.id`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.Product{}
	for rows.Next() {
		var p domain.Product
		var stock []byte
		if e = rows.Scan(&p.ID, &p.Name, &p.Price, &p.Currency, &p.Color, &p.Material, &p.Cut, &p.Category, &p.Image, &stock); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(stock, &p.Stock); e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (r *Repository) Product(ctx context.Context, id, size string) (domain.Product, error) {
	var p domain.Product
	var qty int
	e := r.db.QueryRow(ctx, `SELECT p.id,p.name,p.price,p.currency,p.color,p.material,p.cut,p.category,p.image,v.quantity FROM products p JOIN variants v ON v.product_id=p.id WHERE p.id=$1 AND v.size=$2 FOR UPDATE OF p,v`, id, size).Scan(&p.ID, &p.Name, &p.Price, &p.Currency, &p.Color, &p.Material, &p.Cut, &p.Category, &p.Image, &qty)
	p.Stock = map[string]int{size: qty}
	return p, missing(e)
}
func (r *Repository) CreateSession(ctx context.Context, s domain.Session) error {
	b, e := json.Marshal(s)
	if e != nil {
		return e
	}
	_, e = r.db.Exec(ctx, "INSERT INTO sessions(id,state,expires_at) VALUES($1,$2,$3)", s.ID, b, s.ExpiresAt)
	return e
}
func (r *Repository) Session(ctx context.Context, id string, lock bool) (domain.Session, error) {
	var b []byte
	s := domain.Session{}
	q := "SELECT state FROM sessions WHERE id=$1 AND expires_at>now()"
	if lock {
		q += " FOR UPDATE"
	}
	e := r.db.QueryRow(ctx, q, id).Scan(&b)
	if e != nil {
		return s, missing(e)
	}
	e = json.Unmarshal(b, &s)
	s.ID = id
	return s, e
}
func (r *Repository) SaveSession(ctx context.Context, s domain.Session) error {
	b, e := json.Marshal(s)
	if e != nil {
		return e
	}
	tag, e := r.db.Exec(ctx, "UPDATE sessions SET state=$2 WHERE id=$1", s.ID, b)
	if e == nil && tag.RowsAffected() != 1 {
		return domain.ErrNotFound
	}
	return e
}
func (r *Repository) DeleteSession(ctx context.Context, id string) error {
	_, e := r.db.Exec(ctx, "DELETE FROM sessions WHERE id=$1", id)
	return e
}
func (r *Repository) SetStock(ctx context.Context, id, size string, qty int) error {
	if !domain.ValidSize(size) || qty < 0 || qty > 10000 {
		return domain.ErrInvalid
	}
	tag, e := r.db.Exec(ctx, "UPDATE variants SET quantity=$3 WHERE product_id=$1 AND size=$2", id, size, qty)
	if e == nil && tag.RowsAffected() != 1 {
		return domain.ErrNotFound
	}
	return e
}
func (r *Repository) Summary(ctx context.Context) (domain.Summary, error) {
	out := domain.Summary{Decisions: []domain.Decision{}, Demo: true}
	rows, e := r.db.Query(ctx, "SELECT state FROM sessions ORDER BY created_at DESC")
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var b []byte
		var s domain.Session
		if e = rows.Scan(&b); e != nil {
			return out, e
		}
		if e = json.Unmarshal(b, &s); e != nil {
			return out, e
		}
		out.Sessions++
		if s.Decision != nil {
			out.Decisions = append(out.Decisions, *s.Decision)
			if s.Decision.Kind == "selected" {
				out.Choices++
			} else {
				out.Rejections++
			}
		}
	}
	return out, rows.Err()
}
func (r *Repository) AddPhoto(ctx context.Context, p domain.Photo) error {
	_, e := r.db.Exec(ctx, "INSERT INTO photos(id,session_id,created_at) VALUES($1,$2,$3)", p.ID, p.SessionID, p.CreatedAt)
	return e
}
func (r *Repository) Photos(ctx context.Context, id string) ([]domain.Photo, error) {
	rows, e := r.db.Query(ctx, "SELECT id,created_at FROM photos WHERE session_id=$1", id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.Photo{}
	for rows.Next() {
		p := domain.Photo{SessionID: id}
		if e = rows.Scan(&p.ID, &p.CreatedAt); e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (r *Repository) DeleteMedia(ctx context.Context, id string) error {
	if _, e := r.db.Exec(ctx, "DELETE FROM photos WHERE session_id=$1", id); e != nil {
		return e
	}
	// Retain consumed generation slots after deletion so the limit cannot be reset.
	_, e := r.db.Exec(ctx, `UPDATE render_jobs SET state=state || '{"status":"deleted","image":"","message":"Application image copies deleted."}'::jsonb,task_id='' WHERE session_id=$1`, id)
	return e
}
func (r *Repository) Jobs(ctx context.Context, id string) ([]domain.Job, error) {
	rows, e := r.db.Query(ctx, "SELECT state,task_id FROM render_jobs WHERE session_id=$1 ORDER BY id", id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.Job{}
	for rows.Next() {
		var b []byte
		var task string
		var j domain.Job
		if e = rows.Scan(&b, &task); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(b, &j); e != nil {
			return nil, e
		}
		j.SessionID = id
		j.TaskID = task
		out = append(out, j)
	}
	return out, rows.Err()
}
func (r *Repository) Job(ctx context.Context, id, key string, lock bool) (domain.Job, error) {
	var b []byte
	var task string
	j := domain.Job{}
	q := "SELECT state,task_id FROM render_jobs WHERE session_id=$1 AND id=$2"
	if lock {
		q += " FOR UPDATE"
	}
	e := r.db.QueryRow(ctx, q, id, key).Scan(&b, &task)
	if e != nil {
		return j, missing(e)
	}
	e = json.Unmarshal(b, &j)
	j.SessionID = id
	j.TaskID = task
	return j, e
}
func (r *Repository) SaveJob(ctx context.Context, j domain.Job) error {
	b, e := json.Marshal(j)
	if e != nil {
		return e
	}
	_, e = r.db.Exec(ctx, `INSERT INTO render_jobs(id,session_id,state,task_id) VALUES($1,$2,$3,$4) ON CONFLICT(id) DO UPDATE SET state=excluded.state,task_id=excluded.task_id`, j.ID, j.SessionID, b, j.TaskID)
	return e
}

// ExpiredSessions includes only IDs; image cleanup is done through the storage port.
func (r *Repository) ExpiredSessions(ctx context.Context) ([]string, error) {
	rows, e := r.db.Query(ctx, "SELECT id FROM sessions WHERE expires_at<$1 AND (EXISTS(SELECT 1 FROM photos WHERE session_id=sessions.id) OR EXISTS(SELECT 1 FROM render_jobs WHERE session_id=sessions.id))", time.Now().UTC())
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			return nil, e
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// PurgeExpiredMedia retains anonymous demo choices for merchant history.
func (r *Repository) PurgeExpiredMedia(ctx context.Context, id string) error {
	return r.Transaction(ctx, func(port application.Repository) error {
		q := port.(*Repository).db
		if _, e := q.Exec(ctx, "DELETE FROM photos WHERE session_id=$1 AND EXISTS(SELECT 1 FROM sessions WHERE id=$1 AND expires_at<now())", id); e != nil {
			return e
		}
		_, e := q.Exec(ctx, "DELETE FROM render_jobs WHERE session_id=$1 AND EXISTS(SELECT 1 FROM sessions WHERE id=$1 AND expires_at<now())", id)
		return e
	})
}

func (r *Repository) LockGenerations(ctx context.Context) error {
	_, e := r.db.Exec(ctx, "SELECT pg_advisory_xact_lock(819542)")
	return e
}
func (r *Repository) GenerationCounts(ctx context.Context, provider string, since time.Time, client string) (recent, total, mine int, err error) {
	err = r.db.QueryRow(ctx, `SELECT count(*) FILTER (WHERE created_at>=$2), count(*), count(*) FILTER (WHERE created_at>=$2 AND $3<>'' AND client_key=$3) FROM generation_ledger WHERE provider=$1`, provider, since, client).Scan(&recent, &total, &mine)
	return
}
func (r *Repository) AddGeneration(ctx context.Context, g domain.Generation) error {
	_, e := r.db.Exec(ctx, "INSERT INTO generation_ledger(id,provider,client_key,created_at) VALUES($1,$2,$3,$4)", g.ID, g.Provider, g.ClientKey, g.CreatedAt)
	return e
}
func (r *Repository) ReusableModelJob(ctx context.Context, provider, source, sku string) (domain.Job, error) {
	var b []byte
	j := domain.Job{}
	e := r.db.QueryRow(ctx, `SELECT state FROM render_jobs WHERE state->>'source_id'=$2 AND state->>'provider'=$1 AND state->>'product_id'=$3 AND state->>'status'='success' ORDER BY state->>'updated_at' DESC LIMIT 1`, provider, source, sku).Scan(&b)
	if e != nil {
		return j, missing(e)
	}
	return j, json.Unmarshal(b, &j)
}
func (r *Repository) PreviewStats(ctx context.Context, provider string, since time.Time) (domain.PreviewStats, error) {
	var s domain.PreviewStats
	if e := r.db.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE created_at>=$2) FROM generation_ledger WHERE provider=$1`, provider, since).Scan(&s.Generations, &s.Today); e != nil {
		return s, e
	}
	var avg *float64
	e := r.db.QueryRow(ctx, `SELECT
	 count(*) FILTER (WHERE state->>'status'='success' AND COALESCE(state->>'reused','false')<>'true'),
	 count(*) FILTER (WHERE state->>'status' IN ('error','unknown','timed_out')),
	 count(*) FILTER (WHERE COALESCE(state->>'reused','false')='true'),
	 avg(extract(epoch FROM (state->>'updated_at')::timestamptz-(state->>'created_at')::timestamptz)) FILTER (WHERE state->>'status'='success' AND COALESCE(state->>'reused','false')<>'true')
	 FROM render_jobs WHERE state->>'provider'=$1`, provider).Scan(&s.Succeeded, &s.Failed, &s.Reused, &avg)
	if avg != nil {
		s.AverageSeconds = *avg
	}
	return s, e
}
func (r *Repository) Setting(ctx context.Context, key string) (string, error) {
	var v string
	e := r.db.QueryRow(ctx, "SELECT value FROM settings WHERE key=$1", key).Scan(&v)
	return v, missing(e)
}
func (r *Repository) SetSetting(ctx context.Context, key, value string) error {
	_, e := r.db.Exec(ctx, "INSERT INTO settings(key,value,updated_at) VALUES($1,$2,now()) ON CONFLICT(key) DO UPDATE SET value=excluded.value,updated_at=now()", key, value)
	return e
}
