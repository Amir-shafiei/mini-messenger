package security

import (
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"mini-messenger/internal/model"
	"mini-messenger/internal/service/auth"
	"time"
)

const MaxTokenLifetime = 15 * time.Minute

type JWT struct {
	key              []byte
	issuer, audience string
	now              func() time.Time
}
type tokenClaims struct {
	SessionID string `json:"sid"`
	jwt.RegisteredClaims
}

func (c tokenClaims) Validate() error {
	if c.Subject == "" || c.SessionID == "" || c.IssuedAt == nil || c.ExpiresAt == nil || !c.ExpiresAt.After(c.IssuedAt.Time) || c.ExpiresAt.Sub(c.IssuedAt.Time) > MaxTokenLifetime {
		return model.ErrUnauthorized
	}
	return nil
}

var _ auth.TokenService = (*JWT)(nil)

// Key must be generated with a cryptographically secure RNG; length alone does not prove entropy.
func NewJWT(key []byte, issuer, audience string) (*JWT, error) {
	if len(key) < 32 || issuer == "" || audience == "" {
		return nil, errors.New("JWT requires a random key of at least 32 bytes, issuer and audience")
	}
	return &JWT{key: append([]byte(nil), key...), issuer: issuer, audience: audience, now: time.Now}, nil
}
func (j *JWT) Issue(c auth.Claims) (string, error) {
	now := j.now().UTC().Truncate(time.Second)
	exp := c.ExpiresAt.UTC().Truncate(time.Second)
	if c.UserID == "" || c.SessionID == "" || !exp.After(now) || exp.Sub(now) > MaxTokenLifetime {
		return "", model.ErrInvalid
	}
	claims := tokenClaims{
		SessionID: c.SessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   c.UserID,
			Issuer:    j.issuer,
			Audience:  jwt.ClaimStrings{j.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(j.key)
}
func (j *JWT) Verify(raw string) (auth.Claims, error) {
	if len(raw) == 0 || len(raw) > 8192 {
		return auth.Claims{}, model.ErrUnauthorized
	}
	c := new(tokenClaims)
	t, err := jwt.ParseWithClaims(raw, c, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, model.ErrUnauthorized
		}
		return j.key, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(j.issuer), jwt.WithAudience(j.audience), jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithTimeFunc(j.now))
	if err != nil || !t.Valid {
		return auth.Claims{}, model.ErrUnauthorized
	}
	return auth.Claims{UserID: c.Subject, SessionID: c.SessionID, ExpiresAt: c.ExpiresAt.Time}, nil
}
