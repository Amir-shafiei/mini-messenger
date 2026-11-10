package user

import (
	"context"
	"errors"
	"mini-messenger/internal/model"
	"testing"
)

type users struct{ excluded string }

func (u *users) ListExcept(_ context.Context, id string) ([]model.User, error) {
	u.excluded = id
	return []model.User{{ID: "other"}}, nil
}
func TestListUsers(t *testing.T) {
	u := &users{}
	uc := NewListUsers(u)
	if _, err := uc.Execute(context.Background(), ""); !errors.Is(err, model.ErrUnauthorized) {
		t.Fatal("anonymous listing allowed")
	}
	out, err := uc.Execute(context.Background(), "actor")
	if err != nil || u.excluded != "actor" || len(out) != 1 {
		t.Fatal("wrong user scope")
	}
}
