package athyr

import (
	"testing"
)

func TestHandleRaw_PackageLevel(t *testing.T) {
	server := NewServer("localhost:9090")

	handler := func(ctx Context, data []byte) ([]byte, error) {
		return data, nil
	}

	result := HandleRaw(server, "test.raw", handler)

	if result != server {
		t.Error("HandleRaw should return the same server for chaining")
	}
	if len(server.services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(server.services))
	}
	if server.services[0].subject != "test.raw" {
		t.Errorf("expected subject 'test.raw', got %q", server.services[0].subject)
	}
}

func TestEnsureAgentName_Default(t *testing.T) {
	opts := ensureAgentName(nil, "my-service")

	// Apply to a server and check the name
	server := NewServer("localhost:9090", opts...)
	if server.agentName != "my-service" {
		t.Errorf("expected agent name 'my-service', got %q", server.agentName)
	}
}

func TestEnsureAgentName_UserOverride(t *testing.T) {
	userOpts := []ServerOption{WithAgentName("custom-name")}
	opts := ensureAgentName(userOpts, "my-service")

	// User-provided option should take precedence (last-write-wins)
	server := NewServer("localhost:9090", opts...)
	if server.agentName != "custom-name" {
		t.Errorf("expected agent name 'custom-name', got %q", server.agentName)
	}
}

func TestWithServerSystemTLS(t *testing.T) {
	server := NewServer("localhost:9090", WithServerSystemTLS())

	if !server.systemTLS {
		t.Error("WithServerSystemTLS should set systemTLS to true")
	}
	if server.insecure {
		t.Error("WithServerSystemTLS should not set insecure")
	}
}
