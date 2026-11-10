package chat

import (
	"context"
	"mini-messenger/internal/model"
)

type ListMessages struct {
	users    UserRepository
	messages MessageRepository
}

func NewListMessages(users UserRepository, messages MessageRepository) *ListMessages {
	return &ListMessages{users: users, messages: messages}
}

type ListInput struct {
	ActorID, PeerID string
	Page            Page
}

func (uc *ListMessages) Execute(ctx context.Context, in ListInput) ([]model.Message, error) {
	if in.ActorID == "" {
		return nil, model.ErrUnauthorized
	}
	if in.PeerID == "" || in.ActorID == in.PeerID {
		return nil, model.ErrInvalid
	}
	if in.Page.Limit == 0 {
		in.Page.Limit = 50
	}
	if in.Page.Limit < 1 || in.Page.Limit > 100 {
		return nil, model.ErrInvalid
	}
	if _, err := uc.users.FindByID(ctx, in.PeerID); err != nil {
		return nil, err
	}
	return uc.messages.History(ctx, in.ActorID, in.PeerID, in.Page)
}
