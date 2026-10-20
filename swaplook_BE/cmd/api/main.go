package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"swaplook/internal/adapters/httpapi"
	"swaplook/internal/adapters/postgres"
	"swaplook/internal/adapters/renderer"
	"swaplook/internal/adapters/storage"
	"swaplook/internal/application"
	"syscall"
	"time"
)

func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
func intEnv(k string, fallback int) int {
	if v, err := strconv.Atoi(os.Getenv(k)); err == nil && v >= 0 {
		return v
	}
	return fallback
}

var modelID = regexp.MustCompile(`^[a-z0-9-]{1,32}$`)

// loadModels reads models.json from dir: [{"id","name","note","file"}]. Files must sit directly in dir.
func loadModels(dir string) ([]application.Model, error) {
	b, err := os.ReadFile(filepath.Join(dir, "models.json"))
	if err != nil {
		return nil, errors.New("DEMO_MODELS_DIR must contain a readable models.json")
	}
	var entries []struct{ ID, Name, Note, File string }
	if err = json.Unmarshal(b, &entries); err != nil || len(entries) > 8 {
		return nil, errors.New("models.json must be a JSON list of at most eight models")
	}
	out := make([]application.Model, 0, len(entries))
	seen := map[string]bool{}
	for _, m := range entries {
		if !modelID.MatchString(m.ID) || seen[m.ID] || m.Name == "" || len(m.Name) > 40 || len(m.Note) > 120 || m.File != filepath.Base(m.File) || strings.HasPrefix(m.File, ".") {
			return nil, errors.New("models.json has an invalid id, name, note or file name")
		}
		img, err := os.ReadFile(filepath.Join(dir, m.File))
		if err != nil {
			return nil, errors.New("a model photo listed in models.json is unreadable")
		}
		if img, err = application.NormalizeModel(img); err != nil {
			return nil, errors.New("model photos must be JPEG/PNG within the upload limits")
		}
		seen[m.ID] = true
		out = append(out, application.Model{ID: m.ID, Name: m.Name, Note: m.Note, Image: img})
	}
	return out, nil
}
func main() {
	if e := run(); e != nil {
		slog.Error("startup failed", "reason", e.Error())
		os.Exit(1)
	}
}
func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if len(os.Getenv("MERCHANT_TOKEN")) < 24 {
		return errors.New("MERCHANT_TOKEN must contain at least 24 characters")
	}
	dbctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	repo, e := postgres.Open(dbctx, os.Getenv("DATABASE_URL"))
	if e != nil {
		return errors.New("cannot connect to PostgreSQL; check DATABASE_URL and database availability")
	}
	defer repo.Close()
	if e = repo.Migrate(dbctx); e != nil {
		return e
	}
	store := storage.Local{Root: env("PRIVATE_STORAGE_DIR", "../.data/images")}
	var provider application.Renderer
	switch env("RENDERER_MODE", "simulator") {
	case "simulator":
		provider = renderer.Simulator{}
	case "youcam":
		if os.Getenv("YOUCAM_API_KEY") == "" {
			return errors.New("actual mode requires YOUCAM_API_KEY; simulator fallback is disabled")
		}
		provider = renderer.NewYouCam(os.Getenv("YOUCAM_API_KEY"), env("YOUCAM_GARMENT_DIR", "../.data/garments"))
	default:
		return errors.New("RENDERER_MODE must be simulator or youcam")
	}
	svc := &application.Service{Repo: repo, Storage: store, Renderer: provider}
	if provider.Name() != "simulator" {
		// Lets an operator switch AI previews off to save credits without a restart.
		svc.Simulator = renderer.Simulator{}
	}
	if e = svc.LoadAISetting(dbctx); e != nil {
		return errors.New("cannot read the AI preview setting")
	}
	if provider.Name() != "simulator" {
		// Conservative defaults protect the 1,000-unit hackathon allowance; override per deployment.
		svc.Budget = application.Budget{Daily: intEnv("PREVIEW_DAILY_LIMIT", 25), Total: intEnv("PREVIEW_TOTAL_LIMIT", 150), PerClient: intEnv("PREVIEW_CLIENT_DAILY_LIMIT", 5)}
	}
	if dir := os.Getenv("DEMO_MODELS_DIR"); dir != "" {
		if svc.Models, e = loadModels(dir); e != nil {
			return e
		}
	}
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	// Run immediately, then periodically; anonymous demo decision history is retained.
	if _, e = application.PurgeExpiredMedia(ctx, repo, store); e != nil {
		return errors.New("expired image cleanup failed; records retained for retry")
	}
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cleanupCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
				n, err := application.PurgeExpiredMedia(cleanupCtx, repo, store)
				cancel()
				if err != nil {
					slog.Error("expired image cleanup failed; will retry")
				} else if n > 0 {
					slog.Info("expired image cleanup complete", "sessions", n)
				}
			}
		}
	}()
	// The per-process salt makes client keys unlinkable across restarts.
	api := &httpapi.Server{Service: svc, Config: httpapi.Config{Origin: env("APP_ORIGIN", "http://127.0.0.1:5173"), MerchantToken: os.Getenv("MERCHANT_TOKEN"), Secure: os.Getenv("COOKIE_SECURE") == "true", TrustProxy: os.Getenv("TRUST_PROXY") == "true", ClientSalt: salt}}
	if dir := os.Getenv("STATIC_DIR"); dir != "" {
		if info, err := os.Stat(dir + "/index.html"); err != nil || !info.Mode().IsRegular() {
			return errors.New("STATIC_DIR must contain the built frontend index.html")
		}
		api.Config.Assets = os.DirFS(dir)
	}
	server := &http.Server{Addr: env("HTTP_ADDR", "127.0.0.1:8088"), Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 60 * time.Second, WriteTimeout: 150 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	slog.Info("SwapLook API ready", "address", server.Addr, "renderer", provider.Name(), "ai_enabled", svc.AIEnabled(), "daily_limit", svc.Budget.Daily, "total_limit", svc.Budget.Total, "client_limit", svc.Budget.PerClient, "models", len(svc.Models))
	if e = server.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
		return errors.New("HTTP server could not start")
	}
	return nil
}
