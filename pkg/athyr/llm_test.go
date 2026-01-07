package athyr

import (
	"encoding/json"
	"testing"
)

func TestBuildCompletionRequest_WithTools(t *testing.T) {
	// Create an agent with minimal setup for testing
	a := &agent{agentID: "test-agent"}

	tools := []Tool{
		{
			Name:        "get_weather",
			Description: "Get current weather for a city",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}`),
		},
		{
			Name:        "search",
			Description: "Search the web",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}}}`),
		},
	}

	req := CompletionRequest{
		Model:      "gpt-4",
		Messages:   []Message{{Role: "user", Content: "What's the weather in Paris?"}},
		Tools:      tools,
		ToolChoice: "auto",
	}

	protoReq := a.buildCompletionRequest(req)

	// Verify tools are converted
	if len(protoReq.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(protoReq.Tools))
	}

	if protoReq.Tools[0].Name != "get_weather" {
		t.Errorf("expected tool name 'get_weather', got '%s'", protoReq.Tools[0].Name)
	}
	if protoReq.Tools[0].Description != "Get current weather for a city" {
		t.Errorf("unexpected description: %s", protoReq.Tools[0].Description)
	}
	if protoReq.Tools[0].Parameters != `{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}` {
		t.Errorf("unexpected parameters: %s", protoReq.Tools[0].Parameters)
	}

	// Verify tool choice
	if protoReq.ToolChoice != "auto" {
		t.Errorf("expected tool choice 'auto', got '%s'", protoReq.ToolChoice)
	}
}

func TestBuildCompletionRequest_WithToolCalls(t *testing.T) {
	a := &agent{agentID: "test-agent"}

	// Simulate a conversation with tool calls
	messages := []Message{
		{Role: "user", Content: "What's the weather in Paris?"},
		{
			Role: "assistant",
			ToolCalls: []ToolCall{
				{
					ID:        "call_123",
					Name:      "get_weather",
					Arguments: json.RawMessage(`{"city":"Paris"}`),
				},
			},
		},
		{
			Role:       "tool",
			ToolCallID: "call_123",
			Content:    `{"temperature": 22, "condition": "sunny"}`,
		},
	}

	req := CompletionRequest{
		Model:    "gpt-4",
		Messages: messages,
	}

	protoReq := a.buildCompletionRequest(req)

	// Verify messages are converted correctly
	if len(protoReq.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(protoReq.Messages))
	}

	// Check assistant message with tool calls
	assistantMsg := protoReq.Messages[1]
	if assistantMsg.Role != "assistant" {
		t.Errorf("expected role 'assistant', got '%s'", assistantMsg.Role)
	}
	if len(assistantMsg.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(assistantMsg.ToolCalls))
	}
	if assistantMsg.ToolCalls[0].Id != "call_123" {
		t.Errorf("expected tool call ID 'call_123', got '%s'", assistantMsg.ToolCalls[0].Id)
	}
	if assistantMsg.ToolCalls[0].Name != "get_weather" {
		t.Errorf("expected tool name 'get_weather', got '%s'", assistantMsg.ToolCalls[0].Name)
	}
	if assistantMsg.ToolCalls[0].Arguments != `{"city":"Paris"}` {
		t.Errorf("unexpected arguments: %s", assistantMsg.ToolCalls[0].Arguments)
	}

	// Check tool result message
	toolMsg := protoReq.Messages[2]
	if toolMsg.Role != "tool" {
		t.Errorf("expected role 'tool', got '%s'", toolMsg.Role)
	}
	if toolMsg.ToolCallId != "call_123" {
		t.Errorf("expected tool call ID 'call_123', got '%s'", toolMsg.ToolCallId)
	}
}

func TestBuildCompletionRequest_NoTools(t *testing.T) {
	a := &agent{agentID: "test-agent"}

	req := CompletionRequest{
		Model:    "gpt-4",
		Messages: []Message{{Role: "user", Content: "Hello"}},
	}

	protoReq := a.buildCompletionRequest(req)

	if len(protoReq.Tools) != 0 {
		t.Errorf("expected no tools, got %d", len(protoReq.Tools))
	}
	if protoReq.ToolChoice != "" {
		t.Errorf("expected empty tool choice, got '%s'", protoReq.ToolChoice)
	}
}

func TestToolCall_JSONRawMessage(t *testing.T) {
	// Test that json.RawMessage preserves exact JSON
	tc := ToolCall{
		ID:        "call_456",
		Name:      "calculate",
		Arguments: json.RawMessage(`{"a": 1, "b": 2, "op": "add"}`),
	}

	// Verify we can unmarshal arguments into a struct
	var args struct {
		A  int    `json:"a"`
		B  int    `json:"b"`
		Op string `json:"op"`
	}
	if err := json.Unmarshal(tc.Arguments, &args); err != nil {
		t.Fatalf("failed to unmarshal arguments: %v", err)
	}

	if args.A != 1 || args.B != 2 || args.Op != "add" {
		t.Errorf("unexpected arguments: %+v", args)
	}
}

func TestTool_ParametersSchema(t *testing.T) {
	// Test that JSON Schema parameters work correctly
	tool := Tool{
		Name:        "send_email",
		Description: "Send an email",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"to": {"type": "string", "format": "email"},
				"subject": {"type": "string"},
				"body": {"type": "string"}
			},
			"required": ["to", "subject", "body"]
		}`),
	}

	// Verify we can parse the schema
	var schema map[string]interface{}
	if err := json.Unmarshal(tool.Parameters, &schema); err != nil {
		t.Fatalf("failed to unmarshal parameters schema: %v", err)
	}

	if schema["type"] != "object" {
		t.Errorf("expected type 'object', got '%v'", schema["type"])
	}

	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("expected properties to be a map")
	}

	if _, exists := props["to"]; !exists {
		t.Error("expected 'to' property in schema")
	}
}

func TestMessage_ToolCallRoundTrip(t *testing.T) {
	// Test that a message with tool calls can be serialized and deserialized
	original := Message{
		Role: "assistant",
		ToolCalls: []ToolCall{
			{
				ID:        "call_abc",
				Name:      "lookup",
				Arguments: json.RawMessage(`{"key":"value"}`),
			},
		},
	}

	// Serialize
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Deserialize
	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Role != original.Role {
		t.Errorf("role mismatch: %s != %s", decoded.Role, original.Role)
	}
	if len(decoded.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(decoded.ToolCalls))
	}
	if decoded.ToolCalls[0].ID != "call_abc" {
		t.Errorf("tool call ID mismatch")
	}
	if decoded.ToolCalls[0].Name != "lookup" {
		t.Errorf("tool call name mismatch")
	}
}

func TestCompletionRequest_WithMultipleTools(t *testing.T) {
	// Test a realistic multi-tool scenario
	a := &agent{agentID: "test-agent"}

	req := CompletionRequest{
		Model: "gpt-4",
		Messages: []Message{
			{Role: "system", Content: "You are a helpful assistant with access to tools."},
			{Role: "user", Content: "What's the weather in Tokyo and search for Japanese restaurants nearby?"},
		},
		Tools: []Tool{
			{
				Name:        "get_weather",
				Description: "Get weather for a location",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"location":{"type":"string"}}}`),
			},
			{
				Name:        "search_places",
				Description: "Search for places nearby",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"location":{"type":"string"}}}`),
			},
		},
		ToolChoice: "auto",
	}

	protoReq := a.buildCompletionRequest(req)

	if len(protoReq.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(protoReq.Messages))
	}
	if len(protoReq.Tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(protoReq.Tools))
	}
	if protoReq.ToolChoice != "auto" {
		t.Errorf("expected tool choice 'auto', got '%s'", protoReq.ToolChoice)
	}
}
