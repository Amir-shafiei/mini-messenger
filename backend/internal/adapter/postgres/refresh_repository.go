package postgres

import (
	"context"
	"database/sql"
	"errors"
	"mini-messenger/internal/model"
	"time"
)

func (r *SessionRepository) StoreRefresh(ctx context.Context, id, hash string) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO refresh_tokens(token_hash,session_id) VALUES($1,$2)`, hash, id)
	return translate(err)
}
func (r *SessionRepository) RotateRefresh(ctx context.Context, oldHash, newHash string, now time.Time) (model.Session, error) {
	var s model.Session
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return s, translate(err)
	}
	defer tx.Rollback()
	// Lock the parent session first: rotations, replay revocation and logout serialize.
	err = tx.QueryRowContext(ctx, `SELECT s.id,s.user_id,s.expires_at,s.revoked_at IS NOT NULL
 FROM sessions s JOIN refresh_tokens t ON t.session_id=s.id WHERE t.token_hash=$1 FOR UPDATE OF s`, oldHash).Scan(&s.ID, &s.UserID, &s.ExpiresAt, &s.Revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return s, model.ErrUnauthorized
	}
	if err != nil {
		return s, translate(err)
	}
	if !s.Active(now) {
		return s, model.ErrUnauthorized
	}
	var used sql.NullTime
	if err = tx.QueryRowContext(ctx, `SELECT consumed_at FROM refresh_tokens WHERE token_hash=$1`, oldHash).Scan(&used); err != nil {
		return s, translate(err)
	}
	if used.Valid {
		if _, err = tx.ExecContext(ctx, `UPDATE sessions SET revoked_at=$2 WHERE id=$1`, s.ID, now); err != nil {
			return s, translate(err)
		}
		if err = tx.Commit(); err != nil {
			return s, translate(err)
		}
		return s, model.ErrUnauthorized
	}
	if _, err = tx.ExecContext(ctx, `UPDATE refresh_tokens SET consumed_at=$2 WHERE token_hash=$1`, oldHash, now); err != nil {
		return s, translate(err)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO refresh_tokens(token_hash,session_id) VALUES($1,$2)`, newHash, s.ID); err != nil {
		return s, translate(err)
	}
	if err = tx.Commit(); err != nil {
		return s, translate(err)
	}
	return s, nil
}
