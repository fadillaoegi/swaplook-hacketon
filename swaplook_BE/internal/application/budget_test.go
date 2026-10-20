package application_test

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"strings"
	"swaplook/internal/adapters/storage"
	"swaplook/internal/application"
	"swaplook/internal/domain"
	"swaplook/internal/testkit"
	"testing"
	"time"
)

func actualSetup(t *testing.T, budget application.Budget) (*application.Service, *counted) {
	t.Helper()
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 160, 200)))
	model, e := application.NormalizeModel(b.Bytes())
	if e != nil {
		t.Fatal(e)
	}
	c := &counted{name: "youcam-v4"}
	return &application.Service{Repo: testkit.NewRepository(), Storage: storage.Local{Root: t.TempDir()}, Renderer: c, Budget: budget, Models: []application.Model{{ID: "ari", Name: "Ari", Image: model}, {ID: "bea", Name: "Bea", Image: model}}}, c
}
func shopper(t *testing.T, s *application.Service) string {
	t.Helper()
	state, e := s.Start(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Shortlist(ctx, state.ID, domain.Constraints{OriginalID: "SL-001", Size: "M", Budget: 6000}); e != nil {
		t.Fatal(e)
	}
	return state.ID
}

func TestGenerationBudgetIsGlobalAndPerClient(t *testing.T) {
	s, c := actualSetup(t, application.Budget{Daily: 3, Total: 10, PerClient: 2})
	a := shopper(t, s)
	for _, sku := range []string{"SL-002", "SL-003"} {
		if _, e := s.PreviewFor(ctx, a, sku, "model:ari", "client-a"); e != nil {
			t.Fatal(e)
		}
	}
	// A fresh session from the same client cannot reset the per-client allowance.
	if _, e := s.PreviewFor(ctx, shopper(t, s), "SL-004", "model:ari", "client-a"); !errors.Is(e, domain.ErrQuota) {
		t.Fatal("per-client allowance bypassed by a new session", e)
	}
	b := shopper(t, s)
	if _, e := s.PreviewFor(ctx, b, "SL-004", "model:ari", "client-b"); e != nil {
		t.Fatal(e)
	}
	if _, e := s.PreviewFor(ctx, shopper(t, s), "SL-002", "model:ari", "client-c"); !errors.Is(e, domain.ErrQuota) {
		t.Fatal("daily allowance exceeded", e)
	}
	if got := c.submits.Load(); got != 3 {
		t.Fatal("unexpected provider submissions", got)
	}
	q, e := s.Quota(ctx, "client-b")
	if e != nil || q["remaining_today"] != 0 || q["client_remaining"] != 1 {
		t.Fatal("quota report incorrect", q, e)
	}
}

func TestDemoModelResultIsReusedWithoutNewGeneration(t *testing.T) {
	s, c := actualSetup(t, application.Budget{Daily: 1})
	first := shopper(t, s)
	job, e := s.PreviewFor(ctx, first, "SL-002", "model:ari", "client-a")
	if e != nil {
		t.Fatal(e)
	}
	// Simulate the provider completing the first task.
	job.SessionID, job.Status, job.Image, job.UpdatedAt = first, "success", "/api/images/"+job.ID, time.Now().UTC()
	if e = s.Storage.Put(ctx, job.ID, []byte("result")); e != nil {
		t.Fatal(e)
	}
	if e = s.Repo.SaveJob(ctx, job); e != nil {
		t.Fatal(e)
	}
	second := shopper(t, s)
	reused, e := s.PreviewFor(ctx, second, "SL-002", "model:ari", "client-b")
	if e != nil || !reused.Reused || reused.Status != "success" || !strings.Contains(reused.Message, "No new generation") {
		t.Fatal("demo result not reused", reused, e)
	}
	if b, e := s.Image(ctx, second, reused.ID); e != nil || string(b) != "result" {
		t.Fatal("reused image not readable by its own session", e)
	}
	if c.submits.Load() != 1 {
		t.Fatal("reuse submitted a generation")
	}
	// Reuse does not count against the daily allowance, which is already spent.
	if _, e = s.PreviewFor(ctx, second, "SL-003", "model:ari", "client-b"); !errors.Is(e, domain.ErrQuota) {
		t.Fatal("expected quota for a new SKU", e)
	}
	stats, e := s.Summary(ctx)
	if e != nil || stats.Previews.Generations != 1 || stats.Previews.Reused != 1 || stats.Previews.Succeeded != 1 || stats.Previews.RemainingToday != 0 {
		t.Fatal("preview statistics incorrect", stats.Previews, e)
	}
}

func TestModelSourcesAreValidatedAndKeptSeparate(t *testing.T) {
	s, c := actualSetup(t, application.Budget{})
	id := shopper(t, s)
	for _, source := range []string{"demo", "model:unknown", "model:"} {
		if _, e := s.PreviewFor(ctx, id, "SL-002", source, ""); !errors.Is(e, domain.ErrInvalid) && !errors.Is(e, domain.ErrNotFound) {
			t.Fatal("invalid source accepted in actual mode", source, e)
		}
	}
	a, e := s.PreviewFor(ctx, id, "SL-002", "model:ari", "")
	if e != nil {
		t.Fatal(e)
	}
	a.SessionID, a.Status, a.UpdatedAt = id, "success", time.Now().UTC()
	_ = s.Storage.Put(ctx, a.ID, []byte("ari"))
	_ = s.Repo.SaveJob(ctx, a)
	// A different model must not reuse another model's result.
	b, e := s.PreviewFor(ctx, shopper(t, s), "SL-002", "model:bea", "")
	if e != nil || b.Reused || c.submits.Load() != 2 {
		t.Fatal("result reused across models", b, e)
	}
	sim := &application.Service{Repo: testkit.NewRepository(), Storage: s.Storage, Renderer: &counted{}, Models: s.Models}
	if _, e = sim.PreviewFor(ctx, shopper(t, sim), "SL-002", "model:ari", ""); !errors.Is(e, domain.ErrInvalid) {
		t.Fatal("model photo accepted by the simulator", e)
	}
}

func TestUnavailableSelectionAndSubstitutionInsight(t *testing.T) {
	s, id := setup(t)
	if e := s.Repo.SetStock(ctx, "SL-002", "M", 0); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Decide(ctx, id, "SL-002", "selected", ""); !errors.Is(e, domain.ErrUnavailable) || !errors.Is(e, domain.ErrConflict) {
		t.Fatal("stock conflict not reported as unavailable", e)
	}
	if _, e := s.Decide(ctx, id, "SL-003", "selected", ""); e != nil {
		t.Fatal(e)
	}
	sum, e := s.Summary(ctx)
	if e != nil || len(sum.Substitutions) != 1 || sum.Substitutions[0] != (domain.Substitution{OriginalID: "SL-001", ProductID: "SL-003", Count: 1}) {
		t.Fatal("substitution insight incorrect", sum.Substitutions, e)
	}
}
