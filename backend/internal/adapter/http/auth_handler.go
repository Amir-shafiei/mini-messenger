package httpadapter

import (
	"errors"
	"io"
	"mini-messenger/internal/model"
	"mini-messenger/internal/service/auth"
	"net/http"
	"strings"
	"time"
)

type userJSON struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}
type authJSON struct {
	User      userJSON  `json:"user"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (h *Handler) loginHandler(w http.ResponseWriter, r *http.Request) {
	if !h.limiter.allow(r.RemoteAddr) {
		w.Header().Set("Retry-After", "60")
		writeError(w, 429, "rate_limited")
		return
	}
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	in.Username = strings.ToLower(strings.TrimSpace(in.Username))
	if len(in.Username) < 3 || len(in.Username) > 32 || len(in.Password) == 0 || len(in.Password) > 72 {
		writeError(w, 400, "invalid_input")
		return
	}
	for _, c := range in.Username {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_') {
			writeError(w, 400, "invalid_input")
			return
		}
	}
	out, err := h.login.Execute(r.Context(), auth.LoginInput{Username: in.Username, Password: in.Password})
	if err != nil {
		if errors.Is(err, model.ErrUnauthorized) {
			writeError(w, 401, "invalid_credentials")
		} else {
			domainError(w, r, err)
		}
		return
	}
	refresh, err := h.refresh.Start(r.Context(), out.SessionID)
	if err != nil {
		_ = h.logout.Execute(r.Context(), auth.Principal{UserID: out.User.ID, SessionID: out.SessionID})
		domainError(w, r, err)
		return
	}
	h.setCookie(w, out.Token, out.ExpiresAt)
	h.setRefreshCookie(w, refresh, out.SessionExpiresAt)
	writeJSON(w, 200, authJSON{userJSON{out.User.ID, out.User.Username}, out.ExpiresAt})
}
func (h *Handler) meHandler(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	u, err := h.me.Execute(r.Context(), p.UserID)
	if err != nil {
		domainError(w, r, err)
		return
	}
	writeJSON(w, 200, authJSON{userJSON{u.ID, u.Username}, p.ExpiresAt})
}
func (h *Handler) logoutHandler(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	b, err := io.ReadAll(io.LimitReader(r.Body, 1))
	if err != nil || len(b) > 0 {
		writeError(w, 400, "invalid_input")
		return
	}
	if err = h.logout.Execute(r.Context(), p); err != nil {
		domainError(w, r, err)
		return
	}
	h.clearCookie(w)
	h.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}
