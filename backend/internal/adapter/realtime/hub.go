package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"mini-messenger/internal/model"
	"mini-messenger/internal/service/auth"
	"mini-messenger/internal/service/chat"
	"sync"
	"time"
)

type Hub struct {
	mu      sync.Mutex
	clients map[*client]bool
	closed  bool
}

func NewHub() *Hub { return &Hub{clients: make(map[*client]bool)} }

var _ auth.SessionConnections = (*Hub)(nil)
var _ chat.MessagePublisher = (*Hub)(nil)

func (h *Hub) register(c *client) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return false
	}
	h.clients[c] = false
	return true
}

// Pending connections receive nothing until the second session check succeeds.
func (h *Hub) activate(c *client) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, exists := h.clients[c]; !exists {
		return false
	}
	h.clients[c] = true
	h.broadcastPresenceLocked()
	return true
}
func (h *Hub) remove(c *client) {
	h.mu.Lock()
	delete(h.clients, c)
	h.broadcastPresenceLocked()
	h.mu.Unlock()
	c.close()
}
func (h *Hub) DisconnectSession(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		if c.sessionID == id {
			delete(h.clients, c)
			c.close()
		}
	}
	h.broadcastPresenceLocked()
}
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for c := range h.clients {
		delete(h.clients, c)
		c.close()
	}
}

type messageEvent struct {
	Type    string       `json:"type"`
	Message eventMessage `json:"message"`
}
type eventMessage struct {
	ID          string    `json:"id"`
	SenderID    string    `json:"sender_id"`
	RecipientID string    `json:"recipient_id"`
	Text        string    `json:"text"`
	CreatedAt   time.Time `json:"created_at"`
}

func (h *Hub) PublishMessage(_ context.Context, m model.Message) error {
	data, err := json.Marshal(messageEvent{
		Type: "message.created",
		Message: eventMessage{
			ID:          m.ID,
			SenderID:    m.SenderID,
			RecipientID: m.RecipientID,
			Text:        m.Text,
			CreatedAt:   m.CreatedAt,
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
		if !active || (c.userID != m.SenderID && c.userID != m.RecipientID) {
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
