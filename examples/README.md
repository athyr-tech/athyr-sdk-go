# Athyr SDK Examples

Example agents demonstrating the Athyr SDK.

## Prerequisites

1. Start the Athyr server (from the athyr repo):
   ```bash
   go run ./cmd/athyr serve
   ```

2. (Optional) Start Ollama for LLM features:
   ```bash
   ollama serve
   ollama pull llama3
   ```

## Examples

### Blog Pipeline (Orchestration)

Demonstrates the **Pipeline orchestration pattern** with a blog post creation workflow:
- Sequential stage execution (Outline → Draft → Edit → SEO)
- Typed handlers with automatic JSON marshaling
- Execution tracing

```bash
go run ./examples/blog-pipeline --topic "AI in Healthcare"
```

See [blog-pipeline/README.md](blog-pipeline/README.md) for details.

### Chat Agent

Interactive conversational assistant demonstrating:
- Agent registration and lifecycle
- Memory sessions with context injection
- LLM completions (blocking and streaming)
- KV storage for user preferences
- Memory hints

```bash
go run ./examples/chat-agent
```

**Commands:**
- `/model <name>` - Change LLM model
- `/hint <text>` - Add memory hint
- `/session` - Show session info
- `/stream` - Toggle streaming mode
- `/quit` - Exit

### Echo Agent

Minimal agent demonstrating pub/sub messaging:
- Subscribe to subjects
- Echo messages with prefix
- Request/reply pattern

```bash
go run ./examples/echo-agent
```

Test with HTTP API:
```bash
# Publish a message
curl -X POST http://localhost:8080/v1/publish \
  -H "Content-Type: application/json" \
  -d '{"subject": "echo.test", "data": "SGVsbG8gV29ybGQ="}'
```

### Resilient Agent

Demonstrates graceful streaming error handling with `StreamError`:
- Detect partial responses when streams fail mid-response
- Automatic retry with same model
- Fallback to backup model
- Final fallback to blocking mode
- Good UX during failures

```bash
go run ./examples/resilient-agent
```

This example shows the recommended pattern for handling `StreamError`:

```go
err := agent.CompleteStream(ctx, req, handler)
if err != nil {
    var streamErr *sdk.StreamError
    if errors.As(err, &streamErr) {
        if streamErr.PartialResponse {
            // Some content was shown - decide recovery strategy
            fmt.Printf("Got %d chars before failure\n",
                len(streamErr.AccumulatedContent))
        }
        // Retry, switch model, or fall back to blocking
    }
}
```

## SDK Quick Start

```go
package main

import (
    "context"
    sdk "github.com/athyr-tech/athyr-sdk-go"
)

func main() {
    // Create agent
    agent, _ := sdk.NewAgent("localhost:9090",
        sdk.WithAgentCard(sdk.AgentCard{
            Name: "my-agent",
            Capabilities: []string{"chat"},
        }),
    )

    // Connect
    ctx := context.Background()
    agent.Connect(ctx)
    defer agent.Close()

    // Use LLM
    resp, _ := agent.Complete(ctx, sdk.CompletionRequest{
        Model:    "llama3",
        Messages: []sdk.Message{{Role: "user", Content: "Hello!"}},
    })
    println(resp.Content)

    // Use KV storage
    kv := agent.KV("my-bucket")
    kv.Put(ctx, "key", []byte("value"))

    // Use memory sessions
    session, _ := agent.CreateSession(ctx, sdk.DefaultSessionProfile())
    agent.AddHint(ctx, session.ID, "User prefers concise answers")
}
```

## Running Tests

```bash
# Unit tests
go test ./...

# Integration tests (requires no running server)
go test ./internal/server/... -v
```
