package bootstrap

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	for _, key := range []string{
		"DATABASE_URL",
		"JWT_SECRET_BASE64",
		"HTTP_ADDR",
		"APP_ORIGIN",
		"COOKIE_SECURE",
		"SEED_DEMO",
		"DEMO_PASSWORD",
		"TLS_CERT_FILE",
		"TLS_KEY_FILE",
		"FRONTEND_DIR",
	} {
		t.Setenv(key, "")
	}
	if _, err := LoadConfig(); err == nil {
		t.Fatal("missing database accepted")
	}
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET_BASE64", base64.StdEncoding.EncodeToString([]byte(strings.Repeat("x", 32))))
	c, err := LoadConfig()
	if err != nil || c.Address != "127.0.0.1:8080" || c.SeedDemo {
		t.Fatal("defaults", err)
	}
	t.Setenv("SEED_DEMO", "true")
	if _, err = LoadConfig(); err == nil {
		t.Fatal("missing seed password accepted")
	}
	t.Setenv("DEMO_PASSWORD", "local-demo-password")
	if _, err = LoadConfig(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("COOKIE_SECURE", "bad")
	if _, err = LoadConfig(); err == nil {
		t.Fatal("bad boolean accepted")
	}
}
