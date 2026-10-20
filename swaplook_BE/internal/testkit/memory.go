// Package testkit provides isolated fixtures for tests. It is never wired into the API.
package testkit

import (
	"context"
	"encoding/json"
	"swaplook/internal/application"
	"swaplook/internal/domain"
	"sync"
	"time"
)

type memoryState struct {
	products []domain.Product
	sessions map[string]domain.Session
	photos   map[string]domain.Photo
	jobs     map[string]domain.Job
	ledger   []domain.Generation
	settings map[string]string
}
type MemoryRepository struct {
	mu    *sync.Mutex
	state *memoryState
	tx    bool
}

func NewRepository() *MemoryRepository {
	p := []domain.Product{{ID: "SL-001", Name: "Original", Price: 4800, Currency: "USD", Color: "cream", Material: "cotton", Cut: "relaxed", Category: "tops", Stock: map[string]int{"M": 0}}}
	for _, id := range []string{"SL-002", "SL-003", "SL-004", "SL-005"} {
		p = append(p, domain.Product{ID: id, Name: id, Price: 4200, Currency: "USD", Color: "cream", Material: "cotton", Cut: "relaxed", Category: "tops", Stock: map[string]int{"M": 5}})
	}
	return &MemoryRepository{mu: &sync.Mutex{}, state: &memoryState{products: p, sessions: map[string]domain.Session{}, photos: map[string]domain.Photo{}, jobs: map[string]domain.Job{}}}
}
func (r *MemoryRepository) guard() func() {
	if r.tx {
		return func() {}
	}
	r.mu.Lock()
	return r.mu.Unlock
}
func (r *MemoryRepository) Transaction(_ context.Context, fn func(application.Repository) error) error {
	defer r.guard()()
	return fn(&MemoryRepository{mu: r.mu, state: r.state, tx: true})
}
func clone[T any](v T) T { b, _ := json.Marshal(v); var out T; _ = json.Unmarshal(b, &out); return out }
func (r *MemoryRepository) Catalog(_ context.Context) ([]domain.Product, error) {
	defer r.guard()()
	return clone(r.state.products), nil
}
func (r *MemoryRepository) Product(_ context.Context, id, size string) (domain.Product, error) {
	defer r.guard()()
	for _, p := range r.state.products {
		if p.ID == id {
			return clone(p), nil
		}
	}
	return domain.Product{}, domain.ErrNotFound
}
func (r *MemoryRepository) CreateSession(_ context.Context, s domain.Session) error {
	defer r.guard()()
	r.state.sessions[s.ID] = s
	return nil
}
func (r *MemoryRepository) Session(_ context.Context, id string, _ bool) (domain.Session, error) {
	defer r.guard()()
	s, ok := r.state.sessions[id]
	if !ok || s.ExpiresAt.Before(time.Now()) {
		return s, domain.ErrNotFound
	}
	out := clone(s)
	out.ID = id
	return out, nil
}
func (r *MemoryRepository) SaveSession(_ context.Context, s domain.Session) error {
	defer r.guard()()
	r.state.sessions[s.ID] = s
	return nil
}
func (r *MemoryRepository) DeleteSession(_ context.Context, id string) error {
	defer r.guard()()
	delete(r.state.sessions, id)
	return nil
}
func (r *MemoryRepository) SetStock(_ context.Context, id, size string, qty int) error {
	defer r.guard()()
	if !domain.ValidSize(size) || qty < 0 || qty > 10000 {
		return domain.ErrInvalid
	}
	for i, p := range r.state.products {
		if p.ID == id {
			r.state.products[i].Stock[size] = qty
			return nil
		}
	}
	return domain.ErrNotFound
}
func (r *MemoryRepository) Summary(_ context.Context) (domain.Summary, error) {
	defer r.guard()()
	out := domain.Summary{Sessions: len(r.state.sessions), Demo: true, Decisions: []domain.Decision{}}
	for _, s := range r.state.sessions {
		if s.Decision != nil {
			out.Decisions = append(out.Decisions, *s.Decision)
			if s.Decision.Kind == "selected" {
				out.Choices++
			} else {
				out.Rejections++
			}
		}
	}
	return out, nil
}
func (r *MemoryRepository) AddPhoto(_ context.Context, p domain.Photo) error {
	defer r.guard()()
	r.state.photos[p.ID] = p
	return nil
}
func (r *MemoryRepository) Photos(_ context.Context, id string) ([]domain.Photo, error) {
	defer r.guard()()
	out := []domain.Photo{}
	for _, p := range r.state.photos {
		if p.SessionID == id {
			out = append(out, p)
		}
	}
	return out, nil
}
func (r *MemoryRepository) DeleteMedia(_ context.Context, id string) error {
	defer r.guard()()
	for key, p := range r.state.photos {
		if p.SessionID == id {
			delete(r.state.photos, key)
		}
	}
	for key, j := range r.state.jobs {
		if j.SessionID == id {
			j.Status = "deleted"
			j.Image = ""
			j.TaskID = ""
			r.state.jobs[key] = j
		}
	}
	return nil
}
func (r *MemoryRepository) Jobs(_ context.Context, id string) ([]domain.Job, error) {
	defer r.guard()()
	out := []domain.Job{}
	for _, j := range r.state.jobs {
		if j.SessionID == id {
			out = append(out, j)
		}
	}
	return out, nil
}
func (r *MemoryRepository) Job(_ context.Context, id, key string, _ bool) (domain.Job, error) {
	defer r.guard()()
	j, ok := r.state.jobs[key]
	if !ok || j.SessionID != id {
		return j, domain.ErrNotFound
	}
	return j, nil
}
func (r *MemoryRepository) SaveJob(_ context.Context, j domain.Job) error {
	defer r.guard()()
	r.state.jobs[j.ID] = j
	return nil
}

func (r *MemoryRepository) ExpiredSessions(_ context.Context) ([]string, error) {
	defer r.guard()()
	ids := []string{}
	for id, state := range r.state.sessions {
		if state.ExpiresAt.Before(time.Now()) {
			hasMedia := false
			for _, p := range r.state.photos {
				if p.SessionID == id {
					hasMedia = true
				}
			}
			for _, j := range r.state.jobs {
				if j.SessionID == id {
					hasMedia = true
				}
			}
			if hasMedia {
				ids = append(ids, id)
			}
		}
	}
	return ids, nil
}
func (r *MemoryRepository) PurgeExpiredMedia(_ context.Context, id string) error {
	defer r.guard()()
	s, ok := r.state.sessions[id]
	if !ok || s.ExpiresAt.After(time.Now()) {
		return nil
	}
	for key, p := range r.state.photos {
		if p.SessionID == id {
			delete(r.state.photos, key)
		}
	}
	for key, j := range r.state.jobs {
		if j.SessionID == id {
			delete(r.state.jobs, key)
		}
	}
	return nil
}

func (r *MemoryRepository) LockGenerations(context.Context) error { return nil }
func (r *MemoryRepository) GenerationCounts(_ context.Context, provider string, since time.Time, client string) (recent, total, mine int, err error) {
	defer r.guard()()
	for _, g := range r.state.ledger {
		if g.Provider != provider {
			continue
		}
		total++
		if !g.CreatedAt.Before(since) {
			recent++
			if client != "" && g.ClientKey == client {
				mine++
			}
		}
	}
	return
}
func (r *MemoryRepository) AddGeneration(_ context.Context, g domain.Generation) error {
	defer r.guard()()
	r.state.ledger = append(r.state.ledger, g)
	return nil
}
func (r *MemoryRepository) ReusableModelJob(_ context.Context, provider, source, sku string) (domain.Job, error) {
	defer r.guard()()
	var best domain.Job
	found := false
	for _, j := range r.state.jobs {
		if j.SourceID == source && j.Provider == provider && j.ProductID == sku && j.Status == "success" && (!found || j.UpdatedAt.After(best.UpdatedAt)) {
			best, found = clone(j), true
		}
	}
	if !found {
		return best, domain.ErrNotFound
	}
	return best, nil
}
func (r *MemoryRepository) PreviewStats(_ context.Context, provider string, since time.Time) (domain.PreviewStats, error) {
	defer r.guard()()
	var s domain.PreviewStats
	for _, g := range r.state.ledger {
		if g.Provider == provider {
			s.Generations++
			if !g.CreatedAt.Before(since) {
				s.Today++
			}
		}
	}
	var seconds float64
	for _, j := range r.state.jobs {
		if j.Provider != provider {
			continue
		}
		switch {
		case j.Reused:
			s.Reused++
		case j.Status == "success":
			s.Succeeded++
			seconds += j.UpdatedAt.Sub(j.CreatedAt).Seconds()
		case j.Status == "error" || j.Status == "unknown" || j.Status == "timed_out":
			s.Failed++
		}
	}
	if s.Succeeded > 0 {
		s.AverageSeconds = seconds / float64(s.Succeeded)
	}
	return s, nil
}
func (r *MemoryRepository) Setting(_ context.Context, key string) (string, error) {
	defer r.guard()()
	v, ok := r.state.settings[key]
	if !ok {
		return "", domain.ErrNotFound
	}
	return v, nil
}
func (r *MemoryRepository) SetSetting(_ context.Context, key, value string) error {
	defer r.guard()()
	if r.state.settings == nil {
		r.state.settings = map[string]string{}
	}
	r.state.settings[key] = value
	return nil
}
