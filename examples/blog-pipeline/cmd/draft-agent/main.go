// Draft Agent - Writes full blog posts from outlines.
//
// This is the second stage in the blog pipeline. It receives the topic
// and outline, then produces a complete first draft.
//
// Usage:
//
//	draft-agent --athyr=localhost:9090 --model=qwen3:4b
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
	athyrAddr = flag.String("athyr", types.DefaultAthyrAddr, "Athyr server address")
	model     = flag.String("model", types.DefaultModel, "LLM model to use")
)

func main() {
	flag.Parse()

	fmt.Println("✍️  Draft Agent")
	fmt.Printf("   Athyr: %s\n", *athyrAddr)
	fmt.Printf("   Model: %s\n", *model)
	fmt.Printf("   Subject: %s\n", types.SubjectDraft)
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
		athyr.WithAgentName("draft-agent"),
		athyr.WithAgentDescription("Writes full blog posts from outlines"),
		athyr.WithVersion("1.0.0"),
	)
	athyr.Handle(server, types.SubjectDraft, handler(*model))

	fmt.Println("✅ Starting service...")
	if err := server.Run(ctx); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func handler(model string) athyr.Handler[types.PipelineData, types.PipelineData] {
	return func(ctx athyr.Context, data types.PipelineData) (types.PipelineData, error) {
		fmt.Printf("✍️  Writing draft for: %q\n", data.Topic)

		userPrompt := fmt.Sprintf(`Topic: %s

Outline:
%s

Write a full blog post based on this outline.`, data.Topic, data.Outline)

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
			return data, athyr.Unavailable("draft generation failed: %v", err)
		}

		fmt.Printf("   ✓ Done (%d tokens, %v)\n", resp.Usage.TotalTokens, duration.Round(time.Millisecond))

		data.Draft = resp.Content
		data.TotalTokens += resp.Usage.TotalTokens

		return data, nil
	}
}

const systemPrompt = `You are an expert blog writer. Write engaging, informative blog posts.

Guidelines:
- Use a conversational but professional tone
- Include relevant examples and explanations
- Keep paragraphs short and scannable
- Use headings and subheadings from the outline
- Aim for 800-1200 words

Write in markdown format.`
