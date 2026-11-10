package chat

import (
	"context"
	"mini-messenger/internal/model"
)

type ListConversations struct{ conversations ConversationRepository }

func NewListConversations(conversations ConversationRepository) *ListConversations {
	return &ListConversations{conversations: conversations}
}
func (service *ListConversations) Execute(ctx context.Context, actorID string) ([]model.Conversation, error) {
	if actorID == "" {
		return nil, model.ErrUnauthorized
	}
	return service.conversations.ListConversations(ctx, actorID)
}
