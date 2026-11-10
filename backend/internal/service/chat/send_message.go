package chat

import (
	"context"
	"mini-messenger/internal/model"
)

type SendMessage struct {
	users    UserRepository
	messages MessageRepository
	events   MessagePublisher
	reporter ErrorReporter
	clock    Clock
	ids      IDGenerator
}

func NewSendMessage(
	users UserRepository,
	messages MessageRepository,
	events MessagePublisher,
	reporter ErrorReporter,
	clock Clock,
	ids IDGenerator,
) *SendMessage {
	return &SendMessage{
		users:    users,
		messages: messages,
		events:   events,
		reporter: reporter,
		clock:    clock,
		ids:      ids,
	}
}

type SendInput struct{ ActorID, RecipientID, Text, IdempotencyKey string }

func (uc *SendMessage) Execute(ctx context.Context, in SendInput) (model.Message, error) {
	if in.ActorID == "" {
		return model.Message{}, model.ErrUnauthorized
	}
	if in.IdempotencyKey == "" {
		return model.Message{}, model.ErrInvalid
	}
	if err := model.ValidateMessage(in.ActorID, in.RecipientID, in.Text); err != nil {
		return model.Message{}, err
	}
	if _, err := uc.users.FindByID(ctx, in.RecipientID); err != nil {
		return model.Message{}, err
	}
	id, err := uc.ids.New()
	if err != nil {
		return model.Message{}, err
	}
	m, err := uc.messages.Save(ctx, model.Message{
		ID:             id,
		IdempotencyKey: in.IdempotencyKey,
		SenderID:       in.ActorID,
		RecipientID:    in.RecipientID,
		Text:           in.Text,
		CreatedAt:      uc.clock.Now(),
	})
	if err != nil {
		return model.Message{}, err
	}
	// The message is already durable: delivery failure must not invite a duplicate retry.
	if err = uc.events.PublishMessage(ctx, m); err != nil {
		uc.reporter.Report(ctx, err)
	}
	return m, nil
}
