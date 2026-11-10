package httpadapter

import (
	"mini-messenger/internal/service/auth"
	"net/http"
	"time"
)

func (h *Handler) accessCookieName() string {
	if h.config.SecureCookie {
		return "__Host-mm_session"
	}
	return "mm_session"
}
func (h *Handler) refreshCookieName() string {
	if h.config.SecureCookie {
		return "__Host-mm_refresh"
	}
	return "mm_refresh"
}

func (h *Handler) setCookie(w http.ResponseWriter, token string, expires time.Time) {
	h.writeAuthCookie(w, h.accessCookieName(), token, expires)
}

func (h *Handler) clearCookie(w http.ResponseWriter) {
	h.writeAuthCookie(w, h.accessCookieName(), "", time.Unix(1, 0))
}

// Access and refresh cookies share the same security attributes and host scope.
func (h *Handler) writeAuthCookie(w http.ResponseWriter, name, value string, expires time.Time) {
	maxAge := -1
	if expires.After(time.Now()) {
		maxAge = int(time.Until(expires).Seconds())
		if maxAge < 1 {
			maxAge = 1
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.config.SecureCookie,
		SameSite: http.SameSiteStrictMode,
		Expires:  expires,
		MaxAge:   maxAge,
	})
}

func (h *Handler) authenticated(w http.ResponseWriter, r *http.Request, next func(http.ResponseWriter, *http.Request, auth.Principal)) {
	c, err := r.Cookie(h.accessCookieName())
	if err != nil {
		writeError(w, 401, "unauthenticated")
		return
	}
	p, err := h.authenticate.Execute(r.Context(), c.Value)
	if err != nil {
		domainError(w, r, err)
		return
	}
	next(w, r, p)
}
