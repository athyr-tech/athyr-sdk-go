# Blog Pipeline Example

Demonstrates the **Pipeline orchestration pattern** with a blog post creation workflow using Athyr.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Pipeline Flow                         │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│   Topic ──► Outline ──► Draft ──► Edit ──► SEO ──► Output   │
│              📋          ✍️        🔍       🔎               │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

Each stage is an independent handler that:
1. Receives `PipelineData` from the previous stage
2. Calls an LLM via Athyr's `Complete()` API
3. Adds its output to `PipelineData` and passes it forward

## File Structure

```
blog-pipeline/
├── main.go      # CLI, orchestration, and pipeline execution
├── types.go     # PipelineData struct and NATS subjects
├── outline.go   # Stage 1: Creates blog outline from topic
├── draft.go     # Stage 2: Writes full draft from outline
├── edit.go      # Stage 3: Improves draft for clarity
├── seo.go       # Stage 4: Optimizes for search engines
└── README.md    # This file
```

## Key Concepts

### Typed Handlers

Each stage uses the SDK's `Handler[Req, Resp]` pattern for automatic JSON marshaling:

```go
import "github.com/athyr/sdk"

func OutlineHandler(model string) sdk.Handler[PipelineData, PipelineData] {
    return func(ctx sdk.Context, data PipelineData) (PipelineData, error) {
        // Access the agent via ctx.Agent()
        resp, err := ctx.Agent().Complete(ctx, sdk.CompletionRequest{...})

        data.Outline = resp.Content
        return data, nil
    }
}
```

### Pipeline Construction

The pipeline is built using the fluent API from `orchestration.Pipeline`:

```go
pipeline := orchestration.NewPipeline("blog-creation").
    Step("outline", SubjectOutline).
    Step("draft", SubjectDraft).
    Step("edit", SubjectEdit).
    Step("seo", SubjectSEO)

trace, err := pipeline.ExecuteWithTrace(ctx, agent, inputBytes)
```

### Data Flow

`PipelineData` accumulates results as it flows through stages:

```go
type PipelineData struct {
    Topic       string  // Input: user-provided topic
    Outline     string  // Stage 1 output
    Draft       string  // Stage 2 output
    Edited      string  // Stage 3 output
    Final       string  // Stage 4 output (final result)
    TotalTokens int     // Cumulative token usage
}
```

## Usage

### Prerequisites

1. **Athyr server running** - The server should be accessible (default: `localhost:9090`)
2. **LLM provider configured** - Athyr should have access to an LLM (Ollama, OpenRouter, etc.)

### Running the Pipeline

```bash
# From the examples directory
cd examples/blog-pipeline

# Basic usage
go run . --topic "AI in Healthcare"

# With options
go run . \
    --topic "Building Microservices with Go" \
    --model "llama3:8b" \
    --output "my-post.md" \
    --athyr "localhost:9090"
```

### Options

| Flag | Default | Description |
|------|---------|-------------|
| `--topic` | (required) | Topic for the blog post |
| `--model` | `qwen3:4b` | LLM model to use |
| `--output` | `blog-post.md` | Output file path |
| `--athyr` | `localhost:9090` | Athyr server address |

## Example Output

```
╔══════════════════════════════════════════════════╗
║         Blog Pipeline - Orchestration Demo       ║
║                                                  ║
║   📋 Outline → ✍️ Draft → 🔍 Edit → 🔎 SEO       ║
╚══════════════════════════════════════════════════╝

⚡ Connecting to Athyr at localhost:9090... ✓ (Agent: agent-xyz123)
⚡ Registering pipeline stages... ✓

📝 Topic: AI in Healthcare
🤖 Model: qwen3:4b

─────────────────────────────────────────

📋 Stage: OUTLINE
   Processing topic: "AI in Healthcare"
   ✓ Generated (245 tokens, 1.2s)

✍️  Stage: DRAFT
   Writing from outline...
   ✓ Generated (1024 tokens, 4.5s)

🔍 Stage: EDIT
   Improving draft...
   ✓ Generated (1156 tokens, 5.1s)

🔎 Stage: SEO
   Optimizing for search...
   ✓ Generated (1289 tokens, 5.8s)

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

## Extending the Pipeline

### Adding a New Stage

1. Create a new file (e.g., `translate.go`):

```go
package main

import "github.com/athyr/sdk"

func TranslateHandler(model, targetLang string) sdk.Handler[PipelineData, PipelineData] {
    return func(ctx sdk.Context, data PipelineData) (PipelineData, error) {
        // Your translation logic here
        return data, nil
    }
}

const translateSystemPrompt = `You are a translator...`
```

2. Add the subject to `types.go`:

```go
const SubjectTranslate = "demo.blog.translate"
```

3. Register in `main.go`:

```go
stages := []struct {
    subject string
    handler sdk.Handler[PipelineData, PipelineData]
}{
    // ... existing stages ...
    {SubjectTranslate, TranslateHandler(*model, "Spanish")},
}
```

4. Add to the pipeline:

```go
pipeline := orchestration.NewPipeline("blog-creation").
    // ... existing steps ...
    Step("translate", SubjectTranslate)
```

## Related Patterns

The SDK includes other orchestration patterns:

- **FanOut**: Run multiple agents in parallel (e.g., analyze stock from multiple perspectives)
- **HandoffRouter**: Route tasks to specialized agents based on content
- **GroupChat**: Collaborative multi-agent discussions

See the `orchestration` package documentation for more patterns.
