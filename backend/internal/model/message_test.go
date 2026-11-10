package model

import (
	"strings"
	"testing"
	"time"
)

func TestMessageValidation(t *testing.T) {
	for _, text := range []string{"", "  ", strings.Repeat("x", 4001), string([]byte{255})} {
		if ValidateMessage("a", "b", text) == nil {
			t.Errorf("accepted invalid text")
		}
	}
	if ValidateMessage("a", "b", "hello 👋") != nil {
		t.Fatal("rejected Persian text")
	}
	if ValidateMessage("a", "a", "hello") == nil {
		t.Fatal("accepted self message")
	}
}
func TestSessionExpiry(t *testing.T) {
	now := time.Now()
	s := Session{ExpiresAt: now}
	if s.Active(now) {
		t.Fatal("active at expiry")
	}
	s.ExpiresAt = now.Add(time.Hour)
	s.Revoked = true
	if s.Active(now) {
		t.Fatal("revoked session active")
	}
}
