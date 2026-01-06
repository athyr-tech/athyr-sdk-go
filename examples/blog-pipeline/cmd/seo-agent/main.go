// SEO Agent - Optimizes blog posts for search engines.
//
// This is the final stage in the blog pipeline. It receives the edited
// post and produces the final SEO-optimized version.
//
// Usage:
//
//	seo-agent --athyr=localhost:9090 --model=qwen3:4b
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

	fmt.Println("🔎 SEO Agent")
	fmt.Printf("   Athyr: %s\n", *athyrAddr)
	fmt.Printf("   Model: %s\n", *model)
	fmt.Printf("   Subject: %s\n", types.SubjectSEO)
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
		athyr.WithAgentName("seo-agent"),
		athyr.WithAgentDescription("Optimizes blog posts for search engines"),
		athyr.WithVersion("1.0.0"),
	)
	athyr.Handle(server, types.SubjectSEO, handler(*model))

	fmt.Println("✅ Starting service...")
	if err := server.Run(ctx); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func handler(model string) athyr.Handler[types.PipelineData, types.PipelineData] {
	return func(ctx athyr.Context, data types.PipelineData) (types.PipelineData, error) {
		fmt.Printf("🔎 Optimizing SEO for: %q\n", data.Topic)

		userPrompt := fmt.Sprintf(`Optimize this blog post for SEO. Add a meta description, suggest keywords, and improve headings:

%s`, data.Edited)

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
			return data, athyr.Unavailable("SEO optimization failed: %v", err)
		}

		fmt.Printf("   ✓ Done (%d tokens, %v)\n", resp.Usage.TotalTokens, duration.Round(time.Millisecond))

		data.Final = resp.Content
		data.TotalTokens += resp.Usage.TotalTokens

		return data, nil
	}
}

const systemPrompt = `You are an SEO specialist. Optimize blog posts for search engines.

Add to the beginning of the post:
- Meta description (150-160 chars)
- Keywords (5-7 relevant terms)

Improvements to make:
- Ensure headings use target keywords naturally
- Add internal linking suggestions as [Link: topic]
- Optimize the title for search

Return the complete optimized post in markdown format.`
