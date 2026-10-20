package application_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"swaplook/internal/adapters/renderer"
	"swaplook/internal/adapters/storage"
	"swaplook/internal/application"
	"swaplook/internal/domain"
	"swaplook/internal/testkit"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var ctx = context.Background()

func setup(t *testing.T) (*application.Service, string) {
	t.Helper()
	s := &application.Service{Repo: testkit.NewRepository(), Storage: storage.Local{Root: t.TempDir()}, Renderer: renderer.Simulator{}}
	state, e := s.Start(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Shortlist(ctx, state.ID, domain.Constraints{OriginalID: "SL-001", Size: "M", Budget: 6000}); e != nil {
		t.Fatal(e)
	}
	return s, state.ID
}
func photo(t *testing.T, s *application.Service, id string) domain.Photo {
	t.Helper()
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 128, 128)))
	p, e := s.Upload(ctx, id, b.Bytes(), true)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func TestSelectionRechecksStockAndIsIdempotent(t *testing.T) {
	s, id := setup(t)
	if e := s.Repo.SetStock(ctx, "SL-002", "M", 0); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Decide(ctx, id, "SL-002", "selected", ""); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("stale stock accepted", e)
	}
	if e := s.Repo.SetStock(ctx, "SL-002", "M", 1); e != nil {
		t.Fatal(e)
	}
	a, e := s.Decide(ctx, id, "SL-002", "selected", "")
	if e != nil {
		t.Fatal(e)
	}
	b, e := s.Decide(ctx, id, "SL-002", "selected", "")
	if e != nil || a.ID != b.ID {
		t.Fatal("duplicate choice not idempotent")
	}
	if _, e = s.Decide(ctx, id, "SL-003", "selected", ""); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("second choice accepted")
	}
	if _, e = s.Shortlist(ctx, id, domain.Constraints{OriginalID: "SL-001", Size: "M", Budget: 6000}); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("final decision overwritten")
	}
	p, e := s.Repo.Product(ctx, "SL-002", "M")
	if e != nil || p.Stock["M"] != 1 {
		t.Fatal("selection should not reserve stock")
	}
	summary, e := s.Repo.Summary(ctx)
	if e != nil || summary.Choices != 1 || !summary.Demo {
		t.Fatal("summary incorrect")
	}
}
func TestRejectAndUnshortlistedSelection(t *testing.T) {
	s, id := setup(t)
	if _, e := s.Decide(ctx, id, "SL-005", "selected", ""); !errors.Is(e, domain.ErrInvalid) {
		t.Fatal("unshortlisted SKU accepted")
	}
	if _, e := s.Decide(ctx, id, "", "rejected", "fabric inferred from photo"); !errors.Is(e, domain.ErrInvalid) {
		t.Fatal("unknown reason accepted")
	}
	d, e := s.Decide(ctx, id, "", "rejected", "style")
	if e != nil || d.Kind != "rejected" {
		t.Fatal(e)
	}
}

type counted struct {
	submits     atomic.Int32
	polls       atomic.Int32
	fail        bool
	entered     chan struct{}
	release     chan struct{}
	name        string
	image       []byte
	pollEntered chan struct{}
	pollRelease chan struct{}
	pollStatus  string
	onSubmit    func()
}

func (c *counted) Name() string {
	if c.name != "" {
		return c.name
	}
	return "simulator"
}
func (c *counted) Submit(_ context.Context, _ application.RenderInput) (string, error) {
	c.submits.Add(1)
	if c.entered != nil {
		close(c.entered)
		<-c.release
	}
	if c.fail {
		return "", errors.New("provider timeout")
	}
	if c.onSubmit != nil {
		c.onSubmit()
	}
	return "private-task", nil
}
func (c *counted) Poll(_ context.Context, _ string) (application.RenderResult, error) {
	c.polls.Add(1)
	if c.pollEntered != nil {
		close(c.pollEntered)
		<-c.pollRelease
	}
	status := c.pollStatus
	if status == "" {
		status = "success"
	}
	return application.RenderResult{Status: status, Image: c.image}, nil
}

func TestPollLeasePreventsOverlappingProviderRequests(t *testing.T) {
	s, id := setup(t)
	p := &counted{pollEntered: make(chan struct{}), pollRelease: make(chan struct{})}
	s.Renderer = p
	j, e := s.Preview(ctx, id, "SL-002", "demo")
	if e != nil {
		t.Fatal(e)
	}
	j.UpdatedAt = time.Now().Add(-time.Minute)
	_ = s.Repo.SaveJob(ctx, j)
	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, e := s.Poll(ctx, id, j.ID); e != nil {
			t.Error(e)
		}
	}()
	<-p.pollEntered
	claimed, e := s.Repo.Job(ctx, id, j.ID, false)
	if e != nil {
		t.Fatal(e)
	}
	claimed.UpdatedAt = time.Now().Add(-time.Minute)
	_ = s.Repo.SaveJob(ctx, claimed)
	if _, e = s.Poll(ctx, id, j.ID); e != nil {
		t.Fatal(e)
	}
	if p.polls.Load() != 1 {
		t.Fatal("provider was polled while its prior request was in flight")
	}
	close(p.pollRelease)
	<-done
}

func TestReconciliationIsBoundedAndNeverResubmitsGeneration(t *testing.T) {
	s, id := setup(t)
	p := &counted{pollStatus: "pending"}
	s.Renderer = p
	j, e := s.Preview(ctx, id, "SL-002", "demo")
	if e != nil {
		t.Fatal(e)
	}
	j.Polls = 60
	j.UpdatedAt = time.Now().Add(-time.Minute)
	_ = s.Repo.SaveJob(ctx, j)
	j, e = s.Poll(ctx, id, j.ID)
	if e != nil || j.Status != "timed_out" {
		t.Fatal(e)
	}
	for i := 1; i <= 3; i++ {
		j.UpdatedAt = time.Now().Add(-time.Minute)
		_ = s.Repo.SaveJob(ctx, j)
		j, e = s.Reconcile(ctx, id, j.ID)
		if e != nil || j.Status != "timed_out" || j.Reconciliations != i {
			t.Fatal("reconciliation did not retain task", e)
		}
	}
	j.UpdatedAt = time.Now().Add(-time.Minute)
	_ = s.Repo.SaveJob(ctx, j)
	if _, e = s.Reconcile(ctx, id, j.ID); !errors.Is(e, domain.ErrLimit) {
		t.Fatal("unbounded manual reconciliation")
	}
	if p.submits.Load() != 1 || p.polls.Load() != 3 || j.TaskID != "private-task" {
		t.Fatal("reconciliation re-submitted a task or lost its provenance")
	}
}

func TestTimedOutTaskCanCompleteThroughReconciliation(t *testing.T) {
	s, id := setup(t)
	p := &counted{}
	s.Renderer = p
	j, e := s.Preview(ctx, id, "SL-002", "demo")
	if e != nil {
		t.Fatal(e)
	}
	j.Status = "timed_out"
	j.Polls = 60
	j.UpdatedAt = time.Now().Add(-time.Minute)
	_ = s.Repo.SaveJob(ctx, j)
	out, e := s.Reconcile(ctx, id, j.ID)
	if e != nil || out.Status != "success" || out.ID != j.ID || p.submits.Load() != 1 || out.Reconciliations != 1 {
		t.Fatal("existing task could not be completed", e)
	}
}

type contextRepository struct{ application.Repository }

func (r contextRepository) Transaction(ctx context.Context, fn func(application.Repository) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.Repository.Transaction(ctx, fn)
}
func TestKnownProviderTaskSurvivesClientDisconnect(t *testing.T) {
	s, id := setup(t)
	s.Repo = contextRepository{s.Repo}
	requestCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	p := &counted{onSubmit: cancel}
	s.Renderer = p
	j, e := s.Preview(requestCtx, id, "SL-002", "demo")
	if e != nil {
		t.Fatal(e)
	}
	persisted, e := s.Repo.Job(ctx, id, j.ID, false)
	if e != nil || persisted.Status != "pending" || persisted.TaskID != "private-task" {
		t.Fatal("client cancellation discarded a known provider task", e)
	}
}

func TestProviderResultAboveUploadLimitIsAcceptedWithinResultLimit(t *testing.T) {
	s, id := setup(t)
	photo := photo(t, s, id)
	pngBytes, e := s.Storage.Read(ctx, photo.ID)
	if e != nil {
		t.Fatal(e)
	}
	padded := append(pngBytes, make([]byte, (8<<20)+1-len(pngBytes))...)
	if _, e = s.Upload(ctx, id, padded, true); !errors.Is(e, domain.ErrInvalid) {
		t.Fatal("oversized shopper upload accepted")
	}
	s.Renderer = &counted{name: "youcam-v4", image: padded}
	j, e := s.Preview(ctx, id, "SL-002", photo.ID)
	if e != nil {
		t.Fatal(e)
	}
	j.UpdatedAt = time.Now().Add(-time.Minute)
	_ = s.Repo.SaveJob(ctx, j)
	j, e = s.Poll(ctx, id, j.ID)
	if e != nil || j.Status != "success" {
		t.Fatal("valid provider result was incorrectly limited to shopper upload size", e)
	}
}

func TestNonSimulatorResultIsStoredPrivatelyAndDeleted(t *testing.T) {
	s, id := setup(t)
	p := photo(t, s, id)
	imageBytes, e := s.Storage.Read(ctx, p.ID)
	if e != nil {
		t.Fatal(e)
	}
	provider := &counted{name: "youcam-v4", image: imageBytes}
	s.Renderer = provider
	if _, e = s.Preview(ctx, id, "SL-002", "demo"); !errors.Is(e, domain.ErrInvalid) {
		t.Fatal("demo source accepted in actual-mode contract")
	}
	j, e := s.Preview(ctx, id, "SL-002", p.ID)
	if e != nil {
		t.Fatal(e)
	}
	j.UpdatedAt = time.Now().Add(-time.Minute)
	if e = s.Repo.SaveJob(ctx, j); e != nil {
		t.Fatal(e)
	}
	j, e = s.Poll(ctx, id, j.ID)
	if e != nil || j.Status != "success" || j.Image != "/api/images/"+j.ID {
		t.Fatal("private result not stored", e)
	}
	if _, e = s.Image(ctx, id, j.ID); e != nil {
		t.Fatal("owner cannot read result", e)
	}
	other, e := s.Start(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Image(ctx, other.ID, j.ID); !errors.Is(e, domain.ErrNotFound) {
		t.Fatal("result visible to foreign session")
	}
	if e = s.DeleteMedia(ctx, id); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Storage.Read(ctx, j.ID); e == nil {
		t.Fatal("result file retained after deletion")
	}
	if _, e = s.Storage.Read(ctx, p.ID); e == nil {
		t.Fatal("source file retained after deletion")
	}
}

func TestProviderModeChangeNeverPollsDifferentProvider(t *testing.T) {
	s, id := setup(t)
	j, e := s.Preview(ctx, id, "SL-002", "demo")
	if e != nil {
		t.Fatal(e)
	}
	different := &counted{name: "youcam-v4"}
	s.Renderer = different
	j, e = s.Poll(ctx, id, j.ID)
	if e != nil || j.Status != "provider_changed" || different.polls.Load() != 0 {
		t.Fatal("task polled using different provider")
	}
}
func TestConcurrentGenerationBudgetIdempotencyAndDeletion(t *testing.T) {
	s, id := setup(t)
	provider := &counted{}
	s.Renderer = provider
	p := photo(t, s, id)
	var wg sync.WaitGroup
	for _, sku := range []string{"SL-002", "SL-003", "SL-004"} {
		for _, source := range []string{"demo", p.ID} {
			for range 3 {
				wg.Add(1)
				go func(sku, source string) {
					defer wg.Done()
					_, e := s.Preview(ctx, id, sku, source)
					if e != nil && !errors.Is(e, domain.ErrLimit) {
						t.Error(e)
					}
				}(sku, source)
			}
		}
	}
	wg.Wait()
	jobs, e := s.Repo.Jobs(ctx, id)
	if e != nil || len(jobs) != 3 || provider.submits.Load() != 3 {
		t.Fatal("generation limit or idempotency broken", len(jobs), provider.submits.Load(), e)
	}
	// Advance timestamps directly to test polling without waiting in the suite.
	for _, j := range jobs {
		j.UpdatedAt = time.Now().Add(-time.Minute)
		if e = s.Repo.SaveJob(ctx, j); e != nil {
			t.Fatal(e)
		}
	}
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, e := s.Poll(ctx, id, jobs[0].ID); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	if provider.polls.Load() != 1 {
		t.Fatal("concurrent tabs polled the provider more than once")
	}
	if e = s.DeleteMedia(ctx, id); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Image(ctx, id, p.ID); !errors.Is(e, domain.ErrNotFound) {
		t.Fatal("deleted photo accessible")
	}
	p2 := photo(t, s, id)
	if _, e = s.Preview(ctx, id, "SL-002", p2.ID); !errors.Is(e, domain.ErrLimit) {
		t.Fatal("deletion reset generation budget")
	}
}
func TestAmbiguousSubmissionNeverAutomaticallyResubmits(t *testing.T) {
	s, id := setup(t)
	p := &counted{fail: true}
	s.Renderer = p
	j, e := s.Preview(ctx, id, "SL-002", "demo")
	if !errors.Is(e, domain.ErrProvider) || j.Status != "unknown" {
		t.Fatal("ambiguous task not retained")
	}
	same, e := s.Preview(ctx, id, "SL-002", "demo")
	if e != nil || same.ID != j.ID || p.submits.Load() != 1 {
		t.Fatal("failed submission was retried")
	}
}
func TestDeleteDuringSubmissionDoesNotResurrectImages(t *testing.T) {
	s, id := setup(t)
	p := &counted{entered: make(chan struct{}), release: make(chan struct{})}
	s.Renderer = p
	done := make(chan domain.Job, 1)
	go func() {
		j, e := s.Preview(ctx, id, "SL-002", "demo")
		if e != nil {
			t.Error(e)
		}
		done <- j
	}()
	<-p.entered
	if e := s.DeleteMedia(ctx, id); e != nil {
		t.Fatal(e)
	}
	close(p.release)
	if j := <-done; j.Status != "deleted" {
		t.Fatal("deletion was undone by a late provider response")
	}
}
func TestPhotoConsentAndSessionIsolation(t *testing.T) {
	s, id := setup(t)
	if _, e := s.Upload(ctx, id, []byte("not an image"), false); !errors.Is(e, domain.ErrInvalid) {
		t.Fatal("consent required")
	}
	if _, e := s.Upload(ctx, id, []byte("<svg/>"), true); !errors.Is(e, domain.ErrInvalid) {
		t.Fatal("invalid image accepted")
	}
	p := photo(t, s, id)
	other, e := s.Start(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Image(ctx, other.ID, p.ID); !errors.Is(e, domain.ErrNotFound) {
		t.Fatal("foreign photo visible")
	}
	if _, e = s.Shortlist(ctx, other.ID, domain.Constraints{OriginalID: "SL-001", Size: "M", Budget: 6000}); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Preview(ctx, other.ID, "SL-002", p.ID); !errors.Is(e, domain.ErrNotFound) {
		t.Fatal("foreign photo usable for preview")
	}
}
func TestBoundedPollingKeepsTaskForReconciliation(t *testing.T) {
	s, id := setup(t)
	p := &counted{}
	s.Renderer = p
	j, e := s.Preview(ctx, id, "SL-002", "demo")
	if e != nil {
		t.Fatal(e)
	}
	j.Polls = 60
	j.UpdatedAt = time.Now().Add(-time.Minute)
	_ = s.Repo.SaveJob(ctx, j)
	out, e := s.Poll(ctx, id, j.ID)
	if e != nil || out.Status != "timed_out" || out.TaskID != "private-task" || p.polls.Load() != 0 {
		t.Fatal("poll limit or task retention broken")
	}
}
