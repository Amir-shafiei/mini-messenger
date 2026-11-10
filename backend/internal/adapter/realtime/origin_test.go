package realtime

import "testing"

func TestOriginAllowed(t *testing.T) {
	if !originAllowed("http://localhost:8080", "http://127.0.0.1:8080") {
		t.Fatal("local Docker alias rejected")
	}
	for _, origin := range []string{"http://127.0.0.1:8081", "http://example.com:8080", "https://127.0.0.1:8080"} {
		if originAllowed("http://localhost:8080", origin) {
			t.Fatalf("unexpected origin accepted: %s", origin)
		}
	}
}
