package auth

import (
	"context"
	"errors"
	"mini-messenger/internal/model"
	"testing"
	"time"
)

type refreshStore struct {
	session    model.Session
	failure    error
	storedHash string
}

func (s *refreshStore) StoreRefresh(_ context.Context, _ string, hash string) error {
	s.storedHash = hash
	return s.failure
}
func (s *refreshStore) RotateRefresh(context.Context, string, string, time.Time) (model.Session, error) {
	return s.session, s.failure
}

type refreshSecrets struct{}

func (refreshSecrets) Generate() (string, string, error) { return "raw-secret", "hash-only", nil }
func (refreshSecrets) Digest(raw string) (string, error) {
	if raw == "" {
		return "", model.ErrUnauthorized
	}
	return "old-hash", nil
}

type issuedTokens struct{ claims Claims }

func (t *issuedTokens) Issue(claims Claims) (string, error) { t.claims = claims; return "access", nil }
func (t *issuedTokens) Verify(string) (Claims, error)       { return t.claims, nil }
func TestRefreshLifetimeAndReplay(t *testing.T) {
	now := time.Now()
	repository := &refreshStore{session: model.Session{ID: "session", UserID: "user", ExpiresAt: now.Add(5 * time.Minute)}}
	connections := &connections{}
	tokens := &issuedTokens{}
	service := NewRefresh(repository, refreshSecrets{}, tokens, connections, clock{now})
	raw, err := service.Start(context.Background(), "session")
	if err != nil || raw != "raw-secret" || repository.storedHash != "hash-only" {
		t.Fatal("plaintext stored")
	}
	output, err := service.Execute(context.Background(), "old")
	if err != nil || !output.ExpiresAt.Equal(repository.session.ExpiresAt) {
		t.Fatal("absolute expiry extended")
	}
	repository.failure = model.ErrUnauthorized
	if _, err = service.Execute(context.Background(), "old"); !errors.Is(err, model.ErrUnauthorized) || !connections.closed {
		t.Fatal("replay did not close sockets")
	}
	connections.closed = false
	repository.failure = errors.New("database unavailable")
	if _, err = service.Execute(context.Background(), "old"); err == nil || connections.closed {
		t.Fatal("storage failure treated as revocation")
	}
}
