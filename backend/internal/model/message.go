package model

import (
	"strings"
	"time"
	"unicode/utf8"
)

const MaxMessageRunes = 4000

type Message struct {
	ID, SenderID, RecipientID, Text string
	IdempotencyKey                  string
	CreatedAt                       time.Time
}

func ValidateMessage(sender, recipient, text string) error {
	if sender == "" || recipient == "" || sender == recipient || !utf8.ValidString(text) || strings.TrimSpace(text) == "" || utf8.RuneCountInString(text) > MaxMessageRunes {
		return ErrInvalid
	}
	return nil
}
