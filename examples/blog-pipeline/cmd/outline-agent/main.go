// Outline Agent - Creates blog post outlines from topics.
//
// This is the first stage in the blog pipeline. It receives a topic
// and produces a structured outline for subsequent stages.
//
// Usage:
//
//	outline-agent --athyr=localhost:9090 --model=qwen3:4b
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

	fmt.Println("📋 Outline Agent")
	fmt.Printf("   Athyr: %s\n", *athyrAddr)
	fmt.Printf("   Model: %s\n", *model)
	fmt.Printf("   Subject: %s\n", types.SubjectOutline)
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

	// Run the service - this connects, subscribes, and blocks
	server := athyr.NewServer(*athyrAddr,
		athyr.WithAgentName("outline-agent"),
		athyr.WithAgentDescription("Creates blog post outlines from topics"),
		athyr.WithVersion("1.0.0"),
	)
	athyr.Handle(server, types.SubjectOutline, handler(*model))

	fmt.Println("✅ Starting service...")
	if err := server.Run(ctx); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func handler(model string) athyr.Handler[types.PipelineData, types.PipelineData] {
	return func(ctx athyr.Context, data types.PipelineData) (types.PipelineData, error) {
		fmt.Printf("📋 Processing: %q\n", data.Topic)

		userPrompt := fmt.Sprintf("Create a blog post outline for the topic: %s", data.Topic)

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
			return data, athyr.Unavailable("outline generation failed: %v", err)
		}

		fmt.Printf("   ✓ Done (%d tokens, %v)\n", resp.Usage.TotalTokens, duration.Round(time.Millisecond))

		data.Outline = resp.Content
		data.TotalTokens += resp.Usage.TotalTokens

		return data, nil
	}
}

const systemPrompt = `You are an expert content strategist. Create well-structured blog post outlines.

Your outline should include:
- A compelling title
- An introduction hook
- 3-5 main sections with subpoints
- A conclusion section
- A call-to-action

Be concise but comprehensive. Format as markdown.`
