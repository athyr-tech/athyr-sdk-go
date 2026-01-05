# Blog Pipeline Example

Demonstrates a **distributed multi-agent pipeline** for blog post creation using Athyr. Each pipeline stage runs as an independent agent that can be deployed separately, scaled horizontally, or run on different machines.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Distributed Pipeline                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐     │
│  │ Outline  │   │  Draft   │   │   Edit   │   │   SEO    │     │
│  │  Agent   │   │  Agent   │   │  Agent   │   │  Agent   │     │
│  │    📋    │   │    ✍️    │   │    🔍    │   │    🔎    │     │
│  └────┬─────┘   └────┬─────┘   └────┬─────┘   └────┬─────┘     │
│       │              │              │              │            │
│       └──────────────┴──────────────┴──────────────┘            │
│                           │                                     │
│                    ┌──────┴──────┐                              │
│                    │    Athyr    │                              │
│                    │   Broker    │                              │
│                    └──────┬──────┘                              │
│                           │                                     │
│                    ┌──────┴──────┐                              │
│                    │Orchestrator │                              │
│                    │     🎯      │                              │
│                    └─────────────┘                              │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

Each agent:
1. Connects to Athyr and subscribes to its subject
2. Receives `PipelineData` requests from the orchestrator
3. Calls an LLM via Athyr's `Complete()` API
4. Returns the enriched data

## File Structure

```
blog-pipeline/
├── cmd/
│   ├── outline-agent/   # Stage 1: Creates blog outline from topic
│   ├── draft-agent/     # Stage 2: Writes full draft from outline
│   ├── edit-agent/      # Stage 3: Improves draft for clarity
│   ├── seo-agent/       # Stage 4: Optimizes for search engines
│   └── orchestrator/    # Pipeline controller
├── internal/
│   └── types/           # Shared data structures
├── Dockerfile           # Multi-stage build for any agent
├── docker-compose.yml   # Runs all agents as containers
└── README.md
```

## Quick Start

### Option 1: Run Locally (Development)

```bash
# Terminal 1-4: Start each agent (from examples/blog-pipeline)
go run ./cmd/outline-agent
go run ./cmd/draft-agent
go run ./cmd/edit-agent
go run ./cmd/seo-agent

# Terminal 5: Run the orchestrator
go run ./cmd/orchestrator --topic "AI in Healthcare"
```

### Option 2: Docker Compose (Production-like)

```bash
cd examples/blog-pipeline

# Build all images
docker compose build

# Start all agents (background)
docker compose up -d outline-agent draft-agent edit-agent seo-agent

# Run the orchestrator
docker compose run --rm orchestrator --topic "AI in Healthcare"
```

## Configuration

### Agent Flags

All agents accept:

| Flag | Default | Description |
|------|---------|-------------|
| `--athyr` | `localhost:9090` | Athyr server address |
| `--model` | `qwen3:4b` | LLM model to use |

### Orchestrator Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--athyr` | `localhost:9090` | Athyr server address |
| `--topic` | (required) | Topic for the blog post |
| `--output` | `blog-post.md` | Output file path |

### Environment Variables (Docker)

| Variable | Default | Description |
|----------|---------|-------------|
| `ATHYR_ADDR` | `host.docker.internal:9090` | Athyr server address |
| `MODEL` | `qwen3:4b` | LLM model to use |

## Key Concepts

### Typed Service Handlers

Each agent uses the SDK's `Server` abstraction with typed handlers:

```go
import (
    "github.com/athyr-tech/athyr-sdk-go/examples/blog-pipeline/internal/types"
    "github.com/athyr-tech/athyr-sdk-go/pkg/athyr"
)

func main() {
    server := athyr.NewServer(*athyrAddr,
        athyr.WithAgentName("outline-agent"),
        athyr.WithAgentDescription("Creates blog post outlines"),
    )
    athyr.Handle(server, types.SubjectOutline, handler(*model))
    server.Run(ctx)
}

func handler(model string) athyr.Handler[types.PipelineData, types.PipelineData] {
    return func(ctx athyr.Context, data types.PipelineData) (types.PipelineData, error) {
        resp, err := ctx.Agent().Complete(ctx, athyr.CompletionRequest{...})
        data.Outline = resp.Content
        return data, nil
    }
}
```

### Pipeline Orchestration

The orchestrator uses the `orchestration.Pipeline` pattern:

```go
pipeline := orchestration.NewPipeline("blog-creation").
    Step("outline", types.SubjectOutline).
    Step("draft", types.SubjectDraft).
    Step("edit", types.SubjectEdit).
    Step("seo", types.SubjectSEO)

trace, err := pipeline.ExecuteWithTrace(ctx, agent, inputBytes)
```

### Shared Types

Data flows through the pipeline via `PipelineData`:

```go
type PipelineData struct {
    Topic       string  // Input: user-provided topic
    Outline     string  // Stage 1 output
    Draft       string  // Stage 2 output
    Edited      string  // Stage 3 output
    Final       string  // Stage 4 output
    TotalTokens int     // Cumulative token usage
}
```

## Scaling

### Horizontal Scaling

Run multiple instances of any agent to handle more load:

```bash
# Scale draft agent (heaviest workload) to 3 instances
docker compose up -d --scale draft-agent=3

# Agents automatically load-balance via queue groups
```

### Separate Deployments

Deploy agents on different machines:

```bash
# Machine A: Outline and Draft agents
outline-agent --athyr athyr.example.com:9090
draft-agent --athyr athyr.example.com:9090

# Machine B: Edit and SEO agents
edit-agent --athyr athyr.example.com:9090
seo-agent --athyr athyr.example.com:9090

# Machine C: Orchestrator
orchestrator --athyr athyr.example.com:9090 --topic "..."
```

## Example Output

```
╔══════════════════════════════════════════════════╗
║      Blog Pipeline Orchestrator                  ║
║                                                  ║
║   📋 Outline → ✍️ Draft → 🔍 Edit → 🔎 SEO       ║
╚══════════════════════════════════════════════════╝

⚡ Connecting to Athyr at localhost:9090... ✓ (Agent: agent-xyz123)

📝 Topic: AI in Healthcare

─────────────────────────────────────────

✅ Pipeline Complete!

   ✓ outline    1.2s
   ✓ draft      4.5s
   ✓ edit       5.1s
   ✓ seo        5.8s

   Total time:   16.6s
   Total tokens: ~3714
   Output saved: blog-post.md
```

## Adding New Stages

1. Create a new agent in `cmd/my-agent/main.go`
2. Add the subject to `internal/types/types.go`
3. Add the service to `docker-compose.yml`
4. Add the step to the orchestrator's pipeline

## Related Patterns

The SDK includes other orchestration patterns:

- **FanOut**: Run multiple agents in parallel
- **HandoffRouter**: Route tasks to specialized agents
- **GroupChat**: Collaborative multi-agent discussions

See the `orchestration` package for more patterns.
