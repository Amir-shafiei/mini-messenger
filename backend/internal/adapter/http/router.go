package httpadapter

import (
	"errors"
	"mini-messenger/internal/service/auth"
	"mini-messenger/internal/service/chat"
	"mini-messenger/internal/service/user"
	"net/http"
	"net/url"
	"strings"
)

type Config struct {
	Origin       string
	SecureCookie bool
}
type Handler struct {
	refresh       *auth.Refresh
	config        Config
	login         *auth.Login
	authenticate  *auth.Authenticate
	logout        *auth.Logout
	me            *user.GetUser
	limiter       *loginLimiter
	users         *user.ListUsers
	send          *chat.SendMessage
	history       *chat.ListMessages
	conversations *chat.ListConversations
	reads         *chat.ReadStatus
}

func NewRouter(
	config Config,
	login *auth.Login,
	authenticate *auth.Authenticate,
	logout *auth.Logout,
	me *user.GetUser,
	users *user.ListUsers,
	send *chat.SendMessage,
	history *chat.ListMessages,
	reads *chat.ReadStatus,
	conversations *chat.ListConversations,
	refresh *auth.Refresh,
) (http.Handler, error) {
	u, err := url.Parse(config.Origin)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return nil, errors.New("origin must be an exact HTTP(S) origin")
	}
	if config.SecureCookie && u.Scheme != "https" {
		return nil, errors.New("secure cookies require HTTPS origin")
	}
	if !config.SecureCookie && (u.Scheme != "http" || (u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" && u.Hostname() != "::1")) {
		return nil, errors.New("insecure cookie is allowed only for local HTTP development")
	}
	if login == nil || authenticate == nil || logout == nil || me == nil || users == nil || send == nil || history == nil || reads == nil || conversations == nil || refresh == nil {
		return nil, errors.New("missing HTTP use cases")
	}
	h := &Handler{
		refresh:       refresh,
		config:        config,
		login:         login,
		authenticate:  authenticate,
		logout:        logout,
		me:            me,
		limiter:       newLoginLimiter(),
		users:         users,
		send:          send,
		history:       history,
		reads:         reads,
		conversations: conversations,
	}
	return http.HandlerFunc(h.serve), nil
}
func (h *Handler) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.URL.Path == "/api/v1/users" || r.URL.Path == "/api/v1/conversations" {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			writeError(w, 405, "method_not_allowed")
			return
		}
		if r.URL.Path == "/api/v1/conversations" {
			h.authenticated(w, r, h.conversationsHandler)
		} else {
			h.authenticated(w, r, h.usersHandler)
		}
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if len(parts) == 5 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "users" && (parts[4] == "messages" || parts[4] == "read") {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			w.Header().Set("Allow", "GET, POST")
			writeError(w, 405, "method_not_allowed")
			return
		}
		if r.Method == http.MethodPost && r.Header.Get("Origin") != h.config.Origin {
			writeError(w, 403, "forbidden_origin")
			return
		}
		r.SetPathValue("user_id", parts[3])
		if parts[4] == "read" {
			h.authenticated(w, r, h.readHandler)
		} else {
			h.authenticated(w, r, h.messagesHandler)
		}
		return
	}
	var method string
	switch r.URL.Path {
	case "/api/v1/auth/login", "/api/v1/auth/logout", "/api/v1/auth/refresh":
		method = http.MethodPost
	case "/api/v1/auth/me":
		method = http.MethodGet
	default:
		writeError(w, 404, "not_found")
		return
	}
	if r.Method != method {
		w.Header().Set("Allow", method)
		writeError(w, 405, "method_not_allowed")
		return
	}
	if method == http.MethodPost && r.Header.Get("Origin") != h.config.Origin {
		writeError(w, 403, "forbidden_origin")
		return
	}
	switch r.URL.Path {
	case "/api/v1/auth/refresh":
		h.refreshHandler(w, r)
	case "/api/v1/auth/login":
		h.loginHandler(w, r)
	case "/api/v1/auth/me":
		h.authenticated(w, r, h.meHandler)
	case "/api/v1/auth/logout":
		h.authenticated(w, r, h.logoutHandler)
	}
}
