package bootstrap

import "testing"

func TestDockerDemoTransportBoundary(t *testing.T) {
	c := Config{DockerLocalDemo: true, Address: "0.0.0.0:8080", Origin: "http://localhost:8080", DatabaseURL: "postgres://db/messenger?sslmode=disable"}
	if err := validateTransport(c); err != nil {
		t.Fatal(err)
	}
	c.DockerLocalDemo = false
	if validateTransport(c) == nil {
		t.Fatal("container exception enabled without explicit demo mode")
	}
	c.DockerLocalDemo = true
	c.Origin = "http://example.com"
	if validateTransport(c) == nil {
		t.Fatal("public plaintext origin accepted")
	}
	c.Origin = "http://localhost:8080"
	c.DatabaseURL = "postgres://example.com/messenger?sslmode=disable"
	if validateTransport(c) == nil {
		t.Fatal("remote plaintext database accepted")
	}
}
