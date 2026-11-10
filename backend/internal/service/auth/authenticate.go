package auth

import (
	"context"
	"errors"
	"mini-messenger/internal/model"
)

type Authenticate struct {
	sessions SessionRepository
	tokens   TokenService
	clock    Clock
}

func NewAuthenticate(sessions SessionRepository, tokens TokenService, clock Clock) *Authenticate {
	return &Authenticate{sessions: sessions, tokens: tokens, clock: clock}
}
func (uc *Authenticate) Execute(ctx context.Context, token string) (Principal, error) {
	claims, err := uc.tokens.Verify(token)
	if err != nil || claims.UserID == "" || claims.SessionID == "" || !uc.clock.Now().Before(claims.ExpiresAt) {
		return Principal{}, model.ErrUnauthorized
	}
	// A valid signature alone cannot detect logout; always check the live session.
	s, err := uc.sessions.Find(ctx, claims.SessionID)
	if errors.Is(err, model.ErrNotFound) {
		return Principal{}, model.ErrUnauthorized
	}
	if err != nil {
		return Principal{}, err
	}
	if s.ID != claims.SessionID || s.UserID != claims.UserID || !s.Active(uc.clock.Now()) {
		return Principal{}, model.ErrUnauthorized
	}
	expires := s.ExpiresAt
	if claims.ExpiresAt.Before(expires) {
		expires = claims.ExpiresAt
	}
	return Principal{s.UserID, s.ID, expires}, nil
}
