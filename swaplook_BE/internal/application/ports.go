package application

import (
	"context"
	"swaplook/internal/domain"
	"time"
)

type Repository interface {
	Transaction(context.Context, func(Repository) error) error
	Catalog(context.Context) ([]domain.Product, error)
	Product(context.Context, string, string) (domain.Product, error)
	CreateSession(context.Context, domain.Session) error
	Session(context.Context, string, bool) (domain.Session, error)
	SaveSession(context.Context, domain.Session) error
	DeleteSession(context.Context, string) error
	SetStock(context.Context, string, string, int) error
	Summary(context.Context) (domain.Summary, error)
	AddPhoto(context.Context, domain.Photo) error
	Photos(context.Context, string) ([]domain.Photo, error)
	DeleteMedia(context.Context, string) error
	Jobs(context.Context, string) ([]domain.Job, error)
	Job(context.Context, string, string, bool) (domain.Job, error)
	SaveJob(context.Context, domain.Job) error
	// LockGenerations serializes quota checks until the surrounding transaction ends.
	LockGenerations(context.Context) error
	// GenerationCounts returns ledger counts for a provider: since a time, in total, and for one client since that time.
	GenerationCounts(ctx context.Context, provider string, since time.Time, client string) (sinceCount, total, clientCount int, err error)
	AddGeneration(context.Context, domain.Generation) error
	// ReusableModelJob finds a successful result for a model source and SKU in any session.
	ReusableModelJob(ctx context.Context, provider, source, sku string) (domain.Job, error)
	PreviewStats(ctx context.Context, provider string, since time.Time) (domain.PreviewStats, error)
	Setting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string) error
}
type Storage interface {
	Put(context.Context, string, []byte) error
	Read(context.Context, string) ([]byte, error)
	Delete(context.Context, string) error
}
type RenderInput struct {
	SKU    string
	Source []byte
}
type RenderResult struct {
	Status string
	Image  []byte
	// Reason is the provider's machine-readable failure code, never free text or URLs.
	Reason string
}
type Renderer interface {
	Name() string
	Submit(context.Context, RenderInput) (string, error)
	Poll(context.Context, string) (RenderResult, error)
}
