package user

import (
	"context"
	"mini-messenger/internal/model"
)

type UserRepository interface {
	ListExcept(context.Context, string) ([]model.User, error)
}
