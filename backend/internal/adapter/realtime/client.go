package realtime

import (
	"bytes"
	"encoding/json"
	"github.com/gorilla/websocket"
	"io"
	"sync"
	"time"
)

const queueSize = 64
const pongWait = 60 * time.Second
const pingInterval = 25 * time.Second
const writeWait = 5 * time.Second

type client struct {
	typingPeer              string
	typingUntil, lastTyping time.Time
	presence                bool
	conn                    *websocket.Conn
	hub                     *Hub
	userID, sessionID       string
	expiresAt               time.Time
	send                    chan []byte
	done                    chan struct{}
	once                    sync.Once
}

func (c *client) close() {
	c.once.Do(func() {
		close(c.done)
		if c.conn != nil {
			c.conn.Close()
		}
	})
}
func (c *client) readLoop() {
	defer c.hub.remove(c)
	c.conn.SetReadLimit(1024)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { return c.conn.SetReadDeadline(time.Now().Add(pongWait)) })
	for {
		kind, data, err := c.conn.ReadMessage()
		if err != nil || kind != websocket.TextMessage {
			return
		}
		var event struct {
			Type      string `json:"type"`
			Recipient string `json:"recipient_id"`
			Active    bool   `json:"active"`
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&event) != nil || event.Type != "typing" {
			return
		}
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			return
		}
		c.hub.typing(c, event.Recipient, event.Active)
	}
}
func (c *client) writeLoop() {
	defer c.hub.remove(c)
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	expiry := time.NewTimer(time.Until(c.expiresAt))
	defer expiry.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-expiry.C:
			return
		case data := <-c.send:
			select {
			case <-c.done:
				return
			default:
			}
			if !time.Now().Before(c.expiresAt) {
				return
			}
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
