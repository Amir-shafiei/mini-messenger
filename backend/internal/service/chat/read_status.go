package chat

import (
	"context"
	"mini-messenger/internal/model"
)

type ReadRepository interface {
	MarkRead(context.Context, string, string, string) (model.ReadReceipt, error)
	ReadBy(context.Context, string, string) (model.ReadReceipt, error)
}
type ReadPublisher interface {
	PublishRead(context.Context, model.ReadReceipt) error
}
type ReadStatus struct {
	repository ReadRepository
	events     ReadPublisher
	reporter   ErrorReporter
}

func NewReadStatus(repository ReadRepository, events ReadPublisher, reporter ErrorReporter) *ReadStatus {
	return &ReadStatus{repository: repository, events: events, reporter: reporter}
}
func (s *ReadStatus) Mark(ctx context.Context, actor, peer, messageID string) (model.ReadReceipt, error) {
	if actor == "" {
		return model.ReadReceipt{}, model.ErrUnauthorized
	}
	if peer == "" || actor == peer || messageID == "" {
		return model.ReadReceipt{}, model.ErrInvalid
	}
	receipt, err := s.repository.MarkRead(ctx, actor, peer, messageID)
	if err != nil {
		return model.ReadReceipt{}, err
	}
	if err = s.events.PublishRead(ctx, receipt); err != nil {
		s.reporter.Report(ctx, err)
	}
	return receipt, nil
}

// Return what the peer has read from the authenticated actor, never arbitrary pairs.
func (s *ReadStatus) PeerRead(ctx context.Context, actor, peer string) (model.ReadReceipt, error) {
	if actor == "" {
		return model.ReadReceipt{}, model.ErrUnauthorized
	}
	if peer == "" || peer == actor {
		return model.ReadReceipt{}, model.ErrInvalid
	}
	return s.repository.ReadBy(ctx, peer, actor)
}
