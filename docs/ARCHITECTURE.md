# Athyr SDK Architecture

This document explains the internal architecture of the Athyr Go SDK. It's intended for contributors and developers who
want to understand how the SDK works.

## Overview

```
┌─────────────────────────────────────────────────────────────┐
│                      Your Application                       │
├─────────────────────────────────────────────────────────────┤
│                     Athyr SDK (this repo)                   │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────────────┐ │
│  │  Agent  │  │   LLM   │  │ Memory  │  │    Messaging    │ │
│  │Lifecycle│  │Complete │  │Sessions │  │ Pub/Sub/Request │ │
│  └────┬────┘  └────┬────┘  └───┬─────┘  └────────┬────────┘ │
│       └────────────┴───────────┴─────────────────┘          │
│                           │                                 │
│                    ┌──────┴──────┐                          │
│                    │    gRPC     │                          │
│                    │  Transport  │                          │
│                    └──────┬──────┘                          │
└───────────────────────────┼─────────────────────────────────┘
                            │
                    ┌───────▼───────┐
                    │  Athyr Server │
                    └───────────────┘
```

The SDK provides a clean Go API that communicates with the Athyr server over gRPC.

---

## Agent Lifecycle

### Connection States

```
                    ┌──────────────┐
         ┌─────────►│ Disconnected │◄────────────┐
         │          └──────┬───────┘             │
         │                 │ Connect()           │ Close()
         │                 ▼                     │
         │          ┌──────────────┐             │
         │          │  Connecting  │             │
         │          └──────┬───────┘             │
         │                 │ success             │
         │                 ▼                     │
         │          ┌──────────────┐             │
         │ fail     │  Connected   │─────────────┤
         │          └──────┬───────┘             │
         │                 │ connection lost     │
         │                 ▼                     │
         │          ┌──────────────┐             │
         └──────────│ Reconnecting │─────────────┘
           max      └──────────────┘   success
          retries        │
                         └─► back to Connected
```

### Code Flow

```go
// 1. Create agent (no network yet)
agent, err := athyr.NewAgent("localhost:9090",
athyr.WithAgentCard(athyr.AgentCard{Name: "my-agent"}),
)

// 2. Connect (establishes gRPC, registers with server)
err = agent.Connect(ctx) // State: Disconnected → Connecting → Connected

// 3. Use the agent
agent.Publish(ctx, "subject", data)

// 4. Close (graceful shutdown)
agent.Close() // State: → Disconnected
```

### Auto-Reconnect

When enabled, the SDK automatically reconnects on connection loss:

```go
agent, _ := athyr.NewAgent(addr,
athyr.WithAutoReconnect(10, time.Second), // 10 retries, 1s base backoff
)
```

Backoff formula: `min(baseBackoff * 2^attempt + jitter, maxBackoff)`

---

## Core Components

### Package Structure

```
pkg/athyr/
├── agent.go        # Agent interface & implementation
├── options.go      # AgentOption, ConnectionState
├── messaging.go    # Publish, Subscribe, Request
├── llm.go          # Complete, CompleteStream, Models
├── memory.go       # Session management
├── kv.go           # Key-Value storage
├── middleware.go   # Middleware chain
├── service.go      # Service pattern (Server-side)
├── server.go       # Multi-service server
├── errors.go       # Error types
├── types.go        # Shared types
└── logger.go       # Logger interface
```

### Agent Interface

The `Agent` interface is the main entry point:

```go
type Agent interface {
// Lifecycle
Connect(ctx context.Context) error
Close() error
AgentID() string
Connected() bool
State() ConnectionState

// Messaging
Publish(ctx context.Context, subject string, data []byte) error
Subscribe(ctx context.Context, subject string, handler MessageHandler) (Subscription, error)
QueueSubscribe(ctx context.Context, subject, queue string, handler MessageHandler) (Subscription, error)
Request(ctx context.Context, subject string, data []byte) ([]byte, error)

// LLM
Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
CompleteStream(ctx context.Context, req CompletionRequest, handler StreamHandler) error
Models(ctx context.Context) ([]Model, error)

// Memory
CreateSession(ctx context.Context, profile SessionProfile, systemPrompt string) (*Session, error)
GetSession(ctx context.Context, sessionID string) (*Session, error)
DeleteSession(ctx context.Context, sessionID string) error
AddHint(ctx context.Context, sessionID, hint string) error

// KV
KV(bucket string) KVBucket
}
```

---

## Messaging Patterns

### Publish/Subscribe

```
Publisher                    Athyr Server                 Subscribers
    │                             │                            │
    │  Publish("events.user")     │                            │
    ├────────────────────────────►│                            │
    │                             │  fan-out to all            │
    │                             ├───────────────────────────►│ Sub A
    │                             ├───────────────────────────►│ Sub B
    │                             ├───────────────────────────►│ Sub C
```

### Queue Subscribe (Load Balancing)

```
Publisher                    Athyr Server                 Queue Group
    │                             │                            │
    │  Publish("tasks.process")   │                            │
    ├────────────────────────────►│                            │
    │                             │  round-robin (one only)    │
    │                             ├───────────────────────────►│ Worker A ✓
    │                             │            ╳               │ Worker B
    │                             │            ╳               │ Worker C
```

### Request/Reply

```
Requester                    Athyr Server                   Responder
    │                             │                              │
    │  Request("svc.echo", data)  │                              │
    ├────────────────────────────►│  forward                     │
    │                             ├─────────────────────────────►│
    │                             │                  process     │
    │                             │◄─────────────────────────────┤
    │◄────────────────────────────┤  response                    │
    │         []byte              │                              │
```

---

## LLM Integration

### Completion Flow

```
Agent                        Athyr Server                    LLM Backend
  │                               │                              │
  │  Complete(CompletionRequest)  │                              │
  ├──────────────────────────────►│                              │
  │                               │  route to backend            │
  │                               ├─────────────────────────────►│
  │                               │                   inference  │
  │                               │◄─────────────────────────────┤
  │◄──────────────────────────────┤                              │
  │    CompletionResponse         │                              │
```

### Streaming Flow

```
Agent                        Athyr Server                    LLM Backend
  │                               │                              │
  │  CompleteStream(req, handler) │                              │
  ├──────────────────────────────►│                              │
  │                               ├─────────────────────────────►│
  │   chunk 1                     │◄──────── token ──────────────┤
  │◄──────────────────────────────┤                              │
  │   handler(chunk)              │◄──────── token ──────────────┤
  │◄──────────────────────────────┤                              │
  │   handler(chunk)              │◄──────── token ──────────────┤
  │◄──────────────────────────────┤                              │
  │   chunk.Done = true           │◄──────── done ───────────────┤
  │◄──────────────────────────────┤                              │
```

### Tool Calling Flow

```
┌─────────────────────────────────────────────────────────────────┐
│  1. Send request with tools                                     │
│     Complete(req{Tools: [get_weather, search]})                 │
├─────────────────────────────────────────────────────────────────┤
│  2. LLM decides to call a tool                                  │
│     Response: ToolCalls: [{name: "get_weather", args: {...}}]   │
├─────────────────────────────────────────────────────────────────┤
│  3. Your code executes the tool                                 │
│     result := getWeather(args)                                  │
├─────────────────────────────────────────────────────────────────┤
│  4. Send tool result back                                       │
│     Complete(req{Messages: [..., {role: "tool", content: ...}]})│
├─────────────────────────────────────────────────────────────────┤
│  5. LLM generates final response                                │
│     Response: "The weather in Paris is 18°C and sunny."         │
└─────────────────────────────────────────────────────────────────┘
```

---

## Middleware Chain

Middleware wraps handlers to add cross-cutting concerns.

### How It Works

```
Request → [Recover] → [Metrics] → [RateLimit] → [Handler] → Response
              │           │            │            │
              │           │            │            └─ Your logic
              │           │            └─ Concurrency limit
              │           └─ Timing & metrics
              └─ Panic recovery
```

### Chain Execution Order

```go
// Middleware applied in order: first = outermost
chain := athyr.Chain(
athyr.Recover(logger),   // 1st: catches panics from everything below
athyr.Metrics(callback), // 2nd: times everything below
athyr.RateLimit(100), // 3rd: limits concurrency
)
handler := chain(myHandler)
```

Execution: `Recover → Metrics → RateLimit → myHandler → RateLimit → Metrics → Recover`

### Built-in Middleware

| Middleware    | Purpose                                |
|---------------|----------------------------------------|
| `Recover`     | Catches panics, returns Internal error |
| `Metrics`     | Calls callback with timing data        |
| `RateLimit`   | Limits concurrent requests             |
| `Timeout`     | Enforces request deadline              |
| `Retry`       | Retries on Unavailable errors          |
| `Validate`    | Validates request before processing    |
| `LogRequests` | Logs request/response info             |

---

## Service Pattern

For building request handlers (server-side):

```
┌─────────────────────────────────────────────────────────────┐
│                         Server                              │
│  ┌─────────────────────────────────────────────────────────┐│
│  │ Global Middleware: [Recover] [Metrics]                  ││
│  └─────────────────────────────────────────────────────────┘│
│                                                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │  Service A  │  │  Service B  │  │  Service C  │          │
│  │ "svc.echo"  │  │ "svc.math"  │  │ "svc.user"  │          │
│  │ + local mw  │  │             │  │ + local mw  │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
└─────────────────────────────────────────────────────────────┘
```

### Typed Handlers

```go
// Automatic JSON marshal/unmarshal
func echoHandler(ctx athyr.Context, req EchoRequest) (EchoResponse, error) {
return EchoResponse{Echo: req.Message}, nil
}

server := athyr.NewServer(addr)
athyr.Handle(server, "svc.echo", echoHandler)
server.Run(ctx)
```

### Raw Handlers

```go
// Full control over serialization
func rawHandler(ctx athyr.Context, data []byte) ([]byte, error) {
return append([]byte("got: "), data...), nil
}

server.HandleRaw("svc.raw", rawHandler)
```

---

## Error Handling

### Error Types

```
┌─────────────────────────────────────────────────────────────┐
│                        AthyrError                           │
│  ┌──────────┐  ┌──────────────┐  ┌────────┐  ┌───────────┐  │
│  │   Code   │  │   Message    │  │   Op   │  │   Cause   │  │
│  │not_found │  │ "user 123"   │  │"KV.Get"│  │ gRPC err  │  │
│  └──────────┘  └──────────────┘  └────────┘  └───────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### Error Codes (map to gRPC)

| Code                | gRPC Status      | Use Case               |
|---------------------|------------------|------------------------|
| `not_found`         | NotFound         | Resource doesn't exist |
| `invalid_argument`  | InvalidArgument  | Bad request            |
| `unavailable`       | Unavailable      | Retry later            |
| `internal`          | Internal         | Server error           |
| `unauthenticated`   | Unauthenticated  | Auth required          |
| `permission_denied` | PermissionDenied | Not allowed            |
| `already_exists`    | AlreadyExists    | Duplicate              |
| `deadline_exceeded` | DeadlineExceeded | Timeout                |

### Checking Errors

```go
resp, err := agent.Complete(ctx, req)
if err != nil {
if athyr.IsUnavailable(err) {
// Retry later
}
if athyr.IsNotFound(err) {
// Model not found
}
}
```

### StreamError (Partial Responses)

When streaming fails mid-response:

```go
err := agent.CompleteStream(ctx, req, handler)
var streamErr *athyr.StreamError
if errors.As(err, &streamErr) {
if streamErr.PartialResponse {
// Some content was received before failure
fmt.Println("Received:", streamErr.AccumulatedContent)
}
}
```

---

## gRPC Transport

### Connection Setup

```go
// Insecure (development)
agent, _ := athyr.NewAgent(addr, athyr.WithInsecure())

// System TLS (production)
agent, _ := athyr.NewAgent(addr, athyr.WithSystemTLS())

// Custom TLS
agent, _ := athyr.NewAgent(addr, athyr.WithTLS("/path/to/ca.pem"))
```

### Proto to SDK Type Mapping

| Proto                | SDK Type                   |
|----------------------|----------------------------|
| `CompletionRequest`  | `athyr.CompletionRequest`  |
| `CompletionResponse` | `athyr.CompletionResponse` |
| `StreamChunk`        | `athyr.StreamChunk`        |
| `Message`            | `athyr.Message`            |
| `Tool`               | `athyr.Tool`               |
| `ToolCall`           | `athyr.ToolCall`           |

The SDK handles all proto ↔ Go type conversion internally.

---

## Key Design Decisions

1. **Interface-based API** - `Agent` is an interface for testability
2. **Functional options** - `WithXxx()` pattern for configuration
3. **Context everywhere** - All operations accept `context.Context`
4. **Explicit connection** - `Connect()` is separate from `NewAgent()`
5. **Middleware pattern** - Composable request processing
6. **Typed + Raw handlers** - Convenience with escape hatch
7. **Structured errors** - Machine-readable error codes

---

## File Reference

| File            | Responsibility                           |
|-----------------|------------------------------------------|
| `agent.go`      | Agent interface, connection, heartbeat   |
| `options.go`    | Configuration options, connection states |
| `messaging.go`  | Pub/Sub, Request/Reply                   |
| `llm.go`        | LLM completions, streaming, models       |
| `memory.go`     | Session CRUD, hints                      |
| `kv.go`         | Key-value storage                        |
| `middleware.go` | All middleware implementations           |
| `service.go`    | Service, Handler, Context                |
| `server.go`     | Multi-service server                     |
| `errors.go`     | AthyrError, StreamError, error helpers   |
| `types.go`      | Shared structs (Message, Tool, etc.)     |
| `logger.go`     | Logger interface                         |