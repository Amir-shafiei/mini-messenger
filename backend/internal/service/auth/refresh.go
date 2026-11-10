package auth

import (
	"context"
	"errors"
	"mini-messenger/internal/model"
	"time"
)

type Refresh struct {
	repo        RefreshRepository
	secrets     RefreshSecrets
	tokens      TokenService
	connections SessionConnections
	clock       Clock
}

func NewRefresh(
	repository RefreshRepository,
	secrets RefreshSecrets,
	tokens TokenService,
	connections SessionConnections,
	clock Clock,
) *Refresh {
	return &Refresh{
		repo:        repository,
		secrets:     secrets,
		tokens:      tokens,
		connections: connections,
		clock:       clock,
	}
}

func (uc *Refresh) Start(ctx context.Context, sessionID string) (string, error) {
	raw, hash, err := uc.secrets.Generate()
	if err != nil {
		return "", err
	}
	if err = uc.repo.StoreRefresh(ctx, sessionID, hash); err != nil {
		return "", err
	}
	return raw, nil
}

type RefreshOutput struct {
	Token, RefreshToken, UserID string
	ExpiresAt, SessionExpiresAt time.Time
}

func (uc *Refresh) Execute(ctx context.Context, raw string) (RefreshOutput, error) {
	var out RefreshOutput
	hash, err := uc.secrets.Digest(raw)
	if err != nil {
		return out, model.ErrUnauthorized
	}
	next, nextHash, err := uc.secrets.Generate()
	if err != nil {
		return out, err
	}
	session, err := uc.repo.RotateRefresh(ctx, hash, nextHash, uc.clock.Now())
	if err != nil {
		if errors.Is(err, model.ErrUnauthorized) && session.ID != "" {
			uc.connections.DisconnectSession(session.ID)
		}
		return out, err
	}
	expires := uc.clock.Now().Add(15 * time.Minute)
	if session.ExpiresAt.Before(expires) {
		expires = session.ExpiresAt
	}
	token, err := uc.tokens.Issue(Claims{session.UserID, session.ID, expires})
	if err != nil {
		return out, err
	}
	return RefreshOutput{
		Token:            token,
		RefreshToken:     next,
		UserID:           session.UserID,
		ExpiresAt:        expires,
		SessionExpiresAt: session.ExpiresAt,
	}, nil
}
