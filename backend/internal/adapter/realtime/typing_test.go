package realtime

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTypingPrivateAndAuthenticated(t *testing.T) {
	h := NewHub()
	defer h.Close()
	a := "00000000-0000-4000-8000-000000000001"
	b := "00000000-0000-4000-8000-000000000002"
	makeClient := func(id string) *client {
		c := &client{
			userID:    id,
			expiresAt: time.Now().Add(time.Minute),
			send:      make(chan []byte, 64),
			done:      make(chan struct{}),
		}
		h.register(c)
		h.activate(c)
		return c
	}
	sender := makeClient(a)
	recipient := makeClient(b)
	third := makeClient("third")
	h.typing(sender, b, true)
	select {
	case data := <-recipient.send:
		var event struct {
			Sender string `json:"sender_id"`
			Active bool   `json:"active"`
		}
		if json.Unmarshal(data, &event) != nil || event.Sender != a || !event.Active {
			t.Fatal("wrong sender")
		}
	default:
		t.Fatal("missing typing")
	}
	if len(third.send) != 0 || len(sender.send) != 0 {
		t.Fatal("typing leaked")
	}
	h.typing(sender, b, true)
	if len(recipient.send) != 0 {
		t.Fatal("throttle bypassed")
	}
	h.DisconnectSession(sender.sessionID)
	h.typing(sender, b, true)
	if len(recipient.send) != 0 {
		t.Fatal("revoked socket published")
	}
}
