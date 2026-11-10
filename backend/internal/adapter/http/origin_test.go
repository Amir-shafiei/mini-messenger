package httpadapter

import "testing"

func TestOriginAllowed(t *testing.T) {
	for _, origin := range []string{"http://localhost:8080", "http://127.0.0.1:8080", "http://[::1]:8080"} {
		if !originAllowed("http://localhost:8080", origin) {
			t.Fatalf("local alias rejected: %s", origin)
		}
	}
	for _, origin := range []string{"http://localhost:8081", "http://example.com:8080", "https://localhost:8080", ""} {
		if originAllowed("http://localhost:8080", origin) {
			t.Fatalf("unexpected origin accepted: %s", origin)
		}
	}
	if originAllowed("https://chat.example.test", "https://127.0.0.1") {
		t.Fatal("HTTPS alias accepted")
	}
}
