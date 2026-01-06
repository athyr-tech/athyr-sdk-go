// Edit Agent - Improves blog posts for clarity and engagement.
//
// This is the third stage in the blog pipeline. It receives the draft
// and produces a polished, edited version.
//
// Usage:
//
//	edit-agent --athyr=localhost:9090 --model=qwen3:4b
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/athyr-tech/athyr-sdk-go/examples/blog-pipeline/internal/types"
	"github.com/athyr-tech/athyr-sdk-go/pkg/athyr"
)

var (
	athyrAddr = flag.String("athyr", types.DefaultAthyrAddr(), "Athyr server address")
	model     = flag.String("model", types.DefaultModel(), "LLM model to use")
)

func main() {
	flag.Parse()

	fmt.Println("🔍 Edit Agent")
	fmt.Printf("   Athyr: %s\n", *athyrAddr)
	fmt.Printf("   Model: %s\n", *model)
	fmt.Printf("   Subject: %s\n", types.SubjectEdit)
	fmt.Println()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nShutting down...")
		cancel()
	}()

	server := athyr.NewServer(*athyrAddr,
		athyr.WithAgentName("edit-agent"),
		athyr.WithAgentDescription("Improves blog posts for clarity and engagement"),
		athyr.WithVersion("1.0.0"),
	)
	athyr.Handle(server, types.SubjectEdit, handler(*model))

	fmt.Println("✅ Starting service...")
	if err := server.Run(ctx); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func handler(model string) athyr.Handler[types.PipelineData, types.PipelineData] {
	return func(ctx athyr.Context, data types.PipelineData) (types.PipelineData, error) {
		fmt.Printf("🔍 Editing draft for: %q\n", data.Topic)

		userPrompt := fmt.Sprintf(`Edit and improve this blog post for clarity, flow, and engagement:

%s`, data.Draft)

		startTime := time.Now()
		resp, err := ctx.Agent().Complete(ctx, athyr.CompletionRequest{
			Model: model,
			Messages: []athyr.Message{
				{Role: "system", Content: systemPrompt},
				{Role: "user", Content: userPrompt},
			},
		})
		duration := time.Since(startTime)

		if err != nil {
			fmt.Printf("   ✗ Error: %v\n", err)
			return data, athyr.Unavailable("edit failed: %v", err)
		}

		fmt.Printf("   ✓ Done (%d tokens, %v)\n", resp.Usage.TotalTokens, duration.Round(time.Millisecond))

		data.Edited = resp.Content
		data.TotalTokens += resp.Usage.TotalTokens

		return data, nil
	}
}

const systemPrompt = `You are an expert editor. Improve blog posts for clarity and engagement.

Focus on:
- Fixing grammar and spelling
- Improving sentence flow
- Enhancing readability
- Strengthening the introduction and conclusion
- Ensuring consistent tone

Return the improved version in markdown format. Do not add commentary, just return the edited post.`
