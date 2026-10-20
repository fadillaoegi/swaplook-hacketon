package application_test

import (
	"context"
	"errors"
	"swaplook/internal/application"
	"swaplook/internal/testkit"
	"testing"
	"time"
)

type failDeletion struct {
	application.Storage
	key string
}

func (s failDeletion) Delete(ctx context.Context, key string) error {
	if key == s.key {
		return errors.New("synthetic delete failure")
	}
	return s.Storage.Delete(ctx, key)
}

func TestExpiredMediaCleanupRetainsDecisionsAndActivePhotos(t *testing.T) {
	s, id := setup(t)
	expired := photo(t, s, id)
	if _, e := s.Decide(ctx, id, "SL-002", "selected", ""); e != nil {
		t.Fatal(e)
	}
	state, e := s.Repo.Session(ctx, id, false)
	if e != nil {
		t.Fatal(e)
	}
	state.ExpiresAt = time.Now().Add(-time.Hour)
	_ = s.Repo.SaveSession(ctx, state)
	active, e := s.Start(ctx)
	if e != nil {
		t.Fatal(e)
	}
	kept := photo(t, s, active.ID)
	repo := s.Repo.(*testkit.MemoryRepository)
	if n, e := application.PurgeExpiredMedia(ctx, repo, s.Storage); e != nil || n != 1 {
		t.Fatal("expired cleanup failed", e)
	}
	if _, e = s.Storage.Read(ctx, expired.ID); e == nil {
		t.Fatal("expired photo still on disk")
	}
	if _, e = s.Storage.Read(ctx, kept.ID); e != nil {
		t.Fatal("active photo deleted")
	}
	photos, e := repo.Photos(ctx, id)
	if e != nil || len(photos) != 0 {
		t.Fatal("expired media ownership retained")
	}
	summary, e := repo.Summary(ctx)
	if e != nil || summary.Choices != 1 {
		t.Fatal("anonymous decision history lost")
	}
}
func TestFailedExpiryDeletionRemainsRetryable(t *testing.T) {
	s, id := setup(t)
	p := photo(t, s, id)
	state, e := s.Repo.Session(ctx, id, false)
	if e != nil {
		t.Fatal(e)
	}
	state.ExpiresAt = time.Now().Add(-time.Hour)
	_ = s.Repo.SaveSession(ctx, state)
	repo := s.Repo.(*testkit.MemoryRepository)
	if _, e = application.PurgeExpiredMedia(ctx, repo, failDeletion{Storage: s.Storage, key: p.ID}); e == nil {
		t.Fatal("deletion failure hidden")
	}
	photos, e := repo.Photos(ctx, id)
	if e != nil || len(photos) != 1 {
		t.Fatal("ownership record lost before deleting file")
	}
	if _, e = application.PurgeExpiredMedia(ctx, repo, s.Storage); e != nil {
		t.Fatal("cleanup retry failed", e)
	}
	if _, e = s.Storage.Read(ctx, p.ID); e == nil {
		t.Fatal("retry left an expired photo")
	}
}
