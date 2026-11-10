package chat

import (
	"context"
	"errors"
	"mini-messenger/internal/model"
	"testing"
)

type reads struct {
	reader, peer string
	fail         bool
}

func (r *reads) MarkRead(_ context.Context, reader, peer, id string) (model.ReadReceipt, error) {
	if r.fail {
		return model.ReadReceipt{}, model.ErrInvalid
	}
	return model.ReadReceipt{ReaderID: reader, PeerID: peer, MessageID: id}, nil
}
func (r *reads) ReadBy(_ context.Context, reader, peer string) (model.ReadReceipt, error) {
	r.reader = reader
	r.peer = peer
	return model.ReadReceipt{}, nil
}

type readEvents struct{ called bool }

func (e *readEvents) PublishRead(context.Context, model.ReadReceipt) error {
	e.called = true
	return errors.New("offline")
}
func TestReadStatus(t *testing.T) {
	r := &reads{}
	e := &readEvents{}
	log := &reporter{}
	s := NewReadStatus(r, e, log)
	ctx := context.Background()
	if _, err := s.Mark(ctx, "", "peer", "id"); !errors.Is(err, model.ErrUnauthorized) {
		t.Fatal("anonymous read")
	}
	r.fail = true
	if _, err := s.Mark(ctx, "actor", "peer", "id"); err == nil || e.called {
		t.Fatal("published failed persistence")
	}
	r.fail = false
	receipt, err := s.Mark(ctx, "actor", "peer", "id")
	if err != nil || receipt.MessageID != "id" || !e.called || !log.called {
		t.Fatal("publication failure changed persisted result")
	}
	if _, err = s.PeerRead(ctx, "actor", "peer"); err != nil || r.reader != "peer" || r.peer != "actor" {
		t.Fatal("wrong receipt scope")
	}
}
