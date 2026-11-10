package integration

import (
	"context"
	"database/sql"
	"errors"
	"mini-messenger/internal/adapter/postgres"
	"mini-messenger/internal/adapter/security"
	"mini-messenger/internal/model"
	"sync"
	"testing"
	"time"
)

func testRefreshRotation(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx := context.Background()
	repository := postgres.NewSessionRepository(db)
	secrets := security.RefreshSecrets{}
	newSession := func(expires time.Time) (model.Session, string) {
		t.Helper()
		id, err := (security.IDs{}).New()
		if err != nil {
			t.Fatal(err)
		}
		session := model.Session{ID: id, UserID: "00000000-0000-4000-8000-000000000001", ExpiresAt: expires}
		if err = repository.Create(ctx, session); err != nil {
			t.Fatal(err)
		}
		_, hash, err := secrets.Generate()
		if err != nil {
			t.Fatal(err)
		}
		if err = repository.StoreRefresh(ctx, id, hash); err != nil {
			t.Fatal(err)
		}
		return session, hash
	}
	nextHash := func() string {
		t.Helper()
		_, hash, err := secrets.Generate()
		if err != nil {
			t.Fatal(err)
		}
		return hash
	}
	t.Run("rotation and replay revoke family", func(t *testing.T) {
		session, oldHash := newSession(time.Now().Add(time.Hour))
		newHash := nextHash()
		rotated, err := repository.RotateRefresh(ctx, oldHash, newHash, time.Now())
		if err != nil || rotated.ID != session.ID {
			t.Fatal("rotation", err)
		}
		var stored string
		if err = db.QueryRow(`SELECT token_hash FROM refresh_tokens WHERE token_hash=$1`, newHash).Scan(&stored); err != nil || len(stored) != 64 {
			t.Fatal("hash storage", err)
		}
		replay, err := repository.RotateRefresh(ctx, oldHash, nextHash(), time.Now())
		if !errors.Is(err, model.ErrUnauthorized) || replay.ID != session.ID {
			t.Fatal("replay accepted", err)
		}
		current, err := repository.Find(ctx, session.ID)
		if err != nil || !current.Revoked {
			t.Fatal("replay did not revoke", err)
		}
		if _, err = repository.RotateRefresh(ctx, newHash, nextHash(), time.Now()); !errors.Is(err, model.ErrUnauthorized) {
			t.Fatal("family survived replay")
		}
	})
	t.Run("expired and logout cannot refresh", func(t *testing.T) {
		session, hash := newSession(time.Now().Add(time.Minute))
		if _, err := repository.RotateRefresh(ctx, hash, nextHash(), time.Now().Add(time.Hour)); !errors.Is(err, model.ErrUnauthorized) {
			t.Fatal("expired session renewed")
		}
		if err := repository.Revoke(ctx, session.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := repository.RotateRefresh(ctx, hash, nextHash(), time.Now()); !errors.Is(err, model.ErrUnauthorized) {
			t.Fatal("logout renewed")
		}
	})
	t.Run("concurrent reuse is detected", func(t *testing.T) {
		session, hash := newSession(time.Now().Add(time.Hour))
		hashes := []string{nextHash(), nextHash()}
		var group sync.WaitGroup
		results := make(chan error, 2)
		for _, next := range hashes {
			group.Add(1)
			go func(next string) {
				defer group.Done()
				_, err := repository.RotateRefresh(ctx, hash, next, time.Now())
				results <- err
			}(next)
		}
		group.Wait()
		close(results)
		success, rejected := 0, 0
		for err := range results {
			if err == nil {
				success++
			} else if errors.Is(err, model.ErrUnauthorized) {
				rejected++
			} else {
				t.Fatal(err)
			}
		}
		if success != 1 || rejected != 1 {
			t.Fatal("concurrent reuse", success, rejected)
		}
		current, err := repository.Find(ctx, session.ID)
		if err != nil || !current.Revoked {
			t.Fatal("family not revoked")
		}
	})
}
