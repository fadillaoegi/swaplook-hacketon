package postgres

import (
	"bytes"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"image"
	"image/png"
	"net/url"
	"os"
	"swaplook/internal/adapters/renderer"
	"swaplook/internal/adapters/storage"
	"swaplook/internal/application"
	"swaplook/internal/domain"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func integration(t *testing.T) *Repository {
	t.Helper()
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("TEST_DATABASE_URL not set: real PostgreSQL integration not executed")
	}
	ctx := context.Background()
	admin, e := pgx.Connect(ctx, raw)
	if e != nil {
		t.Fatal("test PostgreSQL unavailable")
	}
	schema := "test_" + application.ID()[:16]
	if _, e = admin.Exec(ctx, "CREATE SCHEMA "+schema); e != nil {
		t.Fatal(e)
	}
	u, e := url.Parse(raw)
	if e != nil {
		t.Fatal(e)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	r, e := Open(ctx, u.String())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { r.Close(); _, _ = admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); _ = admin.Close(ctx) })
	if e = r.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	if e = r.Migrate(ctx); e != nil {
		t.Fatal("migrations not repeatable", e)
	}
	return r
}

type pollFixture struct {
	renderer.Simulator
	entered, release chan struct{}
	calls            atomic.Int32
}

func (p *pollFixture) Poll(_ context.Context, _ string) (application.RenderResult, error) {
	p.calls.Add(1)
	if p.entered != nil {
		close(p.entered)
		<-p.release
	}
	return application.RenderResult{Status: "success"}, nil
}
func TestPostgresPollLeaseAndReconciliation(t *testing.T) {
	r := integration(t)
	ctx := context.Background()
	provider := &pollFixture{entered: make(chan struct{}), release: make(chan struct{})}
	s := &application.Service{Repo: r, Renderer: provider, Storage: storage.Local{Root: t.TempDir()}}
	state, e := s.Start(ctx)
	if e != nil {
		t.Fatal(e)
	}
	list, e := s.Shortlist(ctx, state.ID, domain.Constraints{OriginalID: "SL-001", Size: "M", Budget: 6000})
	if e != nil {
		t.Fatal(e)
	}
	j, e := s.Preview(ctx, state.ID, list.Candidates[0].Product.ID, "demo")
	if e != nil {
		t.Fatal(e)
	}
	j.UpdatedAt = time.Now().Add(-time.Minute)
	if e = r.SaveJob(ctx, j); e != nil {
		t.Fatal(e)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, e := s.Poll(ctx, state.ID, j.ID); e != nil {
			t.Error(e)
		}
	}()
	<-provider.entered
	current, e := r.Job(ctx, state.ID, j.ID, false)
	if e != nil {
		t.Fatal(e)
	}
	current.UpdatedAt = time.Now().Add(-time.Minute)
	if e = r.SaveJob(ctx, current); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Poll(ctx, state.ID, j.ID); e != nil {
		t.Fatal(e)
	}
	if provider.calls.Load() != 1 {
		t.Fatal("persisted lease did not serialize provider polling")
	}
	close(provider.release)
	<-done
	current, e = r.Job(ctx, state.ID, j.ID, false)
	if e != nil || current.Status != "success" || !current.PollLeaseUntil.IsZero() {
		t.Fatal("claim did not survive database JSON roundtrip", e)
	}
	provider = &pollFixture{}
	s.Renderer = provider
	current.Status = "timed_out"
	current.Polls = 60
	current.UpdatedAt = time.Now().Add(-time.Minute)
	if e = r.SaveJob(ctx, current); e != nil {
		t.Fatal(e)
	}
	reconciled, e := s.Reconcile(ctx, state.ID, j.ID)
	if e != nil || reconciled.Status != "success" || reconciled.Reconciliations != 1 || reconciled.TaskID != current.TaskID {
		t.Fatal("existing task was not reconciled", e)
	}
}
func TestPostgresExpiredMediaPurgeKeepsDecisionHistory(t *testing.T) {
	r := integration(t)
	ctx := context.Background()
	store := storage.Local{Root: t.TempDir()}
	s := &application.Service{Repo: r, Renderer: renderer.Simulator{}, Storage: store}
	state, e := s.Start(ctx)
	if e != nil {
		t.Fatal(e)
	}
	list, e := s.Shortlist(ctx, state.ID, domain.Constraints{OriginalID: "SL-001", Size: "M", Budget: 6000})
	if e != nil {
		t.Fatal(e)
	}
	var data bytes.Buffer
	_ = png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 128, 128)))
	photo, e := s.Upload(ctx, state.ID, data.Bytes(), true)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Decide(ctx, state.ID, list.Candidates[0].Product.ID, "selected", ""); e != nil {
		t.Fatal(e)
	}
	if _, e = r.pool.Exec(ctx, "UPDATE sessions SET expires_at=now()-interval '1 hour' WHERE id=$1", state.ID); e != nil {
		t.Fatal(e)
	}
	if n, e := application.PurgeExpiredMedia(ctx, r, store); e != nil || n != 1 {
		t.Fatal("expired cleanup failed", e)
	}
	if _, e = store.Read(ctx, photo.ID); e == nil {
		t.Fatal("expired photo still on disk")
	}
	summary, e := r.Summary(ctx)
	if e != nil || summary.Choices != 1 {
		t.Fatal("decision history lost", e)
	}
	if n, e := application.PurgeExpiredMedia(ctx, r, store); e != nil || n != 0 {
		t.Fatal("expired cleanup is not idempotent", e)
	}
}
func TestPostgresPersistenceAndRecheck(t *testing.T) {
	r := integration(t)
	ctx := context.Background()
	s := &application.Service{Repo: r, Renderer: renderer.Simulator{}, Storage: storage.Local{Root: t.TempDir()}}
	state, e := s.Start(ctx)
	if e != nil {
		t.Fatal(e)
	}
	c := domain.Constraints{OriginalID: "SL-001", Size: "M", Budget: 6000}
	list, e := s.Shortlist(ctx, state.ID, c)
	if e != nil || len(list.Candidates) != 3 {
		t.Fatal("shortlist failed", e)
	}
	sku := list.Candidates[0].Product.ID
	if e = r.SetStock(ctx, sku, "M", 0); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Decide(ctx, state.ID, sku, "selected", ""); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("stale stock accepted")
	}
	if e = r.SetStock(ctx, sku, "M", 5); e != nil {
		t.Fatal(e)
	}
	if _, e = r.pool.Exec(ctx, "UPDATE products SET price=7000 WHERE id=$1", sku); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Decide(ctx, state.ID, sku, "selected", ""); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("price above budget accepted")
	}
	if _, e = r.pool.Exec(ctx, "UPDATE products SET price=4200 WHERE id=$1", sku); e != nil {
		t.Fatal(e)
	}
	d, e := s.Decide(ctx, state.ID, sku, "selected", "")
	if e != nil {
		t.Fatal(e)
	}
	var successes atomic.Int32
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			duplicate, e := s.Decide(ctx, state.ID, sku, "selected", "")
			if e != nil || duplicate.ID != d.ID {
				t.Error("concurrent decision changed", e)
			} else {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 10 {
		t.Fatal("concurrent decision requests failed")
	}
	again, e := r.Session(ctx, state.ID, false)
	if e != nil || again.Decision == nil || again.Decision.ID != d.ID {
		t.Fatal("persisted choice lost")
	}
	summary, e := r.Summary(ctx)
	if e != nil || summary.Choices != 1 {
		t.Fatal("duplicate decisions counted")
	}
}
func TestPostgresGenerationBudgetUnderConcurrency(t *testing.T) {
	r := integration(t)
	ctx := context.Background()
	s := &application.Service{Repo: r, Renderer: renderer.Simulator{}, Storage: storage.Local{Root: t.TempDir()}}
	state, e := s.Start(ctx)
	if e != nil {
		t.Fatal(e)
	}
	list, e := s.Shortlist(ctx, state.ID, domain.Constraints{OriginalID: "SL-001", Size: "M", Budget: 6000})
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	for _, a := range list.Candidates {
		for range 8 {
			wg.Add(1)
			go func(sku string) {
				defer wg.Done()
				if _, e := s.Preview(ctx, state.ID, sku, "demo"); e != nil {
					t.Error(e)
				}
			}(a.Product.ID)
		}
	}
	wg.Wait()
	jobs, e := r.Jobs(ctx, state.ID)
	if e != nil || len(jobs) != 3 {
		t.Fatal("budget/idempotency failed", e)
	}
	if e = s.DeleteMedia(ctx, state.ID); e != nil {
		t.Fatal(e)
	}
	jobs, e = r.Jobs(ctx, state.ID)
	if e != nil || len(jobs) != 3 {
		t.Fatal("deletion reset budget")
	}
	for _, j := range jobs {
		if j.Status != "deleted" {
			t.Fatal("job remains accessible")
		}
	}
}

func TestPostgresGenerationLedgerAndDemoReuse(t *testing.T) {
	r := integration(t)
	ctx := context.Background()
	now := time.Now().UTC()
	for i, g := range []domain.Generation{{ID: "g1", Provider: "youcam-v4", ClientKey: "a", CreatedAt: now}, {ID: "g2", Provider: "youcam-v4", ClientKey: "b", CreatedAt: now}, {ID: "g3", Provider: "youcam-v4", ClientKey: "a", CreatedAt: now.Add(-48 * time.Hour)}, {ID: "g4", Provider: "simulator", ClientKey: "a", CreatedAt: now}} {
		if e := r.AddGeneration(ctx, g); e != nil {
			t.Fatal(i, e)
		}
	}
	e := r.Transaction(ctx, func(port application.Repository) error {
		if e := port.LockGenerations(ctx); e != nil {
			return e
		}
		today, total, mine, e := port.GenerationCounts(ctx, "youcam-v4", now.Truncate(24*time.Hour), "a")
		if e == nil && (today != 2 || total != 3 || mine != 1) {
			t.Fatal("ledger counts incorrect", today, total, mine)
		}
		return e
	})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = r.ReusableModelJob(ctx, "youcam-v4", "model:ari", "SL-002"); !errors.Is(e, domain.ErrNotFound) {
		t.Fatal("unexpected reusable job", e)
	}
	session := domain.Session{ID: application.ID(), Demo: true, ExpiresAt: now.Add(time.Hour)}
	if e = r.CreateSession(ctx, session); e != nil {
		t.Fatal(e)
	}
	job := domain.Job{ID: application.ID(), SessionID: session.ID, ProductID: "SL-002", Size: "M", SourceID: "model:ari", Provider: "youcam-v4", Status: "success", CreatedAt: now.Add(-14 * time.Second), UpdatedAt: now}
	if e = r.SaveJob(ctx, job); e != nil {
		t.Fatal(e)
	}
	found, e := r.ReusableModelJob(ctx, "youcam-v4", "model:ari", "SL-002")
	if e != nil || found.ID != job.ID {
		t.Fatal("successful demo job not found", e)
	}
	stats, e := r.PreviewStats(ctx, "youcam-v4", now.Truncate(24*time.Hour))
	if e != nil || stats.Generations != 3 || stats.Today != 2 || stats.Succeeded != 1 || stats.AverageSeconds < 13 || stats.AverageSeconds > 15 {
		t.Fatal("preview stats incorrect", stats, e)
	}
}

func TestPostgresSettingsRoundTrip(t *testing.T) {
	r := integration(t)
	ctx := context.Background()
	if _, e := r.Setting(ctx, "ai_previews"); !errors.Is(e, domain.ErrNotFound) {
		t.Fatal("unexpected default setting", e)
	}
	for _, v := range []string{"off", "on"} {
		if e := r.SetSetting(ctx, "ai_previews", v); e != nil {
			t.Fatal(e)
		}
		if got, e := r.Setting(ctx, "ai_previews"); e != nil || got != v {
			t.Fatal("setting not persisted", got, e)
		}
	}
}
