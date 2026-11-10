package bootstrap

import (
	"encoding/base64"
	"errors"
	"net"
	"net/url"
	"os"
	"strconv"
)

type Config struct {
	DockerLocalDemo              bool
	TLSCertFile, TLSKeyFile      string
	FrontendDir                  string
	Address, DatabaseURL, Origin string
	JWTKey                       []byte
	SecureCookie, SeedDemo       bool
	DemoPassword                 string
}

func LoadConfig() (Config, error) {
	c := Config{
		FrontendDir:  "../frontend",
		Address:      "127.0.0.1:8080",
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		Origin:       "http://localhost:8080",
		DemoPassword: os.Getenv("DEMO_PASSWORD"),
	}
	if value := os.Getenv("FRONTEND_DIR"); value != "" {
		c.FrontendDir = value
	}
	if value := os.Getenv("HTTP_ADDR"); value != "" {
		c.Address = value
	}
	if value := os.Getenv("APP_ORIGIN"); value != "" {
		c.Origin = value
	}
	if c.DatabaseURL == "" {
		return c, errors.New("DATABASE_URL is required")
	}
	key, err := base64.StdEncoding.DecodeString(os.Getenv("JWT_SECRET_BASE64"))
	if err != nil || len(key) < 32 {
		return c, errors.New("JWT_SECRET_BASE64 must encode at least 32 random bytes")
	}
	c.JWTKey = key
	for name, destination := range map[string]*bool{"DOCKER_LOCAL_DEMO": &c.DockerLocalDemo, "COOKIE_SECURE": &c.SecureCookie, "SEED_DEMO": &c.SeedDemo} {
		if value := os.Getenv(name); value != "" {
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				return c, errors.New(name + " must be true or false")
			}
			*destination = parsed
		}
	}
	if c.SeedDemo && (len(c.DemoPassword) < 12 || len(c.DemoPassword) > 72) {
		return c, errors.New("DEMO_PASSWORD must be 12-72 bytes when SEED_DEMO is enabled")
	}
	c.TLSCertFile = os.Getenv("TLS_CERT_FILE")
	c.TLSKeyFile = os.Getenv("TLS_KEY_FILE")
	if err := validateTransport(c); err != nil {
		return c, err
	}
	return c, nil
}

// Direct TLS avoids trusting arbitrary forwarded headers or an unconfigured proxy.
func validateTransport(config Config) error {
	origin, err := url.Parse(config.Origin)
	if err != nil || origin.Host == "" {
		return errors.New("APP_ORIGIN must be a valid origin")
	}
	if origin.Scheme == "https" {
		if !config.SecureCookie || config.TLSCertFile == "" || config.TLSKeyFile == "" {
			return errors.New("HTTPS requires COOKIE_SECURE=true, TLS_CERT_FILE and TLS_KEY_FILE")
		}
	} else if origin.Scheme == "http" {
		host, _, err := net.SplitHostPort(config.Address)
		localListener := isLoopback(host) || (config.DockerLocalDemo && host == "0.0.0.0")
		if err != nil || !localListener || !isLoopback(origin.Hostname()) || config.SecureCookie || config.TLSCertFile != "" || config.TLSKeyFile != "" {
			return errors.New("plain HTTP is restricted to loopback development")
		}
	} else {
		return errors.New("APP_ORIGIN must use HTTP or HTTPS")
	}
	database, err := url.Parse(config.DatabaseURL)
	if err != nil || database.Host == "" {
		return errors.New("DATABASE_URL must be a PostgreSQL URL")
	}
	localDatabase := isLoopback(database.Hostname()) || (config.DockerLocalDemo && database.Hostname() == "db" && origin.Scheme == "http" && isLoopback(origin.Hostname()))
	if !localDatabase && database.Query().Get("sslmode") != "verify-full" {
		return errors.New("remote PostgreSQL requires sslmode=verify-full")
	}
	return nil
}
func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
