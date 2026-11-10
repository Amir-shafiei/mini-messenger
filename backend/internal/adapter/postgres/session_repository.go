package postgres

import (
	"context"
	"database/sql"
	"mini-messenger/internal/model"
	"mini-messenger/internal/service/auth"
)

type SessionRepository struct{ db *sql.DB }

func NewSessionRepository(db *sql.DB) *SessionRepository { return &SessionRepository{db: db} }

var _ auth.SessionRepository = (*SessionRepository)(nil)

func (r *SessionRepository) Create(ctx context.Context, s model.Session) error {
	if s.Revoked {
		return model.ErrInvalid
	}
	_, err := r.db.ExecContext(ctx, `
    INSERT INTO sessions(id,user_id,expires_at)
    VALUES ($1,$2,$3)
`, s.ID, s.UserID, s.ExpiresAt)
	return translate(err)
}
func (r *SessionRepository) Find(ctx context.Context, id string) (model.Session, error) {
	var s model.Session
	err := r.db.QueryRowContext(ctx, `
    SELECT id,user_id,expires_at,revoked_at IS NOT NULL
    FROM sessions
    WHERE id=$1
`, id).Scan(&s.ID, &s.UserID, &s.ExpiresAt, &s.Revoked)
	return s, translate(err)
}
func (r *SessionRepository) Revoke(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `
    UPDATE sessions
    SET revoked_at=COALESCE(revoked_at,now())
    WHERE id=$1
`, id)
	if err != nil {
		return translate(err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return translate(err)
	}
	if n == 0 {
		return model.ErrNotFound
	}
	return nil
}
