package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"mini-messenger/internal/model"
)

type RefreshSecrets struct{}

func (RefreshSecrets) Generate() (string, string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", "", err
	}
	raw := base64.RawURLEncoding.EncodeToString(b[:])
	hash, err := (RefreshSecrets{}).Digest(raw)
	return raw, hash, err
}
func (RefreshSecrets) Digest(raw string) (string, error) {
	if len(raw) != 43 {
		return "", model.ErrUnauthorized
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(b) != 32 || base64.RawURLEncoding.EncodeToString(b) != raw {
		return "", model.ErrUnauthorized
	}
	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:]), nil
}
