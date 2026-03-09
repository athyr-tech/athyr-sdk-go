// Package athyr provides a Go SDK for building agents on the Athyr platform.
// Agents use this package to connect, communicate, and access platform services.
package athyr

import (
	"encoding/json"
	"time"
)

// AgentCard describes an agent's identity and capabilities.
type AgentCard struct {
	Name         string            // Display name
	Description  string            // Purpose and functionality
	Version      string            // Agent software version
	Capabilities []string          // List of capability names
	Metadata     map[string]string // Extensible key-value pairs
}

// Tool defines a function that the LLM can call.
type Tool struct {
	Name        string          // Function name (e.g., "get_weather")
	Description string          // What the tool does
	Parameters  json.RawMessage // JSON Schema defining input parameters
}

// ToolCall represents the LLM's request to invoke a tool.
type ToolCall struct {
	ID        string          // Unique identifier for this call
	Name      string          // Tool name to invoke
	Arguments json.RawMessage // JSON arguments
}

// Message represents a chat message in LLM conversations.
type Message struct {
	Role       string     // "system", "user", "assistant", "tool"
	Content    string     // Text content
	ToolCalls  []ToolCall // Tool calls made by assistant (role="assistant")
	ToolCallID string     // ID of the tool call this responds to (role="tool")
}

// CompletionConfig holds optional parameters for completions.
type CompletionConfig struct {
	Temperature float64  // 0.0-1.0, higher = more random
	MaxTokens   int      // Maximum tokens to generate
	TopP        float64  // Nucleus sampling
	Stop        []string // Stop sequences
}

// CompletionRequest is a request for LLM completion.
type CompletionRequest struct {
	Model         string           // Required: model identifier
	Messages      []Message        // Conversation history
	Config        CompletionConfig // Optional parameters
	SessionID     string           // Optional: for memory injection
	IncludeMemory bool             // Whether to inject memory context
	Tools         []Tool           // Available tools the LLM can call
	ToolChoice    string           // "auto", "none", "required", or specific tool name
}

// TokenUsage tracks token consumption.
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// CompletionResponse is the result of an LLM completion.
type CompletionResponse struct {
	Content      string
	Model        string
	Backend      string
	Usage        TokenUsage
	FinishReason string        // "stop", "length", "tool_calls"
	Latency      time.Duration
	ToolCalls    []ToolCall    // Tool calls requested by LLM
}

// StreamChunk represents a single chunk in a streaming response.
type StreamChunk struct {
	Content   string
	Done      bool
	Usage     *TokenUsage // Only on final chunk
	Model     string      // Only on final chunk
	Backend   string      // Only on final chunk
	Error     string      // Error message if failed
	ToolCalls []ToolCall  // Tool calls (complete on final chunk only)
}

// StreamHandler is called for each chunk in a streaming response.
type StreamHandler func(chunk StreamChunk) error

// Model represents an available LLM model.
type Model struct {
	ID        string
	Name      string
	Backend   string
	Available bool
}

// SessionProfile configures memory session behavior.
type SessionProfile struct {
	Type                   string // "rolling_window"
	MaxTokens              int
	SummarizationThreshold int
}

// DefaultSessionProfile returns sensible defaults.
func DefaultSessionProfile() SessionProfile {
	return SessionProfile{
		Type:                   "rolling_window",
		MaxTokens:              4096,
		SummarizationThreshold: 3000,
	}
}

// Session represents a memory session.
type Session struct {
	ID           string
	AgentID      string
	SystemPrompt string // Agent personality/instructions
	Messages     []SessionMessage
	Summary      string
	Hints        []string
	Profile      SessionProfile
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// SessionMessage represents a message stored in session memory.
type SessionMessage struct {
	Role      string
	Content   string
	Timestamp time.Time
	Tokens    int
}

// SubscribeMessage represents a message received from a subscription.
type SubscribeMessage struct {
	Subject string
	Data    []byte
	Reply   string // Reply subject for request/reply pattern
}

// MessageHandler processes incoming subscription messages.
type MessageHandler func(msg SubscribeMessage)

// Subscription represents an active message subscription.
type Subscription interface {
	Unsubscribe() error
}

// KVEntry represents a value retrieved from the KV store.
type KVEntry struct {
	Value    []byte
	Revision uint64
}

// AgentInfo describes a registered agent on the platform.
type AgentInfo struct {
	ID          string
	Card        AgentCard
	Status      string    // "connected", "disconnected"
	ConnectedAt time.Time
	LastSeen    time.Time
}
