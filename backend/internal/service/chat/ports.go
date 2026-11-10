package chat

import (
	"context"
	"mini-messenger/internal/model"
	"time"
)

type UserRepository interface {
	FindByID(context.Context, string) (model.User, error)
}
type Page struct {
	BeforeID string
	Limit    int
}
type MessageRepository interface {
	Save(context.Context, model.Message) (model.Message, error)
	// History must filter both participant IDs, order newest first and apply the cursor.
	History(context.Context, string, string, Page) ([]model.Message, error)
}
type MessagePublisher interface {
	PublishMessage(context.Context, model.Message) error
}
type ErrorReporter interface{ Report(context.Context, error) }
type Clock interface{ Now() time.Time }
type IDGenerator interface{ New() (string, error) }

type ConversationRepository interface {
	ListConversations(context.Context, string) ([]model.Conversation, error)
}
