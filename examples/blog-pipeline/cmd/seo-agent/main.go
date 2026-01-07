// SEO Agent - Final stage: optimizes blog posts for search engines.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/athyr-tech/athyr-sdk-go/examples/blog-pipeline/internal/run"
	"github.com/athyr-tech/athyr-sdk-go/examples/blog-pipeline/internal/types"
	"github.com/athyr-tech/athyr-sdk-go/pkg/athyr"
)

var (
	athyrAddr = flag.String("athyr", "localhost:9090", "Athyr server address")
	model     = flag.String("model", "qwen3:4b", "LLM model")
)

const agentName = "seo-agent"

func main() {
	flag.Parse()

	err := run.UntilSignal(func(ctx context.Context) error {
		server := athyr.NewServer(*athyrAddr,
			athyr.WithAgentName(agentName),
			athyr.WithAgentDescription("Optimizes blog posts for SEO"),
			athyr.WithVersion("1.0.0"),
		)
		athyr.Handle(server, types.SubjectSEO, handler(*model))

		run.Log(agentName, "listening on %s", types.SubjectSEO)
		return server.Run(ctx)
	})

	if err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func handler(model string) athyr.Handler[types.PipelineData, types.PipelineData] {
	return func(ctx athyr.Context, data types.PipelineData) (types.PipelineData, error) {
		run.Log(agentName, "input: %s", run.Truncate(data.Edited, 200))

		start := time.Now()
		resp, err := ctx.Agent().Complete(ctx, athyr.CompletionRequest{
			Model: model,
			Messages: []athyr.Message{
				{Role: "system", Content: systemPrompt},
				{Role: "user", Content: fmt.Sprintf("Optimize this blog post for SEO:\n\n%s", data.Edited)},
			},
		})
		if err != nil {
			return data, athyr.Unavailable("SEO optimization failed: %v", err)
		}

		data.Final = resp.Content
		data.TotalTokens += resp.Usage.TotalTokens

		run.Log(agentName, "output: %s", run.Truncate(data.Final, 200))
		run.Log(agentName, "done (%d tokens, %v)", resp.Usage.TotalTokens, time.Since(start).Round(time.Millisecond))

		return data, nil
	}
}

const systemPrompt = `You are an SEO specialist. Optimize blog posts for search engines.

Add at the beginning:
- Meta description (150-160 chars)
- Keywords (5-7 terms)

Improve:
- Headings with target keywords
- Internal linking suggestions as [Link: topic]

Return the complete optimized post in markdown format.`
