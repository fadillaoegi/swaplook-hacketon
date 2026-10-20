package application_test

import (
	"errors"
	"swaplook/internal/adapters/renderer"
	"swaplook/internal/application"
	"swaplook/internal/domain"
	"testing"
	"time"
)

func TestAISwitchRoutesNewPreviewsAndKeepsExistingJobsOnTheirProvider(t *testing.T) {
	s, ai := actualSetup(t, application.Budget{Daily: 5})
	s.Simulator = renderer.Simulator{}
	id := shopper(t, s)
	pending, e := s.PreviewFor(ctx, id, "SL-002", "model:ari", "client")
	if e != nil {
		t.Fatal(e)
	}
	if e = s.SetAI(ctx, false); e != nil {
		t.Fatal(e)
	}
	if s.AIEnabled() || s.Active().Name() != "simulator" {
		t.Fatal("AI still active after switching off")
	}
	if q, _ := s.Quota(ctx, "client"); q["remaining_today"] != -1 {
		t.Fatal("quota reported while AI is off", q)
	}
	if _, e = s.PreviewFor(ctx, id, "SL-003", "model:ari", "client"); !errors.Is(e, domain.ErrInvalid) {
		t.Fatal("model source accepted while AI is off", e)
	}
	sim, e := s.PreviewFor(ctx, id, "SL-003", "demo", "client")
	if e != nil || sim.Provider != "simulator" {
		t.Fatal("simulator preview not created while AI is off", sim, e)
	}
	if ai.submits.Load() != 1 {
		t.Fatal("switched-off preview reached the provider")
	}
	// The earlier AI job is still polled by the AI provider, not marked as changed.
	pending.SessionID, pending.Status, pending.UpdatedAt = id, "pending", time.Now().Add(-time.Minute)
	if e = s.Repo.SaveJob(ctx, pending); e != nil {
		t.Fatal(e)
	}
	ai.image = []byte("not an image")
	if j, _ := s.Poll(ctx, id, pending.ID); j.Status == "provider_changed" || ai.polls.Load() != 1 {
		t.Fatal("existing AI job not polled by its own provider", j.Status, ai.polls.Load())
	}
	stats, e := s.Summary(ctx)
	if e != nil || !stats.Previews.Available || stats.Previews.Enabled || stats.Previews.Provider != "youcam-v4" {
		t.Fatal("summary does not report the switch", stats.Previews, e)
	}
	// The setting persists for a restarted service.
	restarted := &application.Service{Repo: s.Repo, Storage: s.Storage, Renderer: ai, Simulator: renderer.Simulator{}}
	if e = restarted.LoadAISetting(ctx); e != nil || restarted.AIEnabled() {
		t.Fatal("switch not restored after restart", e)
	}
	if e = restarted.SetAI(ctx, true); e != nil || !restarted.AIEnabled() {
		t.Fatal("AI not switched back on", e)
	}
	simOnly := &application.Service{Repo: s.Repo, Storage: s.Storage, Renderer: renderer.Simulator{}}
	if e = simOnly.SetAI(ctx, true); !errors.Is(e, domain.ErrInvalid) {
		t.Fatal("switch accepted without an AI provider", e)
	}
}
