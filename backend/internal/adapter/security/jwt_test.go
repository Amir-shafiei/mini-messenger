package security

import (
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"mini-messenger/internal/model"
	"mini-messenger/internal/service/auth"
	"strings"
	"testing"
	"time"
)

func TestJWT(t *testing.T) {
	now := time.Unix(1800000000, 0)
	key := []byte(strings.Repeat("k", 32))
	j, err := NewJWT(key, "mini-messenger", "mini-messenger-web")
	if err != nil {
		t.Fatal(err)
	}
	j.now = func() time.Time { return now }
	raw, err := j.Issue(auth.Claims{UserID: "user", SessionID: "session", ExpiresAt: now.Add(15 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	got, err := j.Verify(raw)
	if err != nil || got.UserID != "user" || got.SessionID != "session" {
		t.Fatal("roundtrip", err)
	}
	if _, err = NewJWT([]byte("weak"), "i", "a"); err == nil {
		t.Fatal("weak key accepted")
	}
	for _, name := range []string{
		"expired",
		"missing exp",
		"missing iat",
		"missing sid",
		"missing sub",
		"issuer",
		"audience",
		"future iat",
		"future nbf",
		"long lifetime",
		"algorithm",
		"none",
		"wrong signature",
	} {
		t.Run(name, func(t *testing.T) {
			c := jwt.MapClaims{
				"sub": "user",
				"sid": "session",
				"iss": "mini-messenger",
				"aud": "mini-messenger-web",
				"iat": now.Unix(),
				"exp": now.Add(time.Minute).Unix(),
			}
			method := jwt.SigningMethod(jwt.SigningMethodHS256)
			var signingKey any = key
			switch name {
			case "expired":
				c["exp"] = now.Unix()
			case "missing exp":
				delete(c, "exp")
			case "missing iat":
				delete(c, "iat")
			case "missing sid":
				delete(c, "sid")
			case "missing sub":
				delete(c, "sub")
			case "issuer":
				c["iss"] = "other"
			case "audience":
				c["aud"] = "other"
			case "future iat":
				c["iat"] = now.Add(time.Hour).Unix()
			case "future nbf":
				c["nbf"] = now.Add(time.Hour).Unix()
			case "long lifetime":
				c["exp"] = now.Add(time.Hour).Unix()
			case "algorithm":
				method = jwt.SigningMethodHS384
			case "none":
				method = jwt.SigningMethodNone
				signingKey = jwt.UnsafeAllowNoneSignatureType
			case "wrong signature":
				signingKey = []byte(strings.Repeat("x", 32))
			}
			token, e := jwt.NewWithClaims(method, c).SignedString(signingKey)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = j.Verify(token); !errors.Is(e, model.ErrUnauthorized) {
				t.Fatal("invalid token accepted", e)
			}
		})
	}
	for _, raw := range []string{"", "malformed", strings.Repeat("x", 8193)} {
		if _, err = j.Verify(raw); err == nil {
			t.Fatal("malformed accepted")
		}
	}
	j.now = func() time.Time { return now.Add(15 * time.Minute) }
	if _, err = j.Verify(raw); err == nil {
		t.Fatal("expiry boundary accepted")
	}
}
