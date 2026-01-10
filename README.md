<p align="center">
  <img src="assets/athyr-logo.svg" alt="Athyr" width="120">
</p>

<h1 align="center">Athyr SDK for Go</h1>

<p align="center">
  <strong>Build distributed AI agents with production-grade infrastructure</strong>
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/athyr-tech/athyr-sdk-go"><img src="https://pkg.go.dev/badge/github.com/athyr-tech/athyr-sdk-go.svg" alt="Go Reference"></a>
  <a href="https://goreportcard.com/report/github.com/athyr-tech/athyr-sdk-go"><img src="https://goreportcard.com/badge/github.com/athyr-tech/athyr-sdk-go" alt="Go Report Card"></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT"></a>
  <img src="assets/athyr-badge.svg" alt="Athyr">
</p>

<p align="center">
  <a href="#installation">Installation</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#features">Features</a> •
  <a href="#examples">Examples</a> •
  <a href="docs/">Documentation</a>
</p>

---

## What is Athyr?

[Athyr](https://github.com/athyr-tech/athyr) is a runtime for distributed AI agents. Unlike frameworks where agents are functions in a single process, Athyr treats agents as **independent services** that communicate through a central orchestrator.

This SDK provides the Go client for building agents that connect to the Athyr platform.

**Why Athyr?**
- **Platform-managed services** — LLM access, memory, storage handled by Athyr
- **Language agnostic** — Agents are services, write in any language
- **Independent scaling** — Scale each agent separately
- **Fault isolation** — One agent failure doesn't crash others

## Installation

```bash
go get github.com/athyr-tech/athyr-sdk-go
```

**Requirements:** Go 1.21+ and a running [Athyr server](https://github.com/athyr-tech/athyr)

## Quick Start

```go
package main

import (
    "context"
    "github.com/athyr-tech/athyr-sdk-go/pkg/athyr"
)

func main() {
    agent := athyr.MustConnect("localhost:9090",
        athyr.WithAgentCard(athyr.AgentCard{
            Name:        "my-agent",
            Description: "My first Athyr agent",
        }),
    )
    defer agent.Close()

    resp, _ := agent.Complete(context.Background(), athyr.CompletionRequest{
        Model:    "llama3",
        Messages: []athyr.Message{{Role: "user", Content: "Hello!"}},
    })
    println(resp.Content)
}
```

## Features

### Core Agent

- **Connect/Disconnect** — Lifecycle management with automatic reconnection
- **Pub/Sub Messaging** — Subscribe to subjects, publish messages, request/reply
- **LLM Completions** — Blocking and streaming completions via Athyr backends
- **Memory Sessions** — Conversation context with automatic summarization
- **KV Storage** — Key-value buckets for agent state
- **Tool Calling** — LLM function calling support

### Orchestration Patterns

Located in `pkg/orchestration/`:

| Pattern | Description |
|---------|-------------|
| **Pipeline** | Sequential agent chain (A → B → C) |
| **FanOut** | Parallel execution with aggregation |
| **Handoff** | Dynamic routing via triage agent |
| **GroupChat** | Multi-agent collaborative discussion |

### Middleware

- `Logger` — Request/response logging
- `Recover` — Panic recovery
- `Timeout` — Request timeouts
- `Retry` — Automatic retries with backoff
- `RateLimit` — Concurrency limiting
- `Metrics` — Duration/error callbacks
- `Validate` — Input validation

## Examples

See [`examples/`](examples/) for complete working examples:

| Example | Demonstrates |
|---------|--------------|
| [`quickstart`](examples/quickstart/) | Basic agent setup |
| [`pipeline`](examples/pipeline/) | Sequential orchestration pattern |
| [`fanout`](examples/fanout/) | Parallel execution with aggregation |
| [`group-chat`](examples/group-chat/) | Multi-agent collaboration |
| [`handoff-router`](examples/handoff-router/) | Dynamic routing via triage |
| [`resilience`](examples/resilience/) | Error handling and retries |
| [`tool-calling`](examples/tool-calling/) | LLM function calling |
| [`features-demo`](examples/features-demo/) | Comprehensive feature demo |

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

The server automatically manages:
- **Context injection** — System prompt, summary, hints, and message history
- **Rolling window** — Keeps recent messages within token limit
- **Summarization** — Compresses old messages when threshold reached

## Server Pattern

For building agent services that handle requests:

```go
func main() {
    server := athyr.NewServer("localhost:9090",
        athyr.WithAgentName("my-service"),
    )

    athyr.Handle(server, "echo.request", func(ctx athyr.Context, req EchoRequest) (EchoResponse, error) {
        return EchoResponse{Echo: req.Message}, nil
    })

    server.Run(context.Background())
}
```

## Documentation

- [Architecture Overview](docs/ARCHITECTURE.md) — Internal SDK design
- [Contributing Guide](CONTRIBUTING.md) — How to contribute
- [Changelog](CHANGELOG.md) — Release history
- [API Reference](https://pkg.go.dev/github.com/athyr-tech/athyr-sdk-go) — GoDoc

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT — See [LICENSE](LICENSE) for details.

## Related

- [Athyr Server](https://github.com/athyr-tech/athyr) — The platform this SDK connects to
