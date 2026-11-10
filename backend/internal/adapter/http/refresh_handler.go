package httpadapter

import (
	"errors"
	"io"
	"mini-messenger/internal/model"
	"net/http"
	"time"
)

func (h *Handler) setRefreshCookie(w http.ResponseWriter, token string, expires time.Time) {
	h.writeAuthCookie(w, h.refreshCookieName(), token, expires)
}

func (h *Handler) clearRefreshCookie(w http.ResponseWriter) {
	h.writeAuthCookie(w, h.refreshCookieName(), "", time.Unix(1, 0))
}

func (h *Handler) refreshHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1))
	if err != nil || len(body) != 0 {
		writeError(w, 400, "invalid_input")
		return
	}
	c, err := r.Cookie(h.refreshCookieName())
	if err != nil {
		h.clearCookie(w)
		h.clearRefreshCookie(w)
		writeError(w, 401, "unauthenticated")
		return
	}
	out, err := h.refresh.Execute(r.Context(), c.Value)
	if err != nil {
		if errors.Is(err, model.ErrUnauthorized) {
			h.clearCookie(w)
			h.clearRefreshCookie(w)
		}
		domainError(w, r, err)
		return
	}
	user, err := h.me.Execute(r.Context(), out.UserID)
	if err != nil {
		domainError(w, r, err)
		return
	}
	h.setCookie(w, out.Token, out.ExpiresAt)
	h.setRefreshCookie(w, out.RefreshToken, out.SessionExpiresAt)
	writeJSON(w, 200, authJSON{userJSON{user.ID, user.Username}, out.ExpiresAt})
}
