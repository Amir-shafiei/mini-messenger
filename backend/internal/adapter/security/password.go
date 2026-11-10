package security

import (
	"golang.org/x/crypto/bcrypt"
	"mini-messenger/internal/model"
	"mini-messenger/internal/service/auth"
)

const PasswordCost = 12
const MaxPasswordBytes = 72

type Password struct{ dummy string }

var _ auth.PasswordVerifier = (*Password)(nil)

func NewPassword() (*Password, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte("dummy-password-never-used-for-login"), PasswordCost)
	if err != nil {
		return nil, err
	}
	return &Password{dummy: string(hash)}, nil
}
func (p *Password) Hash(plaintext string) (string, error) {
	if len(plaintext) == 0 || len(plaintext) > MaxPasswordBytes {
		return "", model.ErrInvalid
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), PasswordCost)
	return string(hash), err
}
func (p *Password) Compare(hash, plaintext string) bool {
	valid := true
	if len(plaintext) == 0 || len(plaintext) > MaxPasswordBytes {
		valid = false
		plaintext = "invalid-password"
	}
	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil || cost != PasswordCost {
		valid = false
		hash = p.dummy
	}
	err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext))
	return valid && err == nil
}
