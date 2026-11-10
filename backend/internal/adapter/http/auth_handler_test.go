package httpadapter

import (
	"context"
	"errors"
	"mini-messenger/internal/adapter/security"
	"mini-messenger/internal/model"
	"mini-messenger/internal/service/auth"
	"mini-messenger/internal/service/chat"
	"mini-messenger/internal/service/user"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type store struct {
	hash   string
	s      model.Session
	closed bool
	fail   bool
}

func (s *store) FindCredentials(_ context.Context, name string) (model.Credentials, error) {
	if name != "alice" {
		return model.Credentials{}, model.ErrNotFound
	}
	return model.Credentials{User: model.User{ID: "u", Username: name}, PasswordHash: s.hash}, nil
}
func (s *store) FindByID(context.Context, string) (model.User, error) {
	return model.User{ID: "u", Username: "alice"}, nil
}
func (s *store) Create(_ context.Context, v model.Session) error { s.s = v; return nil }
func (s *store) Find(context.Context, string) (model.Session, error) {
	if s.fail {
		return model.Session{}, errors.New("private database error")
	}
	return s.s, nil
}
func (s *store) Revoke(context.Context, string) error { s.s.Revoked = true; return nil }
func (s *store) DisconnectSession(string)             { s.closed = true }

type clock struct{}

func (clock) Now() time.Time { return time.Now() }
func newAuthTestHandler(t *testing.T) (http.Handler, *store, string) {
	t.Helper()
	passwords, e := security.NewPassword()
	if e != nil {
		t.Fatal(e)
	}
	hash, e := passwords.Hash("correct-password")
	if e != nil {
		t.Fatal(e)
	}
	s := &store{hash: hash}
	tokens, e := security.NewJWT([]byte(strings.Repeat("k", 32)), "mini-messenger", "mini-messenger-web")
	if e != nil {
		t.Fatal(e)
	}
	h, e := NewRouter(
		Config{"https://example.test", true},
		auth.NewLogin(s, s, tokens, passwords, clock{}, security.IDs{}, 15*time.Minute),
		auth.NewAuthenticate(s, tokens, clock{}),
		auth.NewLogout(s, s),
		user.NewGetUser(s),
		user.NewListUsers(s),
		chat.NewSendMessage(s, s, s, s, clock{}, security.IDs{}),
		chat.NewListMessages(s, s),
		chat.NewReadStatus(s, s, s),
		chat.NewListConversations(s),
		auth.NewRefresh(s, security.RefreshSecrets{}, tokens, s, clock{}),
	)
	if e != nil {
		t.Fatal(e)
	}
	return h, s, hash
}

func authRequest(h http.Handler, method, path, body, origin, content string, cookie *http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Origin", origin)
	r.Header.Set("Content-Type", content)
	r.Header.Set("Idempotency-Key", "11111111-1111-4111-8111-111111111111")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestLoginInputValidation(t *testing.T) {
	h, _, _ := newAuthTestHandler(t)
	for _, tc := range []struct {
		name, body, origin, content string
		status                      int
	}{
		{"origin", `{}`, "https://evil.test", "application/json", 403},
		{"media", `{}`, "https://example.test", "text/plain", 415},
		{
			"unknown field",
			`{"username":"alice","password":"x","admin":true}`,
			"https://example.test",
			"application/json",
			400,
		},
		{"trailing", `{} {}`, "https://example.test", "application/json", 400},
		{
			"large",
			`{"username":"alice","password":"` + strings.Repeat("x", 33000) + `"}`,
			"https://example.test",
			"application/json",
			413,
		},
		{
			"wrong password",
			`{"username":"alice","password":"wrong"}`,
			"https://example.test",
			"application/json",
			401,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := authRequest(h, "POST", "/api/v1/auth/login", tc.body, tc.origin, tc.content, nil)
			if w.Code != tc.status {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestLoginMeAndLogout(t *testing.T) {
	h, s, hash := newAuthTestHandler(t)
	w := authRequest(
		h,
		"POST",
		"/api/v1/auth/login",
		`{"username":" ALICE ","password":"correct-password"}`,
		"https://example.test",
		"application/json",
		nil,
	)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatal("missing cookie")
	}
	refresh := cookies[1]
	if refresh.Name != "__Host-mm_refresh" || !refresh.HttpOnly || !refresh.Secure || refresh.Path != "/" || refresh.MaxAge < 6*24*60*60 || strings.Contains(w.Body.String(), refresh.Value) {
		t.Fatal("unsafe refresh cookie")
	}
	c := cookies[0]
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode || c.Path != "/" || c.MaxAge <= 0 {
		t.Fatal("unsafe cookie")
	}
	if strings.Contains(w.Body.String(), c.Value) || strings.Contains(w.Body.String(), hash) {
		t.Fatal("secret in JSON")
	}
	w = authRequest(h, "GET", "/api/v1/auth/me", "", "", "", c)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "alice") {
		t.Fatal("me", w.Code)
	}
	w = authRequest(h, "POST", "/api/v1/auth/logout", "", "https://example.test", "", c)
	if w.Code != 204 || !s.s.Revoked || !s.closed || w.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal("logout", w.Code)
	}
	w = authRequest(h, "GET", "/api/v1/auth/me", "", "", "", c)
	if w.Code != 401 {
		t.Fatal("revoked cookie accepted")
	}
	w = authRequest(h, "POST", "/api/v1/auth/logout", "", "https://example.test", "", c)
	if w.Code != 401 {
		t.Fatal("repeat logout")
	}
}

func TestAnonymousAndRouting(t *testing.T) {
	h, _, _ := newAuthTestHandler(t)
	w := authRequest(h, "GET", "/api/v1/auth/me", "", "", "", nil)
	if w.Code != 401 {
		t.Fatal("anonymous")
	}
	w = authRequest(h, "GET", "/api/v1/auth/login", "", "", "", nil)
	if w.Code != 405 || w.Header().Get("Allow") != "POST" {
		t.Fatal("method")
	}
	w = authRequest(h, "GET", "/missing", "", "", "", nil)
	if w.Code != 404 {
		t.Fatal("route")
	}
}
func TestRateLimit(t *testing.T) {
	l := newLoginLimiter()
	for i := 0; i < 10; i++ {
		if !l.allow("127.0.0.1:1234") {
			t.Fatal("early limit")
		}
	}
	if l.allow("127.0.0.1:9999") {
		t.Fatal("port bypass")
	}
	if !l.allow("127.0.0.2:1234") {
		t.Fatal("shared IP bucket")
	}
}

func TestInternalErrorsDoNotLeak(t *testing.T) {
	h, s, _ := newAuthTestHandler(t)
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
	c := login.Result().Cookies()[0]
	s.fail = true
	w := authRequest(h, "GET", "/api/v1/auth/me", "", "", "", c)
	if w.Code != 500 || strings.Contains(w.Body.String(), "private") {
		t.Fatal("error leakage")
	}
}

func (s *store) ListExcept(context.Context, string) ([]model.User, error)       { return []model.User{}, nil }
func (s *store) Save(_ context.Context, m model.Message) (model.Message, error) { return m, nil }
func (s *store) History(context.Context, string, string, chat.Page) ([]model.Message, error) {
	return []model.Message{}, nil
}
func (s *store) PublishMessage(context.Context, model.Message) error { return nil }
func (s *store) Report(context.Context, error)                       {}

func (s *store) MarkRead(context.Context, string, string, string) (model.ReadReceipt, error) {
	return model.ReadReceipt{}, nil
}
func (s *store) ReadBy(context.Context, string, string) (model.ReadReceipt, error) {
	return model.ReadReceipt{}, nil
}
func (s *store) PublishRead(context.Context, model.ReadReceipt) error { return nil }

func (s *store) ListConversations(context.Context, string) ([]model.Conversation, error) {
	return []model.Conversation{}, nil
}

func (s *store) StoreRefresh(context.Context, string, string) error { return nil }
func (s *store) RotateRefresh(context.Context, string, string, time.Time) (model.Session, error) {
	return s.s, nil
}
