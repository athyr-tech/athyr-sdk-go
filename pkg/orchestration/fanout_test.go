package orchestration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestFanOut_Execute_SingleAgent(t *testing.T) {
	client := newMockAgent().
		OnRequest("agent.analyze.invoke", []byte("analysis result"))

	fanout := NewFanOut("test").
		Agent("analyzer", "agent.analyze.invoke")

	result, err := fanout.Execute(context.Background(), client, []byte("input"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Single agent result wrapped in JSON-like format
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}

	if len(client.calls) != 1 {
		t.Errorf("expected 1 call, got %d", len(client.calls))
	}
}

func TestFanOut_Execute_MultipleAgents(t *testing.T) {
	client := newMockAgent().
		OnRequest("agent.fundamental.invoke", []byte("fundamental analysis")).
		OnRequest("agent.technical.invoke", []byte("technical analysis")).
		OnRequest("agent.sentiment.invoke", []byte("sentiment analysis"))

	fanout := NewFanOut("stock-analysis").
		Agent("fundamental", "agent.fundamental.invoke").
		Agent("technical", "agent.technical.invoke").
		Agent("sentiment", "agent.sentiment.invoke")

	result, err := fanout.Execute(context.Background(), client, []byte("AAPL"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) == 0 {
		t.Error("expected non-empty result")
	}

	// All three agents should have been called
	if len(client.calls) != 3 {
		t.Errorf("expected 3 calls, got %d", len(client.calls))
	}

	// All should receive same input
	for i, call := range client.calls {
		if string(call.data) != "AAPL" {
			t.Errorf("call %d: expected input 'AAPL', got '%s'", i, call.data)
		}
	}
}

func TestFanOut_AllMustSucceed_Success(t *testing.T) {
	client := newMockAgent().
		OnRequest("agent.a.invoke", []byte("result-a")).
		OnRequest("agent.b.invoke", []byte("result-b"))

	fanout := NewFanOut("test").
		Agent("a", "agent.a.invoke").
		Agent("b", "agent.b.invoke")

	result, err := fanout.Execute(context.Background(), client, []byte("input"), AllMustSucceed())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
}

func TestFanOut_AllMustSucceed_Failure(t *testing.T) {
	expectedErr := errors.New("agent b failed")
	client := newMockAgent().
		OnRequest("agent.a.invoke", []byte("result-a")).
		OnRequestError("agent.b.invoke", expectedErr)

	fanout := NewFanOut("test").
		Agent("a", "agent.a.invoke").
		Agent("b", "agent.b.invoke")

	_, err := fanout.Execute(context.Background(), client, []byte("input"), AllMustSucceed())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var fanoutErr *FanOutError
	if !errors.As(err, &fanoutErr) {
		t.Fatalf("expected FanOutError, got %T", err)
	}

	if fanoutErr.FailedAgent != "b" {
		t.Errorf("expected failed agent 'b', got '%s'", fanoutErr.FailedAgent)
	}
}

func TestFanOut_FirstSuccess_AllSucceed(t *testing.T) {
	client := newMockAgent().
		OnRequest("agent.primary.invoke", []byte("primary result")).
		OnRequest("agent.backup.invoke", []byte("backup result"))

	fanout := NewFanOut("redundant").
		Agent("primary", "agent.primary.invoke").
		Agent("backup", "agent.backup.invoke")

	result, err := fanout.Execute(context.Background(), client, []byte("input"), FirstSuccess())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return one of the results (map iteration order not guaranteed)
	resultStr := string(result)
	if resultStr != "primary result" && resultStr != "backup result" {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestFanOut_FirstSuccess_PartialFailure(t *testing.T) {
	client := newMockAgent().
		OnRequestError("agent.primary.invoke", errors.New("primary down")).
		OnRequest("agent.backup.invoke", []byte("backup result"))

	fanout := NewFanOut("redundant").
		Agent("primary", "agent.primary.invoke").
		Agent("backup", "agent.backup.invoke")

	result, err := fanout.Execute(context.Background(), client, []byte("input"), FirstSuccess())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(result) != "backup result" {
		t.Errorf("expected 'backup result', got '%s'", result)
	}
}

func TestFanOut_FirstSuccess_AllFail(t *testing.T) {
	client := newMockAgent().
		OnRequestError("agent.a.invoke", errors.New("a failed")).
		OnRequestError("agent.b.invoke", errors.New("b failed"))

	fanout := NewFanOut("test").
		Agent("a", "agent.a.invoke").
		Agent("b", "agent.b.invoke")

	_, err := fanout.Execute(context.Background(), client, []byte("input"), FirstSuccess())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var fanoutErr *FanOutError
	if !errors.As(err, &fanoutErr) {
		t.Fatalf("expected FanOutError, got %T", err)
	}
}

func TestFanOut_CollectAll_WithFailures(t *testing.T) {
	client := newMockAgent().
		OnRequest("agent.a.invoke", []byte("result-a")).
		OnRequestError("agent.b.invoke", errors.New("b failed")).
		OnRequest("agent.c.invoke", []byte("result-c"))

	fanout := NewFanOut("test").
		Agent("a", "agent.a.invoke").
		Agent("b", "agent.b.invoke").
		Agent("c", "agent.c.invoke")

	result, err := fanout.Execute(context.Background(), client, []byte("input"), CollectAll())
	// CollectAll never fails - it just collects what succeeded
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
}

func TestFanOut_RequireQuorum_Met(t *testing.T) {
	client := newMockAgent().
		OnRequest("agent.a.invoke", []byte("result-a")).
		OnRequest("agent.b.invoke", []byte("result-b")).
		OnRequestError("agent.c.invoke", errors.New("c failed"))

	fanout := NewFanOut("voting").
		Agent("a", "agent.a.invoke").
		Agent("b", "agent.b.invoke").
		Agent("c", "agent.c.invoke")

	// Require 2 out of 3
	result, err := fanout.Execute(context.Background(), client, []byte("proposal"), RequireQuorum(2))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
}

func TestFanOut_RequireQuorum_NotMet(t *testing.T) {
	client := newMockAgent().
		OnRequest("agent.a.invoke", []byte("result-a")).
		OnRequestError("agent.b.invoke", errors.New("b failed")).
		OnRequestError("agent.c.invoke", errors.New("c failed"))

	fanout := NewFanOut("voting").
		Agent("a", "agent.a.invoke").
		Agent("b", "agent.b.invoke").
		Agent("c", "agent.c.invoke")

	// Require 2 out of 3, but only 1 succeeded
	_, err := fanout.Execute(context.Background(), client, []byte("proposal"), RequireQuorum(2))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var fanoutErr *FanOutError
	if !errors.As(err, &fanoutErr) {
		t.Fatalf("expected FanOutError, got %T", err)
	}
}

func TestFanOut_ExecuteWithTrace(t *testing.T) {
	client := newMockAgent().
		OnRequest("agent.a.invoke", []byte("output-a")).
		OnRequest("agent.b.invoke", []byte("output-b"))

	fanout := NewFanOut("traced").
		Agent("agent-a", "agent.a.invoke").
		Agent("agent-b", "agent.b.invoke")

	trace, err := fanout.ExecuteWithTrace(context.Background(), client, []byte("input"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if trace.FanOut != "traced" {
		t.Errorf("expected fanout name 'traced', got '%s'", trace.FanOut)
	}

	if len(trace.Agents) != 2 {
		t.Fatalf("expected 2 agent traces, got %d", len(trace.Agents))
	}

	// Check agent-a trace
	if agentA, ok := trace.Agents["agent-a"]; !ok {
		t.Error("expected agent-a in trace")
	} else {
		if agentA.Name != "agent-a" {
			t.Errorf("expected name 'agent-a', got '%s'", agentA.Name)
		}
		if string(agentA.Output) != "output-a" {
			t.Errorf("expected output 'output-a', got '%s'", agentA.Output)
		}
		if agentA.Duration == 0 {
			t.Error("expected non-zero duration")
		}
	}

	// Check trace counts
	if trace.SuccessCount() != 2 {
		t.Errorf("expected 2 successes, got %d", trace.SuccessCount())
	}
	if trace.FailureCount() != 0 {
		t.Errorf("expected 0 failures, got %d", trace.FailureCount())
	}

	if trace.Duration == 0 {
		t.Error("expected non-zero trace duration")
	}
}

func TestFanOut_WithAgentTimeout(t *testing.T) {
	// Verify timeout option is applied
	fanout := NewFanOut("timeout-test").
		Agent("slow", "agent.slow.invoke", WithAgentTimeout(5*time.Second))

	if len(fanout.agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(fanout.agents))
	}
	if fanout.agents[0].opts.timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", fanout.agents[0].opts.timeout)
	}
}

func TestFanOut_EmptyFanOut(t *testing.T) {
	client := newMockAgent()
	fanout := NewFanOut("empty")

	result, err := fanout.Execute(context.Background(), client, []byte("input"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Empty fanout returns empty JSON object
	if string(result) != "{}" {
		t.Errorf("expected '{}', got '%s'", result)
	}

	if len(client.calls) != 0 {
		t.Errorf("expected 0 calls, got %d", len(client.calls))
	}
}

func TestFanOut_Reusable(t *testing.T) {
	// FanOut should be reusable across multiple executions
	client := newMockAgent().
		OnRequest("agent.echo.invoke", []byte("echoed"))

	fanout := NewFanOut("reusable").
		Agent("echo", "agent.echo.invoke")

	// Execute twice
	_, err1 := fanout.Execute(context.Background(), client, []byte("first"))
	_, err2 := fanout.Execute(context.Background(), client, []byte("second"))

	if err1 != nil || err2 != nil {
		t.Error("fanout should be reusable without errors")
	}
	if len(client.calls) != 2 {
		t.Errorf("expected 2 calls, got %d", len(client.calls))
	}
}

func TestFanOut_CustomAggregator(t *testing.T) {
	client := newMockAgent().
		OnRequest("agent.a.invoke", []byte("10")).
		OnRequest("agent.b.invoke", []byte("20"))

	fanout := NewFanOut("custom").
		Agent("a", "agent.a.invoke").
		Agent("b", "agent.b.invoke")

	// Custom aggregator that concatenates results
	result, err := fanout.Execute(context.Background(), client, []byte("input"),
		WithAggregator(func(results map[string]AgentResult) ([]byte, error) {
			var combined string
			for _, r := range results {
				if r.Success() {
					combined += string(r.Output) + ","
				}
			}
			return []byte(combined), nil
		}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Result should contain both values (order not guaranteed)
	resultStr := string(result)
	if !contains(resultStr, "10") || !contains(resultStr, "20") {
		t.Errorf("expected result to contain '10' and '20', got '%s'", result)
	}
}

func TestAgentResult_Success(t *testing.T) {
	successResult := AgentResult{Output: []byte("data")}
	if !successResult.Success() {
		t.Error("expected Success() to return true for result without error")
	}

	failResult := AgentResult{Error: errors.New("failed")}
	if failResult.Success() {
		t.Error("expected Success() to return false for result with error")
	}
}

func TestFanOutError_Error(t *testing.T) {
	// With failed agent
	err1 := &FanOutError{
		FailedAgent: "agent-b",
		Err:         errors.New("connection timeout"),
	}
	expected1 := `fan-out failed: agent "agent-b": connection timeout`
	if err1.Error() != expected1 {
		t.Errorf("expected '%s', got '%s'", expected1, err1.Error())
	}

	// Without failed agent (general failure)
	err2 := &FanOutError{
		Err: errors.New("all agents failed"),
	}
	expected2 := "fan-out failed: all agents failed"
	if err2.Error() != expected2 {
		t.Errorf("expected '%s', got '%s'", expected2, err2.Error())
	}
}

func TestFanOutError_Unwrap(t *testing.T) {
	underlying := errors.New("underlying error")
	err := &FanOutError{Err: underlying}

	if !errors.Is(err, underlying) {
		t.Error("expected Unwrap to return underlying error")
	}
}

// marshalResults tests

func TestMarshalResults_ValidJSON(t *testing.T) {
	results := map[string][]byte{
		"agent-a": []byte(`{"score":42}`),
		"agent-b": []byte(`{"items":["x","y"]}`),
	}

	out := marshalResults(results)

	// Must be valid JSON
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("marshalResults produced invalid JSON: %v\noutput: %s", err, out)
	}

	// JSON values should round-trip as raw JSON
	if string(parsed["agent-a"]) != `{"score":42}` {
		t.Errorf("agent-a: expected {\"score\":42}, got %s", parsed["agent-a"])
	}
	if string(parsed["agent-b"]) != `{"items":["x","y"]}` {
		t.Errorf("agent-b: expected {\"items\":[\"x\",\"y\"]}, got %s", parsed["agent-b"])
	}
}

func TestMarshalResults_SpecialChars(t *testing.T) {
	results := map[string][]byte{
		"agent": []byte(`he said "hello" and \ backslash`),
	}

	out := marshalResults(results)

	// Must be valid JSON even with quotes and backslashes in values
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("marshalResults produced invalid JSON for special chars: %v\noutput: %s", err, out)
	}

	// Non-JSON bytes should be quoted as a string
	var s string
	if err := json.Unmarshal(parsed["agent"], &s); err != nil {
		t.Fatalf("failed to unmarshal agent value as string: %v", err)
	}
	if s != `he said "hello" and \ backslash` {
		t.Errorf("expected original string, got %q", s)
	}
}

func TestMarshalResults_Empty(t *testing.T) {
	out := marshalResults(map[string][]byte{})
	if string(out) != "{}" {
		t.Errorf("expected '{}', got '%s'", out)
	}

	out2 := marshalResults(nil)
	if string(out2) != "{}" {
		t.Errorf("expected '{}' for nil, got '%s'", out2)
	}
}

func TestMarshalResults_NonJSON(t *testing.T) {
	results := map[string][]byte{
		"agent": []byte("plain text response"),
	}

	out := marshalResults(results)

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("marshalResults produced invalid JSON for non-JSON bytes: %v\noutput: %s", err, out)
	}

	// Non-JSON bytes should be string-quoted
	var s string
	if err := json.Unmarshal(parsed["agent"], &s); err != nil {
		t.Fatalf("expected non-JSON bytes to be quoted as string: %v", err)
	}
	if s != "plain text response" {
		t.Errorf("expected 'plain text response', got %q", s)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
