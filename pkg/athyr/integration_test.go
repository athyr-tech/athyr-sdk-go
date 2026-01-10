//go:build integration

package athyr

import (
	"context"
	"net"
	"os"
	"sync"
	"testing"
	"time"
)

// Integration tests for the Athyr SDK.
// Run with: go test -tags=integration ./pkg/athyr/...
//
// These tests require a running Athyr server on localhost:9090.
// They will be skipped if the server is not available.

const testAddr = "localhost:9090"

// skipIfNoServer skips the test if Athyr server is not reachable.
func skipIfNoServer(t *testing.T) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", testAddr, 2*time.Second)
	if err != nil {
		t.Skipf("Athyr server not available at %s: %v", testAddr, err)
	}
	conn.Close()
}

// testAgent creates a connected agent for testing.
// Returns the agent and a cleanup function.
func testAgent(t *testing.T, name string) (Agent, func()) {
	t.Helper()
	skipIfNoServer(t)

	agent, err := NewAgent(testAddr,
		WithAgentCard(AgentCard{
			Name:        name,
			Description: "Integration test agent",
			Version:     "test",
		}),
	)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := agent.Connect(ctx); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	return agent, func() {
		agent.Close()
	}
}

// =============================================================================
// Connection Tests
// =============================================================================

func TestIntegration_Connect(t *testing.T) {
	skipIfNoServer(t)

	agent, err := NewAgent(testAddr,
		WithAgentCard(AgentCard{Name: "test-connect"}),
	)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := agent.Connect(ctx); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer agent.Close()

	if !agent.Connected() {
		t.Error("expected agent to be connected")
	}

	if agent.AgentID() == "" {
		t.Error("expected non-empty agent ID")
	}

	if agent.State() != StateConnected {
		t.Errorf("expected state %v, got %v", StateConnected, agent.State())
	}
}

func TestIntegration_MustConnect(t *testing.T) {
	skipIfNoServer(t)

	agent := MustConnect(testAddr,
		WithAgentCard(AgentCard{Name: "test-must-connect"}),
	)
	defer agent.Close()

	if !agent.Connected() {
		t.Error("expected agent to be connected")
	}
}

func TestIntegration_Close(t *testing.T) {
	agent, cleanup := testAgent(t, "test-close")
	defer cleanup()

	if err := agent.Close(); err != nil {
		t.Errorf("unexpected error closing agent: %v", err)
	}

	if agent.Connected() {
		t.Error("expected agent to be disconnected after Close")
	}
}

// =============================================================================
// Pub/Sub Tests
// =============================================================================

func TestIntegration_PubSub(t *testing.T) {
	agent, cleanup := testAgent(t, "test-pubsub")
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	received := make(chan []byte, 1)
	subject := "test.pubsub." + randomSuffix()

	sub, err := agent.Subscribe(ctx, subject, func(msg SubscribeMessage) {
		received <- msg.Data
	})
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	// Give subscription time to establish
	time.Sleep(100 * time.Millisecond)

	testData := []byte("hello pubsub")
	if err := agent.Publish(ctx, subject, testData); err != nil {
		t.Fatalf("failed to publish: %v", err)
	}

	select {
	case data := <-received:
		if string(data) != string(testData) {
			t.Errorf("expected %q, got %q", testData, data)
		}
	case <-time.After(5 * time.Second):
		t.Error("timeout waiting for message")
	}
}

func TestIntegration_QueueSubscribe(t *testing.T) {
	agent, cleanup := testAgent(t, "test-queue-sub")
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var received sync.Map
	subject := "test.queue." + randomSuffix()
	queue := "test-queue"

	// Create two queue subscribers
	sub1, err := agent.QueueSubscribe(ctx, subject, queue, func(msg SubscribeMessage) {
		received.Store("sub1", msg.Data)
	})
	if err != nil {
		t.Fatalf("failed to queue subscribe 1: %v", err)
	}
	defer sub1.Unsubscribe()

	sub2, err := agent.QueueSubscribe(ctx, subject, queue, func(msg SubscribeMessage) {
		received.Store("sub2", msg.Data)
	})
	if err != nil {
		t.Fatalf("failed to queue subscribe 2: %v", err)
	}
	defer sub2.Unsubscribe()

	time.Sleep(100 * time.Millisecond)

	// Publish a message - only one subscriber should receive it
	if err := agent.Publish(ctx, subject, []byte("queue test")); err != nil {
		t.Fatalf("failed to publish: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	count := 0
	received.Range(func(_, _ any) bool {
		count++
		return true
	})

	if count != 1 {
		t.Errorf("expected exactly 1 subscriber to receive message, got %d", count)
	}
}

// =============================================================================
// Request/Reply Tests
// =============================================================================

func TestIntegration_RequestReply(t *testing.T) {
	agent, cleanup := testAgent(t, "test-request-reply")
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	subject := "test.echo." + randomSuffix()

	// Set up a responder
	sub, err := agent.Subscribe(ctx, subject, func(msg SubscribeMessage) {
		if msg.Reply != "" {
			response := []byte("echo: " + string(msg.Data))
			agent.Publish(ctx, msg.Reply, response)
		}
	})
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	time.Sleep(100 * time.Millisecond)

	// Send request
	resp, err := agent.Request(ctx, subject, []byte("hello"))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	expected := "echo: hello"
	if string(resp) != expected {
		t.Errorf("expected %q, got %q", expected, resp)
	}
}

// =============================================================================
// LLM Tests
// =============================================================================

func TestIntegration_Models(t *testing.T) {
	agent, cleanup := testAgent(t, "test-models")
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	models, err := agent.Models(ctx)
	if err != nil {
		t.Fatalf("failed to get models: %v", err)
	}

	// Should have at least one model if Ollama is running
	if len(models) == 0 {
		t.Skip("no models available - is Ollama running?")
	}

	t.Logf("available models: %d", len(models))
	for _, m := range models {
		t.Logf("  - %s (backend: %s)", m.Name, m.Backend)
	}
}

func TestIntegration_Complete(t *testing.T) {
	agent, cleanup := testAgent(t, "test-complete")
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check if models are available
	models, err := agent.Models(ctx)
	if err != nil || len(models) == 0 {
		t.Skip("no models available for completion test")
	}

	resp, err := agent.Complete(ctx, CompletionRequest{
		Model: models[0].Name,
		Messages: []Message{
			{Role: "user", Content: "Say 'hello' and nothing else."},
		},
		Config: CompletionConfig{MaxTokens: 10},
	})
	if err != nil {
		t.Fatalf("completion failed: %v", err)
	}

	if resp.Content == "" {
		t.Error("expected non-empty response content")
	}

	t.Logf("response: %s", resp.Content)
}

func TestIntegration_CompleteStream(t *testing.T) {
	agent, cleanup := testAgent(t, "test-stream")
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	models, err := agent.Models(ctx)
	if err != nil || len(models) == 0 {
		t.Skip("no models available for streaming test")
	}

	var chunks []string
	err = agent.CompleteStream(ctx, CompletionRequest{
		Model: models[0].Name,
		Messages: []Message{
			{Role: "user", Content: "Count from 1 to 3."},
		},
		Config: CompletionConfig{MaxTokens: 20},
	}, func(chunk StreamChunk) error {
		chunks = append(chunks, chunk.Content)
		return nil
	})
	if err != nil {
		t.Fatalf("streaming failed: %v", err)
	}

	if len(chunks) == 0 {
		t.Error("expected at least one chunk")
	}

	t.Logf("received %d chunks", len(chunks))
}

// =============================================================================
// Helpers
// =============================================================================

func randomSuffix() string {
	return time.Now().Format("150405.000")
}

func init() {
	// Allow override via environment variable
	if addr := os.Getenv("ATHYR_TEST_ADDR"); addr != "" {
		// Note: testAddr is const, so we can't change it
		// but tests could check this env var
	}
}