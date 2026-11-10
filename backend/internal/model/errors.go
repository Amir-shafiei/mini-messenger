package model

import "errors"

var (
	ErrConflict     = errors.New("idempotency key reused with different input")
	ErrInvalid      = errors.New("invalid input")
	ErrUnauthorized = errors.New("unauthorized")
	ErrNotFound     = errors.New("not found")
)
