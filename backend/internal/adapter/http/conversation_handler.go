package httpadapter

import (
	"mini-messenger/internal/service/auth"
	"net/http"
)

func (h *Handler) conversationsHandler(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	list, err := h.conversations.Execute(r.Context(), p.UserID)
	if err != nil {
		domainError(w, r, err)
		return
	}
	out := make([]any, 0, len(list))
	for _, c := range list {
		var last *messageJSON
		if c.LastMessage != nil {
			m := messageResponse(*c.LastMessage)
			last = &m
		}
		out = append(out, map[string]any{
			"id":           c.Peer.ID,
			"username":     c.Peer.Username,
			"last_message": last,
			"unread_count": c.UnreadCount,
		})
	}
	writeJSON(w, 200, map[string]any{"conversations": out})
}
