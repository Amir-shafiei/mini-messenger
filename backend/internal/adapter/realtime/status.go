package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"mini-messenger/internal/model"
	"mini-messenger/internal/service/chat"
	"time"
)

var _ chat.ReadPublisher = (*Hub)(nil)

// Caller holds the hub lock. Only authenticated, active connections count as online.
func (h *Hub) broadcastPresenceLocked() {
	if h.closed {
		return
	}
	for {
		online := make(map[string]bool)
		for c, active := range h.clients {
			if active && time.Now().Before(c.expiresAt) {
				online[c.userID] = true
			}
		}
		ids := make([]string, 0, len(online))
		for id := range online {
			ids = append(ids, id)
		}
		data, _ := json.Marshal(struct {
			Type    string   `json:"type"`
			UserIDs []string `json:"user_ids"`
		}{Type: "presence.snapshot", UserIDs: ids})
		removed := false
		for c, active := range h.clients {
			if !active || !c.presence {
				continue
			}
			select {
			case c.send <- data:
			default:
				delete(h.clients, c)
				c.close()
				removed = true
			}
		}
		if !removed {
			return
		}
	}
}

type readEvent struct {
	Type    string   `json:"type"`
	Receipt readData `json:"receipt"`
}
type readData struct {
	ReaderID         string    `json:"reader_id"`
	PeerID           string    `json:"peer_id"`
	MessageID        string    `json:"message_id"`
	MessageCreatedAt time.Time `json:"message_created_at"`
}

func (h *Hub) PublishRead(_ context.Context, r model.ReadReceipt) error {
	data, err := json.Marshal(readEvent{
		Type: "message.read",
		Receipt: readData{
			ReaderID:         r.ReaderID,
			PeerID:           r.PeerID,
			MessageID:        r.MessageID,
			MessageCreatedAt: r.MessageCreatedAt,
		},
	})
	if err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return errors.New("realtime hub is closed")
	}
	var deliveryError error
	for c, active := range h.clients {
		if !active || (c.userID != r.ReaderID && c.userID != r.PeerID) {
			continue
		}
		if !time.Now().Before(c.expiresAt) {
			delete(h.clients, c)
			c.close()
			continue
		}
		select {
		case c.send <- data:
		default:
			delete(h.clients, c)
			c.close()
			deliveryError = errors.New("slow realtime client disconnected")
		}
	}
	return deliveryError
}
