package bootstrap

import (
	"context"
	"database/sql"
	"mini-messenger/internal/adapter/security"
)

// Demo accounts are opt-in. Restarting never replaces an existing password.
func seedDemo(ctx context.Context, db *sql.DB, passwords *security.Password, password string) error {
	hash, err := passwords.Hash(password)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, name := range []string{"alice", "bob", "charlie"} {
		id, err := (security.IDs{}).New()
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO users(id,username,password_hash) VALUES($1,$2,$3) ON CONFLICT(username) DO NOTHING`, id, name, hash); err != nil {
			return err
		}
	}
	return tx.Commit()
}
