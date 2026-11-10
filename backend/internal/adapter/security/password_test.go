package security

import (
	"golang.org/x/crypto/bcrypt"
	"strings"
	"testing"
)

func TestPassword(t *testing.T) {
	p, err := NewPassword()
	if err != nil {
		t.Fatal(err)
	}
	hash, err := p.Hash("pässword-example")
	if err != nil {
		t.Fatal(err)
	}
	cost, _ := bcrypt.Cost([]byte(hash))
	if cost != PasswordCost || !p.Compare(hash, "pässword-example") {
		t.Fatal("password roundtrip")
	}
	for _, tc := range []struct{ hash, password string }{
		{hash, "wrong"},
		{hash, ""},
		{hash, strings.Repeat("a", 73)},
		{"", "anything"},
		{"broken", "anything"},
	} {
		if p.Compare(tc.hash, tc.password) {
			t.Fatal("invalid password/hash accepted")
		}
	}
	for _, password := range []string{"", strings.Repeat("a", 73)} {
		if _, err = p.Hash(password); err == nil {
			t.Fatal("invalid length accepted")
		}
	}
	hash2, err := p.Hash("pässword-example")
	if err != nil || hash == hash2 {
		t.Fatal("salt not randomized")
	}
}
func TestIDs(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		id, err := (IDs{}).New()
		if err != nil || len(id) != 36 || id[14] != '4' || !strings.ContainsRune("89ab", rune(id[19])) || seen[id] {
			t.Fatal("invalid/random UUID", err)
		}
		seen[id] = true
	}
}
