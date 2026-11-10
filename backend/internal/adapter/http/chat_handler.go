package httpadapter

import (
	"mini-messenger/internal/model"
	"mini-messenger/internal/service/auth"
	"mini-messenger/internal/service/chat"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type messageJSON struct {
	ID          string    `json:"id"`
	SenderID    string    `json:"sender_id"`
	RecipientID string    `json:"recipient_id"`
	Text        string    `json:"text"`
	CreatedAt   time.Time `json:"created_at"`
}

func messageResponse(m model.Message) messageJSON {
	return messageJSON{
		ID:          m.ID,
		SenderID:    m.SenderID,
		RecipientID: m.RecipientID,
		Text:        m.Text,
		CreatedAt:   m.CreatedAt,
	}
}
func validUUID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, c := range id {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}
func (h *Handler) usersHandler(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	users, err := h.users.Execute(r.Context(), p.UserID)
	if err != nil {
		domainError(w, r, err)
		return
	}
	out := make([]userJSON, 0, len(users))
	for _, u := range users {
		out = append(out, userJSON{ID: u.ID, Username: u.Username})
	}
	writeJSON(w, 200, map[string]any{"users": out})
}
func (h *Handler) messagesHandler(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	peer := r.PathValue("user_id")
	if !validUUID(peer) {
		writeError(w, 400, "invalid_input")
		return
	}
	if r.Method == http.MethodPost {
		var in struct {
			Text string `json:"text"`
		}
		if !readJSON(w, r, &in) {
			return
		}
		key := r.Header.Get("Idempotency-Key")
		if !validUUID(key) {
			writeError(w, 400, "invalid_idempotency_key")
			return
		}
		if strings.ContainsRune(in.Text, 0) {
			writeError(w, 400, "invalid_input")
			return
		}
		m, err := h.send.Execute(r.Context(), chat.SendInput{ActorID: p.UserID, RecipientID: peer, Text: in.Text, IdempotencyKey: key})
		if err != nil {
			domainError(w, r, err)
			return
		}
		writeJSON(w, 201, map[string]any{"message": messageResponse(m)})
		return
	}
	query, err := r.URL.Query(), error(nil)
	limit := 50
	if query.Has("limit") {
		limit, err = strconv.Atoi(query.Get("limit"))
		if err != nil || limit < 1 || limit > 100 {
			writeError(w, 400, "invalid_input")
			return
		}
	}
	before := query.Get("before")
	if query.Has("before") && !validUUID(before) {
		writeError(w, 400, "invalid_input")
		return
	}
	messages, err := h.history.Execute(r.Context(), chat.ListInput{ActorID: p.UserID, PeerID: peer, Page: chat.Page{BeforeID: before, Limit: limit}})
	if err != nil {
		domainError(w, r, err)
		return
	}
	out := make([]messageJSON, 0, len(messages))
	for _, m := range messages {
		out = append(out, messageResponse(m))
	}
	var next *string
	if len(out) == limit {
		next = &out[len(out)-1].ID
	}
	writeJSON(w, 200, map[string]any{"messages": out, "next_before": next})
}
