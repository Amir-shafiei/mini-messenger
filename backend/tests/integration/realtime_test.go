package integration

import (
	"bytes"
	"encoding/json"
	"github.com/gorilla/websocket"
	"io"
	"mini-messenger/internal/adapter/security"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// Opt-in live smoke test writes demo messages; run only against a local demo server.
func TestLiveRealtime(t *testing.T) {
	base := os.Getenv("LIVE_API_URL")
	password := os.Getenv("LIVE_DEMO_PASSWORD")
	if base == "" || password == "" {
		t.Skip("set LIVE_API_URL and LIVE_DEMO_PASSWORD for a demo server")
	}
	origin := base
	parsed, _ := url.Parse(base)
	request := func(client *http.Client, method, path string, body any, status int) []byte {
		t.Helper()
		var data []byte
		if body != nil {
			data, _ = json.Marshal(body)
		}
		req, err := http.NewRequest(method, base+path, bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", origin)
		req.Header.Set("Content-Type", "application/json")
		key, keyErr := (security.IDs{}).New()
		if keyErr != nil {
			t.Fatal(keyErr)
		}
		req.Header.Set("Idempotency-Key", key)
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		out, err := io.ReadAll(res.Body)
		if err != nil || res.StatusCode != status {
			t.Fatalf("%s %s status=%d", method, path, res.StatusCode)
		}
		return out
	}
	login := func(name string) *http.Client {
		jar, _ := cookiejar.New(nil)
		c := &http.Client{Jar: jar, Timeout: 5 * time.Second}
		request(c, "POST", "/api/v1/auth/login", map[string]string{"username": name, "password": password}, 200)
		return c
	}
	alice := login("alice")
	bob := login("bob")
	bobOther := login("bob")
	connect := func(c *http.Client) *websocket.Conn {
		t.Helper()
		headers := http.Header{"Origin": []string{origin}}
		for _, cookie := range c.Jar.Cookies(parsed) {
			headers.Add("Cookie", cookie.String())
		}
		conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(base, "http")+"/api/v1/ws", headers)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.Close() })
		return conn
	}
	sender := connect(alice)
	receiver := connect(bob)
	tab := connect(bob)
	otherSession := connect(bobOther)
	var users struct {
		Users []struct{ ID, Username string }
	}
	json.Unmarshal(request(alice, "GET", "/api/v1/users", nil, 200), &users)
	var bobID string
	for _, u := range users.Users {
		if u.Username == "bob" {
			bobID = u.ID
		}
	}
	if bobID == "" {
		t.Fatal("missing bob")
	}
	send := func() string {
		var out struct{ Message struct{ ID string } }
		json.Unmarshal(request(alice, "POST", "/api/v1/users/"+bobID+"/messages", map[string]string{"text": "realtime smoke test"}, 201), &out)
		return out.Message.ID
	}
	read := func(c *websocket.Conn, id string) {
		t.Helper()
		c.SetReadDeadline(time.Now().Add(3 * time.Second))
		var event struct {
			Type    string
			Message struct{ ID string }
		}
		for {
			if err := c.ReadJSON(&event); err != nil {
				t.Fatal(err)
			}
			if event.Type != "presence.snapshot" {
				break
			}
		}
		if event.Type != "message.created" || event.Message.ID != id {
			t.Fatal("realtime delivery failed")
		}
	}
	// Exercise the complete HTTP persistence -> WebSocket delivery path.
	id := send()
	for _, c := range []*websocket.Conn{sender, receiver, tab, otherSession} {
		read(c, id)
	}
	request(bob, "POST", "/api/v1/auth/logout", nil, 204)
	for _, c := range []*websocket.Conn{receiver, tab} {
		c.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, _, err := c.ReadMessage()
		for err == nil {
			_, _, err = c.ReadMessage()
		}
		if err == nil {
			t.Fatal("logout did not close socket")
		}
		if timeout, ok := err.(interface{ Timeout() bool }); ok && timeout.Timeout() {
			t.Fatal("socket only timed out")
		}
	}
	id = send()
	read(sender, id)
	read(otherSession, id)
	request(alice, "POST", "/api/v1/auth/logout", nil, 204)
	request(bobOther, "POST", "/api/v1/auth/logout", nil, 204)
}
