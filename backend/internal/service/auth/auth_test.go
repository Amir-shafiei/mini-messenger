package auth

import (
	"context"
	"errors"
	"mini-messenger/internal/model"
	"testing"
	"time"
)

type clock struct{ now time.Time }

func (c clock) Now() time.Time { return c.now }

type tokens struct{ claims Claims }

func (t tokens) Verify(string) (Claims, error) { return t.claims, nil }
func (t tokens) Issue(Claims) (string, error)  { return "token", nil }

type sessions struct {
	value model.Session
	fail  bool
}

func (s *sessions) Create(_ context.Context, v model.Session) error     { s.value = v; return nil }
func (s *sessions) Find(context.Context, string) (model.Session, error) { return s.value, nil }
func (s *sessions) Revoke(context.Context, string) error {
	if s.fail {
		return errors.New("storage failure")
	}
	s.value.Revoked = true
	return nil
}

type connections struct{ closed bool }

func (c *connections) DisconnectSession(string) { c.closed = true }

type users struct{ missing bool }

func (u users) FindCredentials(context.Context, string) (model.Credentials, error) {
	if u.missing {
		return model.Credentials{}, model.ErrNotFound
	}
	return model.Credentials{User: model.User{ID: "u"}, PasswordHash: "hash"}, nil
}

type passwords struct{}

func (passwords) Compare(_, p string) bool { return p == "correct" }

type ids struct{}

func (ids) New() (string, error) { return "s", nil }
func TestAuthentication(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	exp := now.Add(time.Hour)
	for _, tc := range []struct {
		name string
		s    model.Session
		c    Claims
		ok   bool
	}{
		{
			"valid",
			model.Session{ID: "s", UserID: "u", ExpiresAt: exp},
			Claims{"u", "s", exp},
			true,
		},
		{
			"revoked",
			model.Session{ID: "s", UserID: "u", ExpiresAt: exp, Revoked: true},
			Claims{"u", "s", exp},
			false,
		},
		{
			"wrong owner",
			model.Session{ID: "s", UserID: "other", ExpiresAt: exp},
			Claims{"u", "s", exp},
			false,
		},
		{
			"expired session",
			model.Session{ID: "s", UserID: "u", ExpiresAt: now},
			Claims{"u", "s", exp},
			false,
		},
		{
			"expired token",
			model.Session{ID: "s", UserID: "u", ExpiresAt: exp},
			Claims{"u", "s", now},
			false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewAuthenticate(&sessions{value: tc.s}, tokens{tc.c}, clock{now}).Execute(ctx, "token")
			if (err == nil) != tc.ok {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestLogoutRevokesBeforeDisconnecting(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	exp := now.Add(time.Hour)
	s := &sessions{value: model.Session{ID: "s", UserID: "u", ExpiresAt: exp}}
	c := &connections{}
	logout := NewLogout(s, c)
	s.fail = true
	if logout.Execute(ctx, Principal{UserID: "u", SessionID: "s"}) == nil || c.closed {
		t.Fatal("disconnect before durable revocation")
	}
	s.fail = false
	if err := logout.Execute(ctx, Principal{UserID: "u", SessionID: "s"}); err != nil || !c.closed {
		t.Fatal("logout failed")
	}
	if _, err := NewAuthenticate(s, tokens{Claims{"u", "s", exp}}, clock{now}).Execute(ctx, "token"); !errors.Is(err, model.ErrUnauthorized) {
		t.Fatal("revoked token accepted")
	}
}
func TestLogin(t *testing.T) {
	for _, tc := range []struct {
		missing  bool
		password string
		ok       bool
	}{{false, "correct", true}, {false, "wrong", false}, {true, "correct", false}} {
		s := &sessions{}
		out, err := NewLogin(users{tc.missing}, s, tokens{}, passwords{}, clock{time.Now()}, ids{}, time.Hour).Execute(context.Background(), LoginInput{"alice", tc.password})
		if (err == nil) != tc.ok {
			t.Fatal("wrong login result")
		}
		if tc.ok && (out.Token == "" || s.value.UserID != "u") {
			t.Fatal("session not created")
		}
	}
}
