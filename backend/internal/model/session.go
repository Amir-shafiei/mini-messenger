package model

import "time"

type Session struct {
	ID, UserID string
	ExpiresAt  time.Time
	Revoked    bool
}

func (s Session) Active(now time.Time) bool { return !s.Revoked && now.Before(s.ExpiresAt) }
