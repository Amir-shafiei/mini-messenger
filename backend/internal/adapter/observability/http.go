// Package observability keeps transport logging outside application services.
package observability

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/lib/pq"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

type requestKey struct{}

func RequestID(ctx context.Context) string { id, _ := ctx.Value(requestKey{}).(string); return id }

// Never log error messages: driver details can contain credentials or message text.
func Report(ctx context.Context, operation string, err error) {
	attrs := []any{
		"request_id",
		RequestID(ctx),
		"operation",
		operation,
		"error_type",
		fmt.Sprintf("%T", err),
	}
	var pg *pq.Error
	if errors.As(err, &pg) {
		attrs = append(attrs, "sql_state", string(pg.Code))
	}
	slog.ErrorContext(ctx, "operation failed", attrs...)
}

type response struct {
	http.ResponseWriter
	status int
}

func (w *response) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
		w.ResponseWriter.WriteHeader(code)
	}
}
func (w *response) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(p)
}
func (w *response) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	c, b, e := http.NewResponseController(w.ResponseWriter).Hijack()
	if e == nil {
		w.status = 101
	}
	return c, b, e
}
func (w *response) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func route(r *http.Request) string {
	switch r.URL.Path {
	case "/api/v1/auth/login", "/api/v1/auth/logout", "/api/v1/auth/me", "/api/v1/auth/refresh", "/api/v1/users", "/api/v1/conversations", "/api/v1/ws":
		return r.URL.Path
	}
	p := strings.Split(r.URL.Path, "/")
	if len(p) == 6 && p[1] == "api" && p[2] == "v1" && p[3] == "users" && (p[5] == "messages" || p[5] == "read") {
		return "/api/v1/users/:id/" + p[5]
	}
	return "other"
}
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var bytes [16]byte
		if _, err := rand.Read(bytes[:]); err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		id := hex.EncodeToString(bytes[:])
		w.Header().Set("X-Request-ID", id)
		r = r.WithContext(context.WithValue(r.Context(), requestKey{}, id))
		rw := &response{ResponseWriter: w}
		started := time.Now()
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.ErrorContext(r.Context(), "handler panic", "request_id", id, "panic_type", fmt.Sprintf("%T", recovered))
				if rw.status == 0 {
					http.Error(rw, "internal error", 500)
				}
			}
			status := rw.status
			if status == 0 {
				status = 200
			}
			slog.InfoContext(
				r.Context(),
				"request completed",
				"request_id",
				id,
				"method",
				r.Method,
				"route",
				route(r),
				"status",
				status,
				"duration_ms",
				time.Since(started).Milliseconds(),
			)
		}()
		next.ServeHTTP(rw, r)
	})
}
