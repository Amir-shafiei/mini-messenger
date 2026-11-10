package user

import (
	"context"
	"mini-messenger/internal/model"
)

type UserReader interface {
	FindByID(context.Context, string) (model.User, error)
}
type GetUser struct{ users UserReader }

func NewGetUser(users UserReader) *GetUser {
	return &GetUser{users: users}
}
func (uc *GetUser) Execute(ctx context.Context, actor string) (model.User, error) {
	if actor == "" {
		return model.User{}, model.ErrUnauthorized
	}
	return uc.users.FindByID(ctx, actor)
}
