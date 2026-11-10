package bootstrap

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestFrontendDoesNotCacheOldCode(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.js"), []byte("new client"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/app.js", "/app.js?v=new"} {
		w := httptest.NewRecorder()
		frontendHandler(dir).ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || w.Body.String() != "new client" {
			t.Fatal("stale asset risk", w.Code, w.Header())
		}
	}
}
