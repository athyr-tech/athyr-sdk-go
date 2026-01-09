# Example: Pipeline Pattern

Demonstrates multi-stage sequential processing with distributed agents.
Use case: Blog post generation through outline, draft, edit, and SEO stages.

```
Topic → [Outline] → [Draft] → [Edit] → [SEO] → Final Post
```

Each stage is handled by a separate agent, demonstrating how to build
distributed pipelines with the Athyr SDK.

## Quick Start

```bash
# Start all services
docker compose up -d

# Pull the LLM model (first time only)
docker compose run --rm model-pull

# Interact with the pipeline
docker compose attach orchestrator
```

Then type a topic:

```
Blog Pipeline
Type a topic to generate a blog post, or 'quit' to exit.

topic> why cats are cool
```

## Project Structure

```
pipeline/
├── internal/
│   ├── pipeline/       # Core pipeline orchestration logic
│   ├── types/          # Shared data structures
│   └── run/            # Signal handling utilities
├── cmd/
│   ├── orchestrator/   # Interactive CLI
│   ├── outline-agent/  # Stage 1: Creates outlines
│   ├── draft-agent/    # Stage 2: Writes drafts
│   ├── edit-agent/     # Stage 3: Edits for clarity
│   └── seo-agent/      # Stage 4: SEO optimization
├── docker-compose.yml
├── Dockerfile
└── athyr-docker.yaml
```

## How It Works

### Pipeline Package (`internal/pipeline/`)

The core orchestration logic:

```go
// Build the pipeline with four stages
pipe := orchestration.NewPipeline("blog-creation").
    Step("outline", types.SubjectOutline).
    Step("draft", types.SubjectDraft).
    Step("edit", types.SubjectEdit).
    Step("seo", types.SubjectSEO)

// Execute
trace, err := pipe.ExecuteWithTrace(ctx, agent, inputBytes)
```

### Agent Pattern (`cmd/*-agent/`)

Each agent follows the same pattern:

```go
server := athyr.NewServer(athyrAddr,
    athyr.WithAgentName("outline-agent"),
    athyr.WithAgentDescription("Creates blog post outlines"),
)
athyr.Handle(server, types.SubjectOutline, handler)
server.Run(ctx)
```

### Data Flow

All agents share the same data structure that accumulates through the pipeline:

```go
type PipelineData struct {
    Topic       string  // Input
    Outline     string  // Stage 1 output
    Draft       string  // Stage 2 output
    Edited      string  // Stage 3 output
    Final       string  // Stage 4 output
    TotalTokens int     // Cumulative tokens
}
```

> **Note:** A shared type works well when you control all agents in the pipeline.
> For pipelines with independent or third-party agents, the SDK supports
> `WithTransform()` to adapt between different input/output types at each stage.

## Running Locally (without Docker)

Start each component in separate terminals:

```bash
# Terminal 1: Start Athyr server
athyr serve

# Terminal 2: Start Ollama
ollama serve

# Terminal 3-6: Start agents
go run ./cmd/outline-agent --athyr=localhost:9090
go run ./cmd/draft-agent --athyr=localhost:9090
go run ./cmd/edit-agent --athyr=localhost:9090
go run ./cmd/seo-agent --athyr=localhost:9090

# Terminal 7: Run orchestrator
go run ./cmd/orchestrator --athyr=localhost:9090
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MODEL` | `smollm2:135m` | Ollama model to use |

### Command Line Flags

All agents support:
- `--athyr` - Athyr server address (default: `localhost:9090`)
- `--model` - LLM model to use (default: `qwen3:4b`)

## Architecture

```
┌─────────────┐     ┌─────────────────┐     ┌──────────────┐
│   Ollama    │◄────│      Athyr      │◄────│ Orchestrator │
│   (LLM)     │     │    (Message     │     │    (CLI)     │
└─────────────┘     │     Broker)     │     └──────────────┘
                    └────────┬────────┘
                             │
        ┌────────────────────┼────────────────────┐
        │                    │                    │
        ▼                    ▼                    ▼
┌───────────────┐  ┌───────────────┐  ┌───────────────┐
│ Outline Agent │  │  Draft Agent  │  │  Edit Agent   │ ...
└───────────────┘  └───────────────┘  └───────────────┘
```

The orchestrator sends requests through Athyr to each agent in sequence.
Each agent processes its input and returns the result for the next stage.