package application

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"sort"
	"swaplook/internal/domain"
	"sync/atomic"
	"time"
)

type Service struct {
	Repo    Repository
	Storage Storage
	// Renderer is the configured provider (actual or simulator). Simulator is used for new
	// previews while an operator has switched AI previews off.
	Renderer  Renderer
	Simulator Renderer
	aiOff     atomic.Bool
	// Budget limits actual provider generations; zero values mean unlimited (simulator and tests).
	Budget Budget
	// Models are consented model photos shoppers can use instead of their own (actual mode only).
	Models []Model
}

// Model is a consented, normalized person photo selectable as source "model:<ID>".
type Model struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Note  string `json:"note"`
	Image []byte `json:"-"`
}

// ModelSource is the preview source identifier for a configured model.
func ModelSource(id string) string { return "model:" + id }

// Model returns a configured model by source identifier.
func (s *Service) Model(source string) (Model, bool) {
	for _, m := range s.Models {
		if ModelSource(m.ID) == source {
			return m, true
		}
	}
	return Model{}, false
}

// Budget protects provider credits. Daily and client windows start at UTC midnight.
type Budget struct {
	Daily, Total, PerClient int
}

func dayStart(t time.Time) time.Time { return t.UTC().Truncate(24 * time.Hour) }

const aiSetting = "ai_previews"

// Active is the renderer for new previews.
func (s *Service) Active() Renderer {
	if s.aiOff.Load() && s.Simulator != nil && s.AIAvailable() {
		return s.Simulator
	}
	return s.Renderer
}

// AIAvailable reports whether an actual provider is configured.
func (s *Service) AIAvailable() bool { return s.Renderer.Name() != "simulator" }

// AIEnabled reports whether new previews use the actual provider.
func (s *Service) AIEnabled() bool { return s.AIAvailable() && s.Active() == s.Renderer }

// LoadAISetting restores the persisted operator switch; missing means on.
func (s *Service) LoadAISetting(ctx context.Context) error {
	v, e := s.Repo.Setting(ctx, aiSetting)
	if errors.Is(e, domain.ErrNotFound) {
		return nil
	}
	s.aiOff.Store(v == "off")
	return e
}

// SetAI switches new previews between the actual provider and the simulator. Jobs already
// submitted keep polling their own provider.
func (s *Service) SetAI(ctx context.Context, enabled bool) error {
	if !s.AIAvailable() || s.Simulator == nil {
		return fmt.Errorf("%w: no actual preview provider is configured", domain.ErrInvalid)
	}
	value := "on"
	if !enabled {
		value = "off"
	}
	if e := s.Repo.SetSetting(ctx, aiSetting, value); e != nil {
		return e
	}
	s.aiOff.Store(!enabled)
	return nil
}

// rendererFor returns the renderer that owns an existing job.
func (s *Service) rendererFor(provider string) Renderer {
	if s.Renderer.Name() == provider {
		return s.Renderer
	}
	if s.Simulator != nil && s.Simulator.Name() == provider {
		return s.Simulator
	}
	return nil
}

func ID() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func (s *Service) Start(ctx context.Context) (domain.Session, error) {
	v := domain.Session{ID: ID(), Demo: true, ExpiresAt: time.Now().UTC().Add(24 * time.Hour)}
	return v, s.Repo.CreateSession(ctx, v)
}
func (s *Service) Shortlist(ctx context.Context, id string, c domain.Constraints) (out domain.Shortlist, err error) {
	err = s.Repo.Transaction(ctx, func(r Repository) error {
		state, e := r.Session(ctx, id, true)
		if e != nil {
			return e
		}
		if state.Decision != nil {
			return fmt.Errorf("%w: start a new session to explore again", domain.ErrConflict)
		}
		products, e := r.Catalog(ctx)
		if e != nil {
			return e
		}
		out, e = domain.Alternatives(products, c)
		if e != nil {
			return e
		}
		state.Shortlist = &out
		return r.SaveSession(ctx, state)
	})
	return
}
func (s *Service) Decide(ctx context.Context, id, product, kind, reason string) (out domain.Decision, err error) {
	if kind != "selected" && kind != "rejected" {
		return out, domain.ErrInvalid
	}
	if kind == "rejected" && reason != "style" && reason != "price" && reason != "no_match" && reason != "other" {
		return out, domain.ErrInvalid
	}
	err = s.Repo.Transaction(ctx, func(r Repository) error {
		state, e := r.Session(ctx, id, true)
		if e != nil {
			return e
		}
		if state.Decision != nil {
			out = *state.Decision
			if out.ProductID != product || out.Kind != kind {
				return domain.ErrConflict
			}
			return nil
		}
		if state.Shortlist == nil {
			return domain.ErrInvalid
		}
		c := state.Shortlist.Constraints
		out = domain.Decision{ID: ID(), ProductID: product, OriginalID: c.OriginalID, Size: c.Size, Kind: kind, Reason: reason, Demo: true, CreatedAt: time.Now().UTC()}
		if kind == "selected" {
			found := false
			for _, a := range state.Shortlist.Candidates {
				if a.Product.ID == product {
					found = true
				}
			}
			if !found {
				return domain.ErrInvalid
			}
			p, e := r.Product(ctx, product, c.Size)
			if e != nil {
				return e
			}
			if !domain.Eligible(p, c) {
				return domain.ErrUnavailable
			}
			out.Price = p.Price
		} else if product != "" {
			return domain.ErrInvalid
		}
		state.Decision = &out
		return r.SaveSession(ctx, state)
	})
	return
}

// providerFailure explains a provider error code without exposing provider free text.
func providerFailure(reason string) string {
	switch reason {
	case "":
		return "The preview provider could not process these images."
	case "error_pose":
		return "The provider could not see the chest and shoulders clearly (error_pose). Use a front-facing, upper-body photo; no new generation was submitted."
	case "error_no_face":
		return "The provider could not detect a face (error_no_face). Use a clear, front-facing photo; no new generation was submitted."
	default:
		return "The preview provider could not process these images (" + reason + "). No new generation was submitted."
	}
}

// NormalizeModel validates and normalizes a configured demo-model photo at startup.
func NormalizeModel(b []byte) ([]byte, error) { return normalize(b, 8<<20, 10<<20) }
func normalize(b []byte, inputLimit, outputLimit int) ([]byte, error) {
	if len(b) == 0 || len(b) > inputLimit {
		return nil, domain.ErrInvalid
	}
	config, format, e := image.DecodeConfig(bytes.NewReader(b))
	if e != nil || (format != "jpeg" && format != "png") || config.Width < 128 || config.Height < 128 || config.Width > 4096 || config.Height > 4096 || config.Width*config.Height > 12000000 {
		return nil, domain.ErrInvalid
	}
	img, _, e := image.Decode(bytes.NewReader(b))
	if e != nil {
		return nil, domain.ErrInvalid
	}
	if format == "jpeg" {
		img = orient(img, jpegOrientation(b))
	}
	// Go encodes non-RGBA images (e.g. decoded JPEG YCbCr) as 16-bit PNG, which the provider's
	// pose detection rejected; always store a standard 8-bit image.
	rgba := image.NewNRGBA(img.Bounds())
	draw.Draw(rgba, rgba.Bounds(), img, img.Bounds().Min, draw.Src)
	var out bytes.Buffer
	e = png.Encode(&out, rgba)
	if out.Len() >= outputLimit {
		return nil, domain.ErrInvalid
	}
	return out.Bytes(), e
}
func (s *Service) Upload(ctx context.Context, id string, b []byte, consent bool) (domain.Photo, error) {
	p := domain.Photo{}
	if !consent {
		return p, domain.ErrInvalid
	}
	b, e := normalize(b, 8<<20, 10<<20)
	if e != nil {
		return p, e
	}
	p = domain.Photo{ID: ID(), SessionID: id, CreatedAt: time.Now().UTC()}
	e = s.Repo.Transaction(ctx, func(r Repository) error {
		if _, e := r.Session(ctx, id, true); e != nil {
			return e
		}
		photos, e := r.Photos(ctx, id)
		if e != nil {
			return e
		}
		if len(photos) >= 3 {
			return domain.ErrLimit
		}
		if e = s.Storage.Put(ctx, p.ID, b); e != nil {
			return e
		}
		return r.AddPhoto(ctx, p)
	})
	if e != nil {
		_ = s.Storage.Delete(ctx, p.ID)
	}
	return p, e
}
func (s *Service) Preview(ctx context.Context, id, sku, source string) (domain.Job, error) {
	return s.PreviewFor(ctx, id, sku, source, "")
}

// PreviewFor creates or returns a preview. client is a salted hash used only for per-client limits.
func (s *Service) PreviewFor(ctx context.Context, id, sku, source, client string) (out domain.Job, err error) {
	fresh := false
	renderer := s.Active()
	actual := renderer.Name() != "simulator"
	var reusedImage []byte
	err = s.Repo.Transaction(ctx, func(r Repository) error {
		state, e := r.Session(ctx, id, true)
		if e != nil {
			return e
		}
		if state.Decision != nil {
			return domain.ErrConflict
		}
		if state.Shortlist == nil {
			return domain.ErrInvalid
		}
		valid := false
		for _, a := range state.Shortlist.Candidates {
			if a.Product.ID == sku {
				valid = true
			}
		}
		if !valid {
			return domain.ErrInvalid
		}
		if source == "demo" {
			if actual {
				return fmt.Errorf("%w: choose a model or an authorized person photograph", domain.ErrInvalid)
			}
		} else if _, ok := s.Model(source); ok {
			if !actual {
				return fmt.Errorf("%w: model photos are used only for actual previews", domain.ErrInvalid)
			}
		} else {
			photos, e := r.Photos(ctx, id)
			if e != nil {
				return e
			}
			owned := false
			for _, p := range photos {
				if p.ID == source {
					owned = true
				}
			}
			if !owned {
				return domain.ErrNotFound
			}
		}
		jobs, e := r.Jobs(ctx, id)
		if e != nil {
			return e
		}
		for _, j := range jobs {
			if j.ProductID == sku && j.SourceID == source && j.Size == state.Shortlist.Constraints.Size {
				out = j
				return nil
			}
		}
		now := time.Now().UTC()
		out = domain.Job{ID: ID(), SessionID: id, ProductID: sku, Size: state.Shortlist.Constraints.Size, SourceID: source, Provider: renderer.Name(), Status: "submitting", CreatedAt: now, UpdatedAt: now}
		// Identical model requests reuse an earlier provider result instead of spending credits.
		if model, ok := s.Model(source); ok {
			prior, e := r.ReusableModelJob(ctx, out.Provider, source, sku)
			if e == nil {
				if reusedImage, e = s.Storage.Read(ctx, prior.ID); e == nil {
					if e = s.Storage.Put(ctx, out.ID, reusedImage); e != nil {
						return e
					}
					out.Status, out.Reused, out.Image = "success", true, "/api/images/"+out.ID
					out.Message = "Reused YouCam result for model " + model.Name + ", generated " + prior.UpdatedAt.Format("2 Jan 15:04 UTC") + ". No new generation was submitted."
					return r.SaveJob(ctx, out)
				}
				reusedImage = nil
			} else if !errors.Is(e, domain.ErrNotFound) {
				return e
			}
		}
		generated := 0
		for _, j := range jobs {
			if !j.Reused {
				generated++
			}
		}
		if generated >= 3 {
			return domain.ErrLimit
		}
		if actual {
			if e = s.reserve(ctx, r, out, client); e != nil {
				return e
			}
		}
		fresh = true
		return r.SaveJob(ctx, out)
	})
	if err != nil && reusedImage != nil {
		_ = s.Storage.Delete(context.WithoutCancel(ctx), out.ID)
	}
	if err != nil || !fresh {
		return
	}
	var src []byte
	var task string
	var e error
	submitted := false
	if model, ok := s.Model(source); ok {
		src = model.Image
	} else if source != "demo" {
		src, e = s.Storage.Read(ctx, source)
	}
	if e == nil {
		submitted = true
		submitCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
		task, e = renderer.Submit(submitCtx, RenderInput{SKU: sku, Source: src})
		cancel()
	}
	// Persist the provider task even if the browser disconnected during submission.
	out.TaskID = task
	out.Status = "pending"
	if e != nil {
		out.Status = "unknown"
		out.Message = "Preview submission was not confirmed. Do not retry generation; check provider records."
		if !submitted {
			out.Status = "error"
			out.Message = "The source photo is unavailable. No generation was submitted."
		}
	}
	out.UpdatedAt = time.Now().UTC()
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	err = s.Repo.Transaction(persistCtx, func(r Repository) error {
		if _, err := r.Session(persistCtx, id, true); err != nil {
			return err
		}
		current, err := r.Job(persistCtx, id, out.ID, true)
		if err != nil {
			return err
		}
		if current.Status != "submitting" && !(current.Status == "unknown" && task != "" && current.TaskID == "") {
			out = current
			return nil
		}
		return r.SaveJob(persistCtx, out)
	})
	if err == nil && e != nil {
		err = domain.ErrProvider
	}
	return
}

// reserve records one provider generation if the configured allowance permits it.
func (s *Service) reserve(ctx context.Context, r Repository, job domain.Job, client string) error {
	if e := r.LockGenerations(ctx); e != nil {
		return e
	}
	today, total, mine, e := r.GenerationCounts(ctx, job.Provider, dayStart(job.CreatedAt), client)
	if e != nil {
		return e
	}
	b := s.Budget
	if (b.Daily > 0 && today >= b.Daily) || (b.Total > 0 && total >= b.Total) || (b.PerClient > 0 && client != "" && mine >= b.PerClient) {
		return domain.ErrQuota
	}
	return r.AddGeneration(ctx, domain.Generation{ID: job.ID, Provider: job.Provider, ClientKey: client, CreatedAt: job.CreatedAt})
}

// Quota reports remaining actual generations for the shopper UI; -1 means unlimited.
func (s *Service) Quota(ctx context.Context, client string) (map[string]int, error) {
	out := map[string]int{"remaining_today": -1, "client_remaining": -1}
	if !s.AIEnabled() {
		return out, nil
	}
	today, total, mine, e := s.Repo.GenerationCounts(ctx, s.Renderer.Name(), dayStart(time.Now()), client)
	if e != nil {
		return out, e
	}
	left := func(limit, used int) int {
		if limit <= 0 {
			return -1
		}
		return max(limit-used, 0)
	}
	out["remaining_today"] = left(s.Budget.Daily, today)
	if t := left(s.Budget.Total, total); t >= 0 && (out["remaining_today"] < 0 || t < out["remaining_today"]) {
		out["remaining_today"] = t
	}
	if client != "" {
		out["client_remaining"] = left(s.Budget.PerClient, mine)
	}
	return out, nil
}

// Summary adds substitution insight and preview usage to the merchant's demo summary.
func (s *Service) Summary(ctx context.Context) (domain.Summary, error) {
	out, e := s.Repo.Summary(ctx)
	if e != nil {
		return out, e
	}
	counts := map[[2]string]int{}
	for _, d := range out.Decisions {
		if d.Kind == "selected" {
			counts[[2]string{d.OriginalID, d.ProductID}]++
		}
	}
	out.Substitutions = []domain.Substitution{}
	for k, n := range counts {
		out.Substitutions = append(out.Substitutions, domain.Substitution{OriginalID: k[0], ProductID: k[1], Count: n})
	}
	sort.Slice(out.Substitutions, func(i, j int) bool {
		a, b := out.Substitutions[i], out.Substitutions[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		return a.OriginalID+a.ProductID < b.OriginalID+b.ProductID
	})
	out.Previews, e = s.Repo.PreviewStats(ctx, s.Renderer.Name(), dayStart(time.Now()))
	if e != nil {
		return out, e
	}
	p := &out.Previews
	p.Provider, p.DailyLimit, p.TotalLimit = s.Renderer.Name(), s.Budget.Daily, s.Budget.Total
	p.Available, p.Enabled = s.AIAvailable(), s.AIEnabled()
	p.RemainingToday, p.RemainingTotal = -1, -1
	if s.Budget.Daily > 0 {
		p.RemainingToday = max(s.Budget.Daily-p.Today, 0)
	}
	if s.Budget.Total > 0 {
		p.RemainingTotal = max(s.Budget.Total-p.Generations, 0)
	}
	return out, nil
}

// Poll checks an existing task. A persisted lease prevents overlapping provider requests.
func (s *Service) Poll(ctx context.Context, id, jobID string) (domain.Job, error) {
	return s.poll(ctx, id, jobID, false)
}

// Reconcile spends one of three additional status checks, never a new generation.
func (s *Service) Reconcile(ctx context.Context, id, jobID string) (domain.Job, error) {
	return s.poll(ctx, id, jobID, true)
}

func (s *Service) poll(ctx context.Context, id, jobID string, reconcile bool) (out domain.Job, err error) {
	claimed := false
	err = s.Repo.Transaction(ctx, func(r Repository) error {
		if _, e := r.Session(ctx, id, true); e != nil {
			return e
		}
		j, e := r.Job(ctx, id, jobID, true)
		if e != nil {
			return e
		}
		out = j
		if s.rendererFor(j.Provider) == nil && (j.Status == "pending" || j.Status == "submitting" || j.Status == "timed_out") {
			j.Status = "provider_changed"
			j.Message = "Preview mode changed on the server. No request was sent to a different provider. Start a new session."
			out = j
			return r.SaveJob(ctx, j)
		}
		now := time.Now().UTC()
		if j.Status == "submitting" && now.Sub(j.CreatedAt) > 2*time.Minute {
			j.Status = "unknown"
			j.Message = "Submission was interrupted. Check provider records; generation will not be resubmitted."
			out = j
			return r.SaveJob(ctx, j)
		}
		if j.Status != "pending" && !(reconcile && j.Status == "timed_out") {
			return nil
		}
		if j.PollLeaseUntil.After(now) {
			return nil
		}
		wait := 10 * time.Second
		if j.Provider == "simulator" {
			wait = 500 * time.Millisecond
		}
		if now.Sub(j.UpdatedAt) < wait {
			return nil
		}
		if reconcile && j.Status == "timed_out" {
			if j.Reconciliations >= 3 {
				return domain.ErrLimit
			}
			if j.TaskID == "" {
				return domain.ErrInvalid
			}
			j.Reconciliations++
		} else if j.Polls >= 60 {
			j.Status = "timed_out"
			j.Message = "Automatic polling paused. You can check the same task up to three more times without creating a new preview."
			j.UpdatedAt = now
			out = j
			return r.SaveJob(ctx, j)
		} else {
			j.Polls++
		}
		j.UpdatedAt = now
		j.PollLeaseUntil = now.Add(90 * time.Second)
		out = j
		claimed = true
		return r.SaveJob(ctx, j)
	})
	if err != nil || !claimed {
		return
	}
	pollCtx, cancel := context.WithTimeout(ctx, 85*time.Second)
	defer cancel()
	result, providerErr := s.rendererFor(out.Provider).Poll(pollCtx, out.TaskID)
	var img []byte
	if providerErr == nil && result.Status == "success" && out.Provider != "simulator" {
		img, providerErr = normalize(result.Image, 10<<20, 20<<20)
	}
	return s.finishPoll(ctx, id, out, result, img, providerErr)
}

func (s *Service) finishPoll(ctx context.Context, id string, claim domain.Job, result RenderResult, img []byte, providerErr error) (out domain.Job, err error) {
	out = claim
	wroteImage := false
	err = s.Repo.Transaction(ctx, func(r Repository) error {
		if _, e := r.Session(ctx, id, true); e != nil {
			return e
		}
		current, e := r.Job(ctx, id, claim.ID, true)
		if e != nil {
			return e
		}
		out = current
		// Deletion, a mode change or a later lease must win over a delayed response.
		if (current.Status != "pending" && current.Status != "timed_out") || !current.PollLeaseUntil.Equal(claim.PollLeaseUntil) {
			return nil
		}
		current.PollLeaseUntil = time.Time{}
		if providerErr != nil {
			current.Message = "Status check failed. The same task is retained; no new generation was submitted."
			out = current
			return r.SaveJob(ctx, current)
		}
		switch result.Status {
		case "success":
			if current.Provider == "simulator" {
				current.Image = "/catalog/preview-" + current.ProductID + ".svg"
			} else {
				if e = s.Storage.Put(ctx, current.ID, img); e != nil {
					return e
				}
				wroteImage = true
				current.Image = "/api/images/" + current.ID
			}
			current.Status = "success"
			current.Message = "Illustrative preview; physical fit is not guaranteed."
		case "error":
			current.Status = "error"
			current.Message = providerFailure(result.Reason)
		case "pending":
			if current.Status == "timed_out" {
				current.Message = "The same task is still processing. No new generation was submitted."
			}
		default:
			return domain.ErrProvider
		}
		out = current
		return r.SaveJob(ctx, current)
	})
	if err != nil && wroteImage {
		_ = s.Storage.Delete(context.WithoutCancel(ctx), claim.ID)
	}
	if err == nil && providerErr != nil && (out.Status == "pending" || out.Status == "timed_out") {
		err = domain.ErrProvider
	}
	return
}
func (s *Service) Image(ctx context.Context, id, key string) ([]byte, error) {
	if _, e := s.Repo.Session(ctx, id, false); e != nil {
		return nil, e
	}
	photos, e := s.Repo.Photos(ctx, id)
	if e != nil {
		return nil, e
	}
	for _, p := range photos {
		if p.ID == key {
			return s.Storage.Read(ctx, key)
		}
	}
	j, e := s.Repo.Job(ctx, id, key, false)
	if e != nil || j.Status != "success" || j.Provider == "simulator" {
		return nil, domain.ErrNotFound
	}
	return s.Storage.Read(ctx, key)
}
func (s *Service) DeleteMedia(ctx context.Context, id string) error {
	return s.Repo.Transaction(ctx, func(r Repository) error {
		if _, e := r.Session(ctx, id, true); e != nil {
			return e
		}
		photos, e := r.Photos(ctx, id)
		if e != nil {
			return e
		}
		jobs, e := r.Jobs(ctx, id)
		if e != nil {
			return e
		}
		for _, p := range photos {
			if e = s.Storage.Delete(ctx, p.ID); e != nil {
				return e
			}
		}
		for _, j := range jobs {
			if e = s.Storage.Delete(ctx, j.ID); e != nil {
				return e
			}
		}
		return r.DeleteMedia(ctx, id)
	})
}
