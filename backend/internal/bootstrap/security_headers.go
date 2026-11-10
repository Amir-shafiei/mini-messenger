package bootstrap

import (
	"net/http"
	"strings"
)

func securityHeaders(next http.Handler, origin string) http.Handler {
	socketOrigin := strings.Replace(origin, "https://", "wss://", 1)
	socketOrigin = strings.Replace(socketOrigin, "http://", "ws://", 1)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		policy := []string{
			"default-src 'self'",
			"script-src 'self'",
			"style-src 'self' 'unsafe-inline'",
			"connect-src 'self' " + socketOrigin,
			"object-src 'none'",
			"base-uri 'none'",
			"frame-ancestors 'none'",
			"form-action 'self'",
		}
		w.Header().Set("Content-Security-Policy", strings.Join(policy, "; "))
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		}
		next.ServeHTTP(w, r)
	})
}
