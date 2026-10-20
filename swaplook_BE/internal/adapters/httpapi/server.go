package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"strings"
	"swaplook/internal/application"
	"swaplook/internal/domain"
	"time"
)

type Config struct {
	Origin, MerchantToken string
	Secure                bool
	Assets                fs.FS
	// TrustProxy uses the first X-Forwarded-For address; enable only behind a trusted reverse proxy.
	TrustProxy bool
	// ClientSalt keys per-client preview limits; addresses are never stored.
	ClientSalt []byte
}
type Server struct {
	Service *application.Service
	Config  Config
	limits  limiter
}

// class groups routes for request limiting; reads of public catalog data are not limited.
func class(r *http.Request) string {
	switch {
	case r.Method == "POST" && r.URL.Path == "/api/session":
		return "session"
	case r.Method == "POST" && r.URL.Path == "/api/photos":
		return "upload"
	case len(r.URL.Path) >= 14 && r.URL.Path[:14] == "/api/merchant/":
		return "merchant"
	case r.Method != "GET" && r.Method != "HEAD":
		return "write"
	}
	return ""
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		_, e := s.Service.Repo.Catalog(r.Context())
		if e != nil {
			s.fail(w, e)
			return
		}
		s.json(w, 200, map[string]any{"status": "ok", "renderer": s.Service.Active().Name(), "ai_available": s.Service.AIAvailable(), "demo": true})
	})
	mux.HandleFunc("GET /api/catalog", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Service.Repo.Catalog(r.Context())
		if e != nil {
			s.fail(w, e)
			return
		}
		s.json(w, 200, v)
	})
	mux.HandleFunc("POST /api/session", s.start)
	mux.HandleFunc("GET /api/session", s.state)
	mux.HandleFunc("DELETE /api/session", s.end)
	mux.HandleFunc("POST /api/shortlist", s.shortlist)
	mux.HandleFunc("POST /api/decisions", s.decide)
	mux.HandleFunc("POST /api/photos", s.upload)
	mux.HandleFunc("DELETE /api/photos", s.deletePhotos)
	mux.HandleFunc("POST /api/previews", s.preview)
	mux.HandleFunc("POST /api/previews/{id}/poll", s.poll)
	mux.HandleFunc("POST /api/previews/{id}/reconcile", s.reconcile)
	mux.HandleFunc("GET /api/images/{id}", s.image)
	mux.HandleFunc("GET /api/models", s.models)
	mux.HandleFunc("GET /api/models/{id}/image", s.modelImage)
	mux.HandleFunc("GET /api/merchant/summary", s.summary)
	// Public, read-only view of the same anonymous demo data; every write still needs the owner token.
	mux.HandleFunc("GET /api/merchant/demo-summary", s.demoSummary)
	mux.HandleFunc("PATCH /api/merchant/stock", s.stock)
	mux.HandleFunc("PATCH /api/merchant/ai", s.setAI)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		s.json(w, 404, map[string]string{"error": "API route not found."})
	})
	if s.Config.Assets != nil {
		mux.HandleFunc("/", s.static)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		if s.Config.Assets != nil {
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; font-src 'self'; frame-ancestors 'none'; form-action 'self'; base-uri 'none'; object-src 'none'")
		}
		if r.Method != "GET" && r.Method != "HEAD" && r.Header.Get("Origin") != "" && r.Header.Get("Origin") != s.Config.Origin {
			s.json(w, 403, map[string]string{"error": "Request origin is not allowed."})
			return
		}
		if !s.limits.allow(s.client(r), class(r)) {
			w.Header().Set("Retry-After", "60")
			s.json(w, 429, map[string]string{"error": "Too many requests from this connection. Wait a minute and try again.", "code": "rate_limited"})
			return
		}
		defer func() {
			if recover() != nil {
				slog.Error("request panic", "method", r.Method)
				s.json(w, 500, map[string]string{"error": "Unexpected server error."})
			}
		}()
		mux.ServeHTTP(w, r)
	})
}
func (s *Server) json(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func (s *Server) fail(w http.ResponseWriter, e error) {
	code := 500
	msg := "The server could not complete this request."
	kind := ""
	switch {
	case errors.Is(e, domain.ErrQuota):
		code, kind = 429, "quota"
		msg = "Today's AI preview allowance for this demo has been used. No generation was submitted. Compare catalog photos, or try again tomorrow."
	case errors.Is(e, domain.ErrUnavailable):
		code, kind = 409, "unavailable"
		msg = "This top is no longer available in your size at the shortlisted price. Choose another alternative or refresh your shortlist."
	case errors.Is(e, domain.ErrInvalid):
		code = 400
		msg = "Check your inputs, shortlist and photo consent."
	case errors.Is(e, domain.ErrNotFound):
		code = 404
		msg = "This session or item is unavailable. Start a new session if it expired."
	case errors.Is(e, domain.ErrConflict):
		code = 409
		msg = "Availability changed, or this session already has a decision. Refresh alternatives or start again."
	case errors.Is(e, domain.ErrLimit):
		code = 429
		msg = "The photo, preview or status-check limit has been reached. No additional generation was submitted."
	case errors.Is(e, domain.ErrProvider):
		code = 502
		msg = "The preview provider did not confirm the request. Your job is retained; generation will not be retried automatically."
	default:
		slog.Error("request failed", "class", "internal")
	}
	body := map[string]string{"error": msg}
	if kind != "" {
		body["code"] = kind
	}
	s.json(w, code, body)
}

// client returns a salted hash of the caller address for per-client limits.
func (s *Server) client(r *http.Request) string {
	if len(s.Config.ClientSalt) == 0 {
		return ""
	}
	addr := r.RemoteAddr
	if host, _, err := net.SplitHostPort(addr); err == nil {
		addr = host
	}
	if s.Config.TrustProxy {
		if f := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); f != "" {
			addr = f
		}
	}
	h := hmac.New(sha256.New, s.Config.ClientSalt)
	h.Write([]byte(addr))
	return hex.EncodeToString(h.Sum(nil))
}
func (s *Server) decode(w http.ResponseWriter, r *http.Request, out any) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		s.json(w, 415, map[string]string{"error": "JSON content type required."})
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16384)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(out); e != nil {
		s.fail(w, domain.ErrInvalid)
		return false
	}
	if d.Decode(new(any)) != io.EOF {
		s.fail(w, domain.ErrInvalid)
		return false
	}
	return true
}
func (s *Server) session(w http.ResponseWriter, r *http.Request) (string, bool) {
	c, e := r.Cookie("swaplook_session")
	if e != nil || len(c.Value) != 64 {
		s.json(w, 401, map[string]string{"error": "Start a shopper session first."})
		return "", false
	}
	if _, e = s.Service.Repo.Session(r.Context(), c.Value, false); e != nil {
		s.fail(w, e)
		return "", false
	}
	return c.Value, true
}
func (s *Server) merchant(w http.ResponseWriter, r *http.Request) bool {
	a := sha256.Sum256([]byte(r.Header.Get("Authorization")))
	b := sha256.Sum256([]byte("Bearer " + s.Config.MerchantToken))
	if len(s.Config.MerchantToken) < 24 || subtle.ConstantTimeCompare(a[:], b[:]) != 1 {
		s.json(w, 401, map[string]string{"error": "Enter a valid merchant access token."})
		return false
	}
	return true
}
func (s *Server) start(w http.ResponseWriter, r *http.Request) {
	v, e := s.Service.Start(r.Context())
	if e != nil {
		s.fail(w, e)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "swaplook_session", Value: v.ID, Path: "/api", HttpOnly: true, Secure: s.Config.Secure, SameSite: http.SameSiteLaxMode, MaxAge: 86400})
	s.json(w, 201, v)
}
func (s *Server) state(w http.ResponseWriter, r *http.Request) {
	id, ok := s.session(w, r)
	if !ok {
		return
	}
	v, e := s.Service.Repo.Session(r.Context(), id, false)
	if e != nil {
		s.fail(w, e)
		return
	}
	jobs, e := s.Service.Repo.Jobs(r.Context(), id)
	if e != nil {
		s.fail(w, e)
		return
	}
	photos, e := s.Service.Repo.Photos(r.Context(), id)
	if e != nil {
		s.fail(w, e)
		return
	}
	quota, e := s.Service.Quota(r.Context(), s.client(r))
	if e != nil {
		s.fail(w, e)
		return
	}
	s.json(w, 200, map[string]any{"session": v, "jobs": jobs, "photos": photos, "renderer": s.Service.Active().Name(), "quota": quota, "models": s.listModels()})
}
func (s *Server) end(w http.ResponseWriter, r *http.Request) {
	id, ok := s.session(w, r)
	if !ok {
		return
	}
	if e := s.Service.DeleteMedia(r.Context(), id); e != nil {
		s.fail(w, e)
		return
	}
	if e := s.Service.Repo.DeleteSession(r.Context(), id); e != nil {
		s.fail(w, e)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "swaplook_session", Value: "", Path: "/api", HttpOnly: true, Secure: s.Config.Secure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	w.WriteHeader(204)
}
func (s *Server) shortlist(w http.ResponseWriter, r *http.Request) {
	id, ok := s.session(w, r)
	if !ok {
		return
	}
	var c domain.Constraints
	if !s.decode(w, r, &c) {
		return
	}
	v, e := s.Service.Shortlist(r.Context(), id, c)
	if e != nil {
		s.fail(w, e)
		return
	}
	s.json(w, 200, v)
}
func (s *Server) decide(w http.ResponseWriter, r *http.Request) {
	id, ok := s.session(w, r)
	if !ok {
		return
	}
	var in struct {
		Product string `json:"product_id"`
		Kind    string `json:"kind"`
		Reason  string `json:"reason"`
	}
	if !s.decode(w, r, &in) {
		return
	}
	v, e := s.Service.Decide(r.Context(), id, in.Product, in.Kind, in.Reason)
	if e != nil {
		s.fail(w, e)
		return
	}
	s.json(w, 200, v)
}
func (s *Server) preview(w http.ResponseWriter, r *http.Request) {
	id, ok := s.session(w, r)
	if !ok {
		return
	}
	var in struct {
		Product string `json:"product_id"`
		Source  string `json:"source_id"`
	}
	if !s.decode(w, r, &in) {
		return
	}
	v, e := s.Service.PreviewFor(r.Context(), id, in.Product, in.Source, s.client(r))
	if e != nil {
		s.fail(w, e)
		return
	}
	s.json(w, 202, v)
}
func (s *Server) poll(w http.ResponseWriter, r *http.Request) {
	id, ok := s.session(w, r)
	if !ok {
		return
	}
	v, e := s.Service.Poll(r.Context(), id, r.PathValue("id"))
	if e != nil {
		s.fail(w, e)
		return
	}
	s.json(w, 200, v)
}
func (s *Server) reconcile(w http.ResponseWriter, r *http.Request) {
	id, ok := s.session(w, r)
	if !ok {
		return
	}
	v, e := s.Service.Reconcile(r.Context(), id, r.PathValue("id"))
	if e != nil {
		s.fail(w, e)
		return
	}
	s.json(w, 200, v)
}
func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	id, ok := s.session(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, (8<<20)+65536)
	if e := r.ParseMultipartForm(1 << 20); e != nil {
		s.fail(w, domain.ErrInvalid)
		return
	}
	defer r.MultipartForm.RemoveAll()
	f, _, e := r.FormFile("photo")
	if e != nil {
		s.fail(w, domain.ErrInvalid)
		return
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, (8<<20)+1))
	if e != nil {
		s.fail(w, domain.ErrInvalid)
		return
	}
	p, e := s.Service.Upload(r.Context(), id, b, r.FormValue("consent") == "true")
	if e != nil {
		s.fail(w, e)
		return
	}
	s.json(w, 201, p)
}
func (s *Server) deletePhotos(w http.ResponseWriter, r *http.Request) {
	id, ok := s.session(w, r)
	if !ok {
		return
	}
	if e := s.Service.DeleteMedia(r.Context(), id); e != nil {
		s.fail(w, e)
		return
	}
	w.WriteHeader(204)
}
func (s *Server) image(w http.ResponseWriter, r *http.Request) {
	id, ok := s.session(w, r)
	if !ok {
		return
	}
	b, e := s.Service.Image(r.Context(), id, r.PathValue("id"))
	if e != nil {
		s.fail(w, domain.ErrNotFound)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Disposition", "inline")
	w.WriteHeader(200)
	_, _ = w.Write(b)
}

// listModels returns model metadata only in actual mode; simulator previews always use the mannequin.
func (s *Server) listModels() []application.Model {
	if !s.Service.AIEnabled() || s.Service.Models == nil {
		return []application.Model{}
	}
	return s.Service.Models
}
func (s *Server) models(w http.ResponseWriter, r *http.Request) {
	s.json(w, 200, s.listModels())
}
func (s *Server) modelImage(w http.ResponseWriter, r *http.Request) {
	m, ok := s.Service.Model(application.ModelSource(r.PathValue("id")))
	if !ok {
		s.json(w, 404, map[string]string{"error": "This model is not available."})
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Disposition", "inline")
	w.WriteHeader(200)
	_, _ = w.Write(m.Image)
}
func (s *Server) demoSummary(w http.ResponseWriter, r *http.Request) {
	v, e := s.Service.Summary(r.Context())
	if e != nil {
		s.fail(w, e)
		return
	}
	s.json(w, 200, v)
}
func (s *Server) summary(w http.ResponseWriter, r *http.Request) {
	if !s.merchant(w, r) {
		return
	}
	v, e := s.Service.Summary(r.Context())
	if e != nil {
		s.fail(w, e)
		return
	}
	s.json(w, 200, v)
}
func (s *Server) setAI(w http.ResponseWriter, r *http.Request) {
	if !s.merchant(w, r) {
		return
	}
	var in struct {
		Enabled *bool `json:"enabled"`
	}
	if !s.decode(w, r, &in) {
		return
	}
	if in.Enabled == nil {
		s.fail(w, domain.ErrInvalid)
		return
	}
	if e := s.Service.SetAI(r.Context(), *in.Enabled); e != nil {
		s.fail(w, e)
		return
	}
	s.json(w, 200, map[string]any{"enabled": s.Service.AIEnabled(), "renderer": s.Service.Active().Name()})
}
func (s *Server) stock(w http.ResponseWriter, r *http.Request) {
	if !s.merchant(w, r) {
		return
	}
	var in struct {
		Product  string `json:"product_id"`
		Size     string `json:"size"`
		Quantity int    `json:"quantity"`
	}
	if !s.decode(w, r, &in) {
		return
	}
	if e := s.Service.Repo.SetStock(r.Context(), in.Product, in.Size, in.Quantity); e != nil {
		s.fail(w, e)
		return
	}
	s.json(w, 200, map[string]any{"updated": true, "checked_at": time.Now().UTC()})
}
