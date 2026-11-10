package realtime

import (
	"errors"
	"github.com/gorilla/websocket"
	"mini-messenger/internal/adapter/observability"
	"mini-messenger/internal/model"
	"mini-messenger/internal/service/auth"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Handler struct {
	hub          *Hub
	authenticate *auth.Authenticate
	origin       string
	upgrader     websocket.Upgrader
}

func NewHandler(hub *Hub, authenticate *auth.Authenticate, origin string) *Handler {
	return &Handler{
		hub:          hub,
		authenticate: authenticate,
		origin:       origin,
		upgrader: websocket.Upgrader{
			HandshakeTimeout: 5 * time.Second,
			CheckOrigin:      func(r *http.Request) bool { return originAllowed(origin, r.Header.Get("Origin")) },
		},
	}
}
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", 405)
		return
	}
	if !originAllowed(h.origin, r.Header.Get("Origin")) {
		http.Error(w, "forbidden origin", 403)
		return
	}
	cookieName := "mm_session"
	if strings.HasPrefix(h.origin, "https://") {
		cookieName = "__Host-mm_session"
	}
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		http.Error(w, "unauthenticated", 401)
		return
	}
	principal, err := h.authenticate.Execute(r.Context(), cookie.Value)
	if err != nil {
		status := 500
		if !errors.Is(err, model.ErrUnauthorized) {
			observability.Report(r.Context(), "websocket authentication", err)
		}
		if errors.Is(err, model.ErrUnauthorized) {
			status = 401
		}
		http.Error(w, http.StatusText(status), status)
		return
	}
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &client{
		presence:  true,
		conn:      conn,
		hub:       h.hub,
		userID:    principal.UserID,
		sessionID: principal.SessionID,
		expiresAt: principal.ExpiresAt,
		send:      make(chan []byte, queueSize),
		done:      make(chan struct{}),
	}
	if !h.hub.register(c) {
		c.close()
		return
	}
	// Register before rechecking so concurrent logout can find and close this connection.
	checked, err := h.authenticate.Execute(r.Context(), cookie.Value)
	if err != nil || checked.UserID != principal.UserID || checked.SessionID != principal.SessionID || !h.hub.activate(c) {
		h.hub.remove(c)
		return
	}
	go c.writeLoop()
	c.readLoop()
}

func originAllowed(expected, actual string) bool {
	if actual == expected {
		return true
	}
	want, wantErr := url.Parse(expected)
	got, gotErr := url.Parse(actual)
	if wantErr != nil || gotErr != nil || want.Scheme != "http" || got.Scheme != "http" || want.Port() != got.Port() {
		return false
	}
	return localOriginHost(want.Hostname()) && localOriginHost(got.Hostname())
}

func localOriginHost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
