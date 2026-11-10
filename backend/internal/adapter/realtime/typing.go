package realtime

import (
	"encoding/json"
	"time"
)

func validPeer(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, c := range id {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func (h *Hub) typing(c *client, peer string, active bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.clients[c] || !time.Now().Before(c.expiresAt) || peer == c.userID || !validPeer(peer) {
		return
	}
	// Bound traffic even if a client ignores the browser's throttle.
	now := time.Now()
	if now.Sub(c.lastTyping) < 200*time.Millisecond {
		return
	}
	c.lastTyping = now
	c.typingPeer = peer
	if active {
		c.typingUntil = now.Add(4 * time.Second)
	} else {
		c.typingUntil = time.Time{}
	}
	// Another tab of this sender may still be typing to the same recipient.
	typing := false
	for other, enabled := range h.clients {
		if enabled && other.userID == c.userID && other.typingPeer == peer && now.Before(other.typingUntil) && now.Before(other.expiresAt) {
			typing = true
		}
	}
	data, _ := json.Marshal(struct {
		Type   string `json:"type"`
		Sender string `json:"sender_id"`
		Active bool   `json:"active"`
	}{"typing", c.userID, typing})
	for target, enabled := range h.clients {
		if !enabled || target.userID != peer || !now.Before(target.expiresAt) {
			continue
		}
		select {
		case target.send <- data:
		default:
		}
	}
}
