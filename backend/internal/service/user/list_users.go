package user

import (
	"context"
	"mini-messenger/internal/model"
)

type ListUsers struct{ users UserRepository }

func NewListUsers(users UserRepository) *ListUsers {
	return &ListUsers{users: users}
}
func (uc *ListUsers) Execute(ctx context.Context, actorID string) ([]model.User, error) {
	if actorID == "" {
		return nil, model.ErrUnauthorized
	}
	return uc.users.ListExcept(ctx, actorID)
}
