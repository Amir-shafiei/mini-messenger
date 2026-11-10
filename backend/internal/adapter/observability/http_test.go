package observability

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestCorrelationAndRedaction(t *testing.T) {
	var logs bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	defer slog.SetDefault(old)
	h := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if RequestID(r.Context()) == "" {
			t.Error("missing context id")
		}
		Report(r.Context(), "save message", errors.New("SECRET_BODY_PASSWORD"))
		w.WriteHeader(500)
	}))
	req := httptest.NewRequest("POST", "/api/v1/users/secret/messages?token=SECRET_QUERY", strings.NewReader("SECRET_BODY"))
	req.Header.Set("X-Request-ID", "FORGED")
	req.Header.Set("Cookie", "SECRET_COOKIE")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	id := w.Header().Get("X-Request-ID")
	if len(id) != 32 || !strings.Contains(logs.String(), id) || strings.Count(logs.String(), id) != 2 {
		t.Fatal("missing correlation", logs.String())
	}
	if strings.Contains(logs.String(), "SECRET") || strings.Contains(logs.String(), "FORGED") || strings.Contains(logs.String(), "/secret/") {
		t.Fatal("sensitive data in log")
	}
	if RequestID(context.Background()) != "" {
		t.Fatal("unscoped id")
	}
}
