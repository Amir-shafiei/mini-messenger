package httpadapter

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChatHTTP(t *testing.T) {
	h, _, _ := newAuthTestHandler(t)
	login := authRequest(
		h,
		"POST",
		"/api/v1/auth/login",
		`{"username":"alice","password":"correct-password"}`,
		"https://example.test",
		"application/json",
		nil,
	)
	if login.Code != 200 {
		t.Fatal(login.Code)
	}
	cookie := login.Result().Cookies()[0]
	path := "/api/v1/users/00000000-0000-4000-8000-000000000001/messages"
	// Older tabs omitted the key: report the protocol problem, not bad message text.
	for _, key := range []string{"", "not-a-uuid"} {
		req := httptest.NewRequest("POST", path, strings.NewReader(`{"text":"valid text"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://example.test")
		req.Header.Set("Idempotency-Key", key)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 400 || !strings.Contains(w.Body.String(), "invalid_idempotency_key") {
			t.Fatal("missing protocol error", w.Code, w.Body.String())
		}
	}
	for _, tc := range []struct {
		name, method, path, body, origin string
		status                           int
	}{
		{"send", "POST", path, `{"text":"hello"}`, "https://example.test", 201},
		{"empty text", "POST", path, `{"text":" "}`, "https://example.test", 400},
		{"nul", "POST", path, `{"text":"a\u0000b"}`, "https://example.test", 400},
		{
			"forged sender",
			"POST",
			path,
			`{"text":"x","sender_id":"other"}`,
			"https://example.test",
			400,
		},
		{"origin", "POST", path, `{"text":"x"}`, "https://evil.test", 403},
		{"invalid UUID", "GET", "/api/v1/users/bad/messages", "", "", 400},
		{"limit zero", "GET", path + "?limit=0", "", "", 400},
		{"limit too high", "GET", path + "?limit=101", "", "", 400},
		{"bad cursor", "GET", path + "?before=bad", "", "", 400},
		{"wrong method", "DELETE", path, "", "", 405},
		{"users", "GET", "/api/v1/users", "", "", 200},
		{"history", "GET", path, "", "", 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := authRequest(h, tc.method, tc.path, tc.body, tc.origin, "application/json", cookie)
			if w.Code != tc.status {
				t.Fatalf("got %d: %s", w.Code, w.Body.String())
			}
			if tc.name == "send" {
				var out struct {
					Message messageJSON `json:"message"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || out.Message.SenderID != "u" {
					t.Fatal("identity not taken from authentication")
				}
			}
			if tc.name == "history" && !strings.Contains(w.Body.String(), `"messages":[]`) {
				t.Fatal("empty history must be array")
			}
		})
	}
	for _, path := range []string{path, "/api/v1/users"} {
		w := authRequest(h, "GET", path, "", "", "", nil)
		if w.Code != http.StatusUnauthorized {
			t.Fatal("anonymous access")
		}
	}
}
