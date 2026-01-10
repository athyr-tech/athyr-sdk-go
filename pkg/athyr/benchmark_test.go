package athyr

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// Middleware Benchmarks
// =============================================================================
//
// These benchmarks measure the overhead of the middleware chain pattern.
// Middleware wraps handlers, so each layer adds function call overhead.
// Understanding this helps users decide how many middleware to stack.

// noopHandler is a minimal handler for measuring pure middleware overhead.
func noopHandler(_ Context, _ []byte) ([]byte, error) {
	return []byte("ok"), nil
}

// Note: mockContext and newMockContext() are defined in service_test.go

// BenchmarkMiddleware_Chain_Empty measures baseline handler call overhead.
// This is our control - calling a handler with no middleware.
func BenchmarkMiddleware_Chain_Empty(b *testing.B) {
	ctx := newMockContext()
	data := []byte("test")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = noopHandler(ctx, data)
	}
}

// BenchmarkMiddleware_Chain_Single measures overhead of one middleware layer.
// The difference from Empty shows the cost of one wrapper function.
func BenchmarkMiddleware_Chain_Single(b *testing.B) {
	// Metrics is a lightweight middleware - just timing
	mw := Metrics(func(string, time.Duration, error) {})
	handler := mw(noopHandler)
	ctx := newMockContext()
	data := []byte("test")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = handler(ctx, data)
	}
}

// BenchmarkMiddleware_Chain_Triple measures realistic middleware stack.
// Production apps often use: Recover + Metrics + custom middleware.
func BenchmarkMiddleware_Chain_Triple(b *testing.B) {
	chain := Chain(
		Recover(nil),                                    // Panic recovery
		Metrics(func(string, time.Duration, error) {}), // Timing
		Validate(func([]byte) error { return nil }),    // Validation
	)
	handler := chain(noopHandler)
	ctx := newMockContext()
	data := []byte("test")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = handler(ctx, data)
	}
}

// BenchmarkMiddleware_Recover measures panic recovery overhead.
// Uses defer/recover which has known performance implications.
func BenchmarkMiddleware_Recover(b *testing.B) {
	handler := Recover(nil)(noopHandler)
	ctx := newMockContext()
	data := []byte("test")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = handler(ctx, data)
	}
}

// BenchmarkMiddleware_Metrics measures timing middleware overhead.
// Calls time.Now() and time.Since() on every request.
func BenchmarkMiddleware_Metrics(b *testing.B) {
	handler := Metrics(func(string, time.Duration, error) {})(noopHandler)
	ctx := newMockContext()
	data := []byte("test")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = handler(ctx, data)
	}
}

// BenchmarkMiddleware_RateLimit measures semaphore-based rate limiting.
// Uses channel operations which can have lock contention.
func BenchmarkMiddleware_RateLimit(b *testing.B) {
	handler := RateLimit(100)(noopHandler)
	ctx := newMockContext()
	data := []byte("test")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = handler(ctx, data)
	}
}

// BenchmarkMiddleware_RateLimit_Contention measures rate limit under load.
// Runs parallel to expose lock contention issues.
func BenchmarkMiddleware_RateLimit_Contention(b *testing.B) {
	handler := RateLimit(100)(noopHandler)
	ctx := newMockContext()
	data := []byte("test")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = handler(ctx, data)
		}
	})
}

// =============================================================================
// Serialization Benchmarks
// =============================================================================
//
// These benchmarks measure the cost of converting between SDK types and
// protobuf messages. This happens on every LLM request/response.

// BenchmarkBuildCompletionRequest_Minimal measures smallest possible request.
func BenchmarkBuildCompletionRequest_Minimal(b *testing.B) {
	agent := &agent{agentID: "bench-agent"}
	req := CompletionRequest{
		Model: "test-model",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = agent.buildCompletionRequest(req)
	}
}

// BenchmarkBuildCompletionRequest_WithTools measures request with tool definitions.
// Tools add JSON schema parsing overhead.
func BenchmarkBuildCompletionRequest_WithTools(b *testing.B) {
	agent := &agent{agentID: "bench-agent"}

	// Realistic tool definition
	params := json.RawMessage(`{
		"type": "object",
		"properties": {
			"location": {"type": "string", "description": "City name"},
			"units": {"type": "string", "enum": ["celsius", "fahrenheit"]}
		},
		"required": ["location"]
	}`)

	req := CompletionRequest{
		Model: "test-model",
		Messages: []Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "What's the weather in Paris?"},
		},
		Tools: []Tool{
			{Name: "get_weather", Description: "Get weather for a location", Parameters: params},
			{Name: "get_time", Description: "Get current time", Parameters: params},
			{Name: "search", Description: "Search the web", Parameters: params},
		},
		Config: CompletionConfig{
			Temperature: 0.7,
			MaxTokens:   1000,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = agent.buildCompletionRequest(req)
	}
}

// BenchmarkBuildCompletionRequest_LargeConversation measures multi-turn chat.
// Conversation history grows over time, affecting serialization cost.
func BenchmarkBuildCompletionRequest_LargeConversation(b *testing.B) {
	agent := &agent{agentID: "bench-agent"}

	// Build 20-message conversation
	messages := make([]Message, 21)
	messages[0] = Message{Role: "system", Content: "You are a helpful assistant."}
	for i := 1; i <= 20; i++ {
		if i%2 == 1 {
			messages[i] = Message{Role: "user", Content: "This is user message number " + string(rune('0'+i))}
		} else {
			messages[i] = Message{Role: "assistant", Content: "This is assistant response number " + string(rune('0'+i))}
		}
	}

	req := CompletionRequest{
		Model:    "test-model",
		Messages: messages,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = agent.buildCompletionRequest(req)
	}
}

// =============================================================================
// Hot Path Benchmarks
// =============================================================================
//
// These benchmarks measure operations that happen on every SDK call.
// Even small inefficiencies here multiply across all operations.

// BenchmarkCheckConnected measures the connection state check.
// Called at the start of every Publish, Subscribe, Complete, etc.
func BenchmarkCheckConnected(b *testing.B) {
	agent := &agent{
		state: StateConnected,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = agent.checkConnected()
	}
}

// BenchmarkCheckConnected_Parallel measures connection check under contention.
// Multiple goroutines checking state simultaneously.
func BenchmarkCheckConnected_Parallel(b *testing.B) {
	agent := &agent{
		state: StateConnected,
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = agent.checkConnected()
		}
	})
}

// BenchmarkSubscriptionTracking measures mutex-protected subscription list.
// Called when creating/destroying subscriptions.
func BenchmarkSubscriptionTracking(b *testing.B) {
	agent := &agent{
		subscriptions: make([]subRecord, 0),
	}
	handler := func(SubscribeMessage) {}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		agent.trackSubscription("test.subject", "queue", handler)
		agent.untrackSubscription("test.subject", "queue")
	}
}

// BenchmarkSubscriptionTracking_Parallel measures tracking under contention.
// Simulates multiple subscriptions being created/destroyed concurrently.
func BenchmarkSubscriptionTracking_Parallel(b *testing.B) {
	agent := &agent{
		subscriptions: make([]subRecord, 0),
		mu:            sync.RWMutex{},
	}
	handler := func(SubscribeMessage) {}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			subject := "test.subject." + string(rune('a'+i%26))
			agent.trackSubscription(subject, "queue", handler)
			agent.untrackSubscription(subject, "queue")
			i++
		}
	})
}

// =============================================================================
// Memory Allocation Benchmarks
// =============================================================================
//
// These focus specifically on allocations. Run with -benchmem to see
// allocs/op. Reducing allocations reduces GC pressure.

// BenchmarkMessageAllocation measures creating Message structs.
func BenchmarkMessageAllocation(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		msg := Message{
			Role:    "user",
			Content: "What is the weather in Paris?",
		}
		_ = msg
	}
}

// BenchmarkToolCallParsing measures JSON unmarshaling of tool arguments.
func BenchmarkToolCallParsing(b *testing.B) {
	args := json.RawMessage(`{"location": "Paris", "units": "celsius"}`)

	type WeatherArgs struct {
		Location string `json:"location"`
		Units    string `json:"units"`
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var parsed WeatherArgs
		_ = json.Unmarshal(args, &parsed)
	}
}

// BenchmarkCompletionRequestAllocation measures full request struct creation.
func BenchmarkCompletionRequestAllocation(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := CompletionRequest{
			Model: "test-model",
			Messages: []Message{
				{Role: "system", Content: "You are helpful."},
				{Role: "user", Content: "Hello!"},
			},
			Config: CompletionConfig{
				Temperature: 0.7,
				MaxTokens:   100,
			},
		}
		_ = req
	}
}