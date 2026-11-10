package httpadapter

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mini-messenger/internal/adapter/observability"
	"mini-messenger/internal/model"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{
		"code":       code,
		"message":    http.StatusText(status),
		"request_id": w.Header().Get("X-Request-ID"),
	}})
}
func domainError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, model.ErrUnauthorized):
		writeError(w, 401, "unauthenticated")
	case errors.Is(err, model.ErrNotFound):
		writeError(w, 404, "not_found")
	case errors.Is(err, model.ErrConflict):
		writeError(w, 409, "idempotency_conflict")
	case errors.Is(err, model.ErrInvalid):
		writeError(w, 400, "invalid_input")
	default:
		observability.Report(r.Context(), "http request", err)
		writeError(w, 500, "internal_error")
	}
}
func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		writeError(w, 415, "unsupported_media_type")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err = dec.Decode(v)
	if err == nil {
		var extra any
		if e := dec.Decode(&extra); e != io.EOF {
			if e == nil {
				err = errors.New("trailing JSON")
			} else {
				err = e
			}
		}
	}
	if err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			writeError(w, 413, "payload_too_large")
		} else {
			writeError(w, 400, "invalid_input")
		}
		return false
	}
	return true
}
