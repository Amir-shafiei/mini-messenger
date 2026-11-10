package auth

import (
	"context"
	"errors"
	"mini-messenger/internal/model"
	"time"
)

type Login struct {
	users     UserRepository
	sessions  SessionRepository
	tokens    TokenService
	passwords PasswordVerifier
	clock     Clock
	ids       IDGenerator
	ttl       time.Duration
}

func NewLogin(
	users UserRepository,
	sessions SessionRepository,
	tokens TokenService,
	passwords PasswordVerifier,
	clock Clock,
	ids IDGenerator,
	ttl time.Duration,
) *Login {
	return &Login{
		users:     users,
		sessions:  sessions,
		tokens:    tokens,
		passwords: passwords,
		clock:     clock,
		ids:       ids,
		ttl:       ttl,
	}
}

type LoginInput struct{ Username, Password string }

const SessionLifetime = 7 * 24 * time.Hour

type LoginOutput struct {
	SessionID        string
	SessionExpiresAt time.Time
	Token            string
	User             model.User
	ExpiresAt        time.Time
}

func (uc *Login) Execute(ctx context.Context, in LoginInput) (LoginOutput, error) {
	var out LoginOutput
	if in.Username == "" || in.Password == "" {
		return out, model.ErrUnauthorized
	}
	cred, err := uc.users.FindCredentials(ctx, in.Username)
	if errors.Is(err, model.ErrNotFound) {
		// Empty hash asks the adapter to perform a dummy comparison.
		uc.passwords.Compare("", in.Password)
		return out, model.ErrUnauthorized
	}
	if err != nil {
		return out, err
	}
	if !uc.passwords.Compare(cred.PasswordHash, in.Password) {
		return out, model.ErrUnauthorized
	}
	id, err := uc.ids.New()
	if err != nil {
		return out, err
	}
	session := model.Session{ID: id, UserID: cred.User.ID, ExpiresAt: uc.clock.Now().Add(SessionLifetime)}
	accessExpires := uc.clock.Now().Add(uc.ttl)
	token, err := uc.tokens.Issue(Claims{session.UserID, session.ID, accessExpires})
	if err != nil {
		return out, err
	}
	if err = uc.sessions.Create(ctx, session); err != nil {
		return out, err
	}
	return LoginOutput{
		Token:            token,
		User:             cred.User,
		ExpiresAt:        accessExpires,
		SessionID:        session.ID,
		SessionExpiresAt: session.ExpiresAt,
	}, nil
}
