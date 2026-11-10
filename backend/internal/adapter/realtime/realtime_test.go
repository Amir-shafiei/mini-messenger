package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gorilla/websocket"
	"mini-messenger/internal/model"
	"mini-messenger/internal/service/auth"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func testClient(user, session string) *client {
	return &client{
		userID:    user,
		sessionID: session,
		expiresAt: time.Now().Add(time.Hour),
		send:      make(chan []byte, 1),
		done:      make(chan struct{}),
	}
}
func TestHubDeliveryAndIsolation(t *testing.T) {
	h := NewHub()
	defer h.Close()
	alice := testClient("a", "a1")
	bob := testClient("b", "b1")
	bobTab := testClient("b", "b1")
	other := testClient("c", "c1")
	pending := testClient("b", "b2")
	for _, c := range []*client{alice, bob, bobTab, other, pending} {
		h.register(c)
		if c != pending {
			h.activate(c)
		}
	}
	m := model.Message{ID: "id", SenderID: "a", RecipientID: "b", Text: "hello"}
	if err := h.PublishMessage(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	for _, c := range []*client{alice, bob, bobTab} {
		select {
		case data := <-c.send:
			var event messageEvent
			if json.Unmarshal(data, &event) != nil || event.Type != "message.created" || event.Message.ID != "id" {
				t.Fatal("bad event")
			}
		default:
			t.Fatal("missing event")
		}
	}
	if len(other.send) != 0 || len(pending.send) != 0 {
		t.Fatal("unauthorized delivery")
	}
	h.DisconnectSession("b1")
	for _, c := range []*client{bob, bobTab} {
		select {
		case <-c.done:
		default:
			t.Fatal("session still open")
		}
	}
	select {
	case <-pending.done:
		t.Fatal("other session closed")
	default:
	}
}
func TestSlowClientAndShutdown(t *testing.T) {
	h := NewHub()
	slow := testClient("b", "s")
	fast := testClient("a", "f")
	h.register(slow)
	h.activate(slow)
	h.register(fast)
	h.activate(fast)
	slow.send <- []byte("full")
	if h.PublishMessage(context.Background(), model.Message{SenderID: "a", RecipientID: "b"}) == nil {
		t.Fatal("overflow not reported")
	}
	select {
	case <-slow.done:
	default:
		t.Fatal("slow client remained")
	}
	if len(fast.send) != 1 {
		t.Fatal("slow client blocked fast")
	}
	h.Close()
	if h.register(testClient("c", "s")) {
		t.Fatal("registration after shutdown")
	}
}

type clock struct{}

func (clock) Now() time.Time { return time.Now() }

type tokens struct{ expiry time.Time }

func (t tokens) Issue(auth.Claims) (string, error) { return "", nil }
func (t tokens) Verify(raw string) (auth.Claims, error) {
	if raw != "valid" {
		return auth.Claims{}, model.ErrUnauthorized
	}
	return auth.Claims{UserID: "u", SessionID: "s", ExpiresAt: t.expiry}, nil
}

type sessions struct {
	mu             sync.Mutex
	calls          int
	revokeOnSecond bool
	expiry         time.Time
}

func (s *sessions) Create(context.Context, model.Session) error { return nil }
func (s *sessions) Revoke(context.Context, string) error        { return nil }
func (s *sessions) Find(context.Context, string) (model.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	return model.Session{ID: "s", UserID: "u", ExpiresAt: s.expiry, Revoked: s.revokeOnSecond && s.calls >= 2}, nil
}
func setup(t *testing.T, ttl time.Duration, revoke bool) (*Hub, string) {
	t.Helper()
	h := NewHub()
	expiry := time.Now().Add(ttl)
	s := &sessions{expiry: expiry, revokeOnSecond: revoke}
	server := httptest.NewServer(NewHandler(h, auth.NewAuthenticate(s, tokens{expiry}, clock{}), "https://example.test"))
	t.Cleanup(func() { h.Close(); server.Close() })
	return h, "ws" + strings.TrimPrefix(server.URL, "http")
}
func dial(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	headers := http.Header{
		"Origin": []string{"https://example.test"},
		"Cookie": []string{"__Host-mm_session=valid"},
	}
	conn, _, err := websocket.DefaultDialer.Dial(url, headers)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}
func TestHandshakeProtection(t *testing.T) {
	_, url := setup(t, time.Minute, false)
	for _, tc := range []struct {
		origin, cookie string
		status         int
	}{
		{"https://evil.test", "__Host-mm_session=valid", 403},
		{"https://example.test", "", 401},
		{"https://example.test", "__Host-mm_session=bad", 401},
	} {
		conn, res, err := websocket.DefaultDialer.Dial(url, http.Header{"Origin": []string{tc.origin}, "Cookie": []string{tc.cookie}})
		if conn != nil {
			conn.Close()
		}
		if err == nil || res == nil || res.StatusCode != tc.status {
			t.Fatal("invalid handshake accepted", err)
		}
		res.Body.Close()
	}
}
func TestConnectionExpiry(t *testing.T) {
	_, url := setup(t, 200*time.Millisecond, false)
	conn := dial(t, url)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err := conn.ReadMessage()
	for err == nil {
		_, _, err = conn.ReadMessage()
	}
	if err == nil {
		t.Fatal("connection survived expiry")
	}
	var timeout interface{ Timeout() bool }
	if errors.As(err, &timeout) && timeout.Timeout() {
		t.Fatal("expiry did not close connection")
	}
}
func TestLogoutDuringHandshake(t *testing.T) {
	_, url := setup(t, time.Minute, true)
	conn := dial(t, url)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err := conn.ReadMessage()
	for err == nil {
		_, _, err = conn.ReadMessage()
	}
	if err == nil {
		t.Fatal("revoked handshake survived")
	}
	var timeout interface{ Timeout() bool }
	if errors.As(err, &timeout) && timeout.Timeout() {
		t.Fatal("second check failed to close")
	}
}
func TestConcurrentPublishAndDisconnect(t *testing.T) {
	h := NewHub()
	defer h.Close()
	var group sync.WaitGroup
	for i := 0; i < 20; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			c := testClient("a", "s")
			h.register(c)
			h.activate(c)
			_ = h.PublishMessage(context.Background(), model.Message{SenderID: "a", RecipientID: "b"})
			h.DisconnectSession("s")
			h.remove(c)
		}()
	}
	group.Wait()
}

func TestPresenceMultipleTabs(t *testing.T) {
	h := NewHub()
	defer h.Close()
	watcher := testClient("watcher", "w")
	watcher.presence = true
	watcher.send = make(chan []byte, 16)
	one := testClient("peer", "p1")
	two := testClient("peer", "p2")
	h.register(watcher)
	h.activate(watcher)
	h.register(one)
	h.activate(one)
	h.register(two)
	h.activate(two)
	drain := func() bool {
		t.Helper()
		var last struct {
			Type    string
			UserIDs []string `json:"user_ids"`
		}
		for len(watcher.send) > 0 {
			if err := json.Unmarshal(<-watcher.send, &last); err != nil {
				t.Fatal(err)
			}
		}
		for _, id := range last.UserIDs {
			if id == "peer" {
				return true
			}
		}
		return false
	}
	if !drain() {
		t.Fatal("peer should be online")
	}
	h.DisconnectSession("p1")
	if !drain() {
		t.Fatal("second session must preserve presence")
	}
	h.remove(two)
	if drain() {
		t.Fatal("last connection gone but still online")
	}
}
