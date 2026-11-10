package postgres

import (
	"context"
	"database/sql"
	"mini-messenger/internal/model"
	"mini-messenger/internal/service/auth"
	"mini-messenger/internal/service/chat"
	"mini-messenger/internal/service/user"
)

type UserRepository struct{ db *sql.DB }

func NewUserRepository(db *sql.DB) *UserRepository { return &UserRepository{db: db} }

var _ auth.UserRepository = (*UserRepository)(nil)
var _ chat.UserRepository = (*UserRepository)(nil)
var _ user.UserRepository = (*UserRepository)(nil)

func (r *UserRepository) FindCredentials(ctx context.Context, name string) (model.Credentials, error) {
	var c model.Credentials
	err := r.db.QueryRowContext(ctx, `
    SELECT id,username,password_hash
    FROM users
    WHERE username=$1
`, name).Scan(&c.User.ID, &c.User.Username, &c.PasswordHash)
	return c, translate(err)
}
func (r *UserRepository) FindByID(ctx context.Context, id string) (model.User, error) {
	var u model.User
	err := r.db.QueryRowContext(ctx, `
    SELECT id,username
    FROM users
    WHERE id=$1
`, id).Scan(&u.ID, &u.Username)
	return u, translate(err)
}
func (r *UserRepository) ListExcept(ctx context.Context, id string) ([]model.User, error) {
	rows, err := r.db.QueryContext(ctx, `
    SELECT id,username
    FROM users
    WHERE id<>$1
    ORDER BY username
`, id)
	if err != nil {
		return nil, translate(err)
	}
	defer rows.Close()
	out := make([]model.User, 0)
	for rows.Next() {
		var u model.User
		if err = rows.Scan(&u.ID, &u.Username); err != nil {
			return nil, translate(err)
		}
		out = append(out, u)
	}
	return out, translate(rows.Err())
}
