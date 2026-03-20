package athyrtest

import (
	"context"
	"errors"
	"testing"

	"github.com/athyr-tech/athyr-sdk-go/pkg/athyr"
)

// Compile-time interface check.
var _ athyr.Agent = (*MockAgent)(nil)

func TestMockAgent_Defaults(t *testing.T) {
	mock := &MockAgent{}

	if err := mock.Connect(context.Background()); err != nil {
		t.Errorf("Connect default should return nil, got %v", err)
	}
	if err := mock.Close(); err != nil {
		t.Errorf("Close default should return nil, got %v", err)
	}
	if id := mock.AgentID(); id != "" {
		t.Errorf("AgentID default should return empty, got %q", id)
	}
	if mock.Connected() {
		t.Error("Connected default should return false")
	}
	if mock.State() != athyr.StateDisconnected {
		t.Errorf("State default should return StateDisconnected, got %v", mock.State())
	}

	resp, err := mock.Request(context.Background(), "test", nil)
	if err != nil || resp != nil {
		t.Errorf("Request default should return nil, nil; got %v, %v", resp, err)
	}
}

func TestMockAgent_Overrides(t *testing.T) {
	expectedErr := errors.New("request failed")

	mock := &MockAgent{
		AgentIDFunc: func() string { return "test-agent-123" },
		ConnectedFunc: func() bool { return true },
		RequestFunc: func(ctx context.Context, subject string, data []byte) ([]byte, error) {
			if subject == "fail" {
				return nil, expectedErr
			}
			return []byte("response for " + subject), nil
		},
	}

	if id := mock.AgentID(); id != "test-agent-123" {
		t.Errorf("expected 'test-agent-123', got %q", id)
	}
	if !mock.Connected() {
		t.Error("expected Connected to return true")
	}

	resp, err := mock.Request(context.Background(), "ok", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp) != "response for ok" {
		t.Errorf("expected 'response for ok', got %q", resp)
	}

	_, err = mock.Request(context.Background(), "fail", nil)
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}
}
