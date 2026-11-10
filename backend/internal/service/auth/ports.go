package auth

import (
	"context"
	"mini-messenger/internal/model"
	"time"
)

type UserRepository interface {
	FindCredentials(context.Context, string) (model.Credentials, error)
}
type SessionRepository interface {
	Create(context.Context, model.Session) error
	Find(context.Context, string) (model.Session, error)
	Revoke(context.Context, string) error
}
type Claims struct {
	UserID, SessionID string
	ExpiresAt         time.Time
}

// Verify must validate signature, pinned algorithm, issuer, audience and expiry.
type TokenService interface {
	Issue(Claims) (string, error)
	Verify(string) (Claims, error)
}
type PasswordVerifier interface {
	Compare(hash, plaintext string) bool
}
type SessionConnections interface{ DisconnectSession(string) }
type Clock interface{ Now() time.Time }
type IDGenerator interface{ New() (string, error) }
type Principal struct {
	UserID, SessionID string
	ExpiresAt         time.Time
}

// Rotate serializes with logout and revokes the session if a consumed token is reused.
type RefreshRepository interface {
	StoreRefresh(context.Context, string, string) error
	RotateRefresh(context.Context, string, string, time.Time) (model.Session, error)
}
type RefreshSecrets interface {
	Generate() (string, string, error)
	Digest(string) (string, error)
}
