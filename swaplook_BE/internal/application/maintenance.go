package application

import (
	"context"
	"swaplook/internal/domain"
)

// MediaMaintenance separates operational cleanup from active shopper use cases.
type MediaMaintenance interface {
	ExpiredSessions(context.Context) ([]string, error)
	Photos(context.Context, string) ([]domain.Photo, error)
	Jobs(context.Context, string) ([]domain.Job, error)
	PurgeExpiredMedia(context.Context, string) error
}

func PurgeExpiredMedia(ctx context.Context, repo MediaMaintenance, store Storage) (int, error) {
	ids, err := repo.ExpiredSessions(ctx)
	if err != nil {
		return 0, err
	}
	cleaned := 0
	for _, id := range ids {
		photos, err := repo.Photos(ctx, id)
		if err != nil {
			return cleaned, err
		}
		jobs, err := repo.Jobs(ctx, id)
		if err != nil {
			return cleaned, err
		}
		// Keep ownership records until all deletions succeed, so a later run can retry.
		for _, p := range photos {
			if err = store.Delete(ctx, p.ID); err != nil {
				return cleaned, err
			}
		}
		for _, j := range jobs {
			if err = store.Delete(ctx, j.ID); err != nil {
				return cleaned, err
			}
		}
		if err = repo.PurgeExpiredMedia(ctx, id); err != nil {
			return cleaned, err
		}
		cleaned++
	}
	return cleaned, nil
}
