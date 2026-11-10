package auth

import (
	"context"
	"mini-messenger/internal/model"
)

type Logout struct {
	sessions    SessionRepository
	connections SessionConnections
}

func NewLogout(sessions SessionRepository, connections SessionConnections) *Logout {
	return &Logout{sessions: sessions, connections: connections}
}

// The caller must supply an authenticated principal, never request-body identity.
func (uc *Logout) Execute(ctx context.Context, p Principal) error {
	if p.UserID == "" || p.SessionID == "" {
		return model.ErrUnauthorized
	}
	s, err := uc.sessions.Find(ctx, p.SessionID)
	if err != nil {
		return err
	}
	if s.UserID != p.UserID {
		return model.ErrUnauthorized
	}
	if err = uc.sessions.Revoke(ctx, p.SessionID); err != nil {
		return err
	}
	uc.connections.DisconnectSession(p.SessionID)
	return nil
}
