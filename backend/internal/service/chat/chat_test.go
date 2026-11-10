package chat

import (
	"context"
	"errors"
	"mini-messenger/internal/model"
	"testing"
	"time"
)

type users struct{}

func (users) FindByID(_ context.Context, id string) (model.User, error) {
	return model.User{ID: id}, nil
}

type clock struct{}

func (clock) Now() time.Time { return time.Unix(100, 0) }

type ids struct{}

func (ids) New() (string, error) { return "id", nil }

type store struct {
	fail        bool
	saved       bool
	actor, peer string
}

func (s *store) Save(_ context.Context, m model.Message) (model.Message, error) {
	if s.fail {
		return model.Message{}, errors.New("db failed")
	}
	s.saved = true
	return m, nil
}
func (s *store) History(_ context.Context, a, b string, p Page) ([]model.Message, error) {
	s.actor = a
	s.peer = b
	return nil, nil
}

type events struct {
	store  *store
	called bool
	fail   bool
}

func (e *events) PublishMessage(context.Context, model.Message) error {
	if !e.store.saved {
		panic("published before save")
	}
	e.called = true
	if e.fail {
		return errors.New("offline")
	}
	return nil
}

type reporter struct{ called bool }

func (r *reporter) Report(context.Context, error) { r.called = true }
func TestPersistenceBeforePublication(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		saveFail, publishFail bool
	}{
		{"saved_then_published", false, false},
		{"save_failure_blocks_publication", true, false},
		{"publication_failure_preserves_success", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &store{fail: tc.saveFail}
			e := &events{store: s, fail: tc.publishFail}
			r := &reporter{}
			m, err := NewSendMessage(users{}, s, e, r, clock{}, ids{}).Execute(context.Background(), SendInput{"a", "b", "hello", "key"})
			if (err != nil) != tc.saveFail {
				t.Fatal("wrong outcome")
			}
			if e.called == tc.saveFail {
				t.Fatal("wrong publication")
			}
			if !tc.saveFail && m.ID != "id" {
				t.Fatal("lost persisted identity")
			}
			if r.called != tc.publishFail {
				t.Fatal("publication error not reported")
			}
		})
	}
}
func TestHistoryBoundaries(t *testing.T) {
	s := &store{}
	uc := NewListMessages(users{}, s)
	_, err := uc.Execute(context.Background(), ListInput{ActorID: "a", PeerID: "b"})
	if err != nil || s.actor != "a" || s.peer != "b" {
		t.Fatal("participant scope missing")
	}
	_, err = uc.Execute(context.Background(), ListInput{PeerID: "b"})
	if !errors.Is(err, model.ErrUnauthorized) {
		t.Fatal("anonymous history allowed")
	}
	_, err = uc.Execute(context.Background(), ListInput{ActorID: "a", PeerID: "b", Page: Page{Limit: 101}})
	if !errors.Is(err, model.ErrInvalid) {
		t.Fatal("unbounded page")
	}
}
