# Athyr SDK for Go

[![Go Reference](https://pkg.go.dev/badge/github.com/athyr-tech/athyr-sdk-go.svg)](https://pkg.go.dev/github.com/athyr-tech/athyr-sdk-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/athyr-tech/athyr-sdk-go)](https://goreportcard.com/report/github.com/athyr-tech/athyr-sdk-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

Go SDK for building agents on the [Athyr](https://github.com/athyr-tech/athyr) platform.

## Installation

```bash
go get github.com/athyr-tech/athyr-sdk-go
```

## Quick Start

```go
package main

import (
    "context"
    github.com/athyr-tech/athyr-sdk-go/pkg/athyr
)

func main() {
    // Create and connect agent
    agent, _ := athyr.NewAgent("localhost:9090",
        athyr.WithAgentCard(athyr.AgentCard{
            Name:         "my-agent",
            Description:  "My first Athyr agent",
            Capabilities: []string{"chat"},
        }),
    )

    ctx := context.Background()
    agent.Connect(ctx)
    defer agent.Close()

    // LLM completion
    resp, _ := agent.Complete(ctx, athyr.CompletionRequest{
        Model:    "llama3",
        Messages: []athyr.Message{{Role: "user", Content: "Hello!"}},
    })
    println(resp.Content)
}
```

## Features

### Core Agent

- **Connect/Disconnect** - Lifecycle management with automatic reconnection
- **Pub/Sub Messaging** - Subscribe to subjects, publish messages, request/reply
- **LLM Completions** - Blocking and streaming completions via Athyr backends
- **Memory Sessions** - Conversation context with automatic summarization
- **KV Storage** - Key-value buckets for agent state
- **Typed Handlers** - Generic `Handler[Req, Resp]` with automatic JSON marshaling

### Orchestration Patterns

Located in `pkg/orchestration/` package:

| Pattern | Description |
|---------|-------------|
| **Pipeline** | Sequential agent chain (A → B → C) |
| **FanOut** | Parallel execution with aggregation |
| **Handoff** | Dynamic routing via triage agent |
| **GroupChat** | Multi-agent collaborative discussion |

### Middleware

- `Logger` - Request/response logging
- `Recover` - Panic recovery
- `Timeout` - Request timeouts
- `Retry` - Automatic retries with backoff
- `RateLimit` - Concurrency limiting
- `Metrics` - Duration/error callbacks
- `Validate` - Input validation
- `Chain` - Compose multiple middleware

## Examples

See [`examples/`](examples/) for complete working examples:

| Example | Demonstrates |
|---------|--------------|
| [`echo-agent`](examples/echo-agent/) | Basic pub/sub messaging |
| [`chat-agent`](examples/chat-agent/) | Sessions, KV, streaming |
| [`resilient-agent`](examples/resilient-agent/) | StreamError handling, retries |
| [`blog-pipeline`](examples/blog-pipeline/) | Pipeline orchestration pattern |

## Memory Sessions

Use sessions for conversation memory with automatic context injection:

```go
// Create session with agent personality
session, _ := agent.CreateSession(ctx, athyr.DefaultSessionProfile(),
    "You are a helpful coding assistant.")

// Completions with session automatically include memory context
resp, _ := agent.Complete(ctx, athyr.CompletionRequest{
    Model:         "gpt-4",
    Messages:      []athyr.Message{{Role: "user", Content: "How do I sort a list?"}},
    SessionID:     session.ID,
    IncludeMemory: true,
})

// Add persistent hints that survive message trimming
agent.AddHint(ctx, session.ID, "User prefers examples with type hints")
```

The orchestrator automatically manages:
- **Context injection** - System prompt, summary, hints, and message history
- **Rolling window** - Keeps recent messages within token limit
- **Summarization** - Compresses old messages when threshold reached

## Server Pattern

For building agent services that handle requests:

```go
import github.com/athyr-tech/athyr-sdk-go/pkg/athyr

func main() {
    server := athyr.NewServer("localhost:9090",
        athyr.WithAgentName("my-service"),
    )

    // Typed handler with automatic JSON marshaling
    athyr.Handle(server, "echo.request", func(ctx athyr.Context, req EchoRequest) (EchoResponse, error) {
        return EchoResponse{Echo: req.Message}, nil
    })

    server.Run(context.Background())
}
```

## Requirements

- Go 1.21+
- Running [Athyr server](https://github.com/athyr-tech/athyr)
- (Optional) Ollama or other LLM backend

## License

MIT - See [LICENSE](LICENSE) for details.

## Related

- [Athyr Server](https://github.com/athyr-tech/athyr) - The platform this SDK connects to
- [Athyr SDK Python](https://github.com/athyr-tech/athyr-sdk-python) - Python SDK (coming soon)
