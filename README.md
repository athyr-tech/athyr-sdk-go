# Athyr SDK for Go

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
    sdk "github.com/athyr-tech/athyr-sdk-go"
)

func main() {
    // Create and connect agent
    agent, _ := sdk.NewAgent("localhost:9090",
        sdk.WithAgentCard(sdk.AgentCard{
            Name:         "my-agent",
            Description:  "My first Athyr agent",
            Capabilities: []string{"chat"},
        }),
    )

    ctx := context.Background()
    agent.Connect(ctx)
    defer agent.Close()

    // LLM completion
    resp, _ := agent.Complete(ctx, sdk.CompletionRequest{
        Model:    "llama3",
        Messages: []sdk.Message{{Role: "user", Content: "Hello!"}},
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

Located in `orchestration/` package:

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

## Server Pattern

For building agent services that handle requests:

```go
func main() {
    server := sdk.NewServer("localhost:9090",
        sdk.WithServerName("my-service"),
    )

    // Typed handler with automatic JSON marshaling
    sdk.Handle(server, "echo.request", func(ctx sdk.Context, req EchoRequest) (EchoResponse, error) {
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
