package bootstrap

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTransportRestrictions(t *testing.T) {
	base := Config{
		Address:     "127.0.0.1:8080",
		Origin:      "http://localhost:8080",
		DatabaseURL: "postgres://localhost/demo",
	}
	if err := validateTransport(base); err != nil {
		t.Fatal(err)
	}
	public := base
	public.Address = "0.0.0.0:8080"
	if validateTransport(public) == nil {
		t.Fatal("public plaintext listener accepted")
	}
	secure := base
	secure.Origin = "https://chat.example.test"
	secure.SecureCookie = true
	if validateTransport(secure) == nil {
		t.Fatal("HTTPS without certificate accepted")
	}
	secure.TLSCertFile = "certificate.pem"
	secure.TLSKeyFile = "private-key.pem"
	if err := validateTransport(secure); err != nil {
		t.Fatal(err)
	}
	secure.DatabaseURL = "postgres://remote.example/demo?sslmode=require"
	if validateTransport(secure) == nil {
		t.Fatal("unverified remote database accepted")
	}
	secure.DatabaseURL = "postgres://remote.example/demo?sslmode=verify-full"
	if err := validateTransport(secure); err != nil {
		t.Fatal(err)
	}
}
func TestSecurityHeaders(t *testing.T) {
	handler := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }), "https://chat.example.test")
	for _, encrypted := range []bool{false, true} {
		request := httptest.NewRequest("GET", "/", nil)
		if encrypted {
			request.TLS = &tls.ConnectionState{}
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if (response.Header().Get("Strict-Transport-Security") != "") != encrypted {
			t.Fatal("wrong HSTS scope")
		}
		if response.Header().Get("Content-Security-Policy") == "" || response.Header().Get("X-Frame-Options") != "DENY" {
			t.Fatal("missing security headers")
		}
	}
}
