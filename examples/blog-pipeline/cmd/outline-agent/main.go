// Outline Agent - First stage: creates blog post outlines from topics.
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

const agentName = "outline-agent"

func main() {
	flag.Parse()

	err := run.UntilSignal(func(ctx context.Context) error {
		server := athyr.NewServer(*athyrAddr,
			athyr.WithAgentName(agentName),
			athyr.WithAgentDescription("Creates blog post outlines"),
			athyr.WithVersion("1.0.0"),
		)
		athyr.Handle(server, types.SubjectOutline, handler(*model))

		run.Log(agentName, "listening on %s", types.SubjectOutline)
		return server.Run(ctx)
	})

	if err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func handler(model string) athyr.Handler[types.PipelineData, types.PipelineData] {
	return func(ctx athyr.Context, data types.PipelineData) (types.PipelineData, error) {
		run.Log(agentName, "input: %s", run.Truncate(data.Topic, 100))

		start := time.Now()
		resp, err := ctx.Agent().Complete(ctx, athyr.CompletionRequest{
			Model: model,
			Messages: []athyr.Message{
				{Role: "system", Content: systemPrompt},
				{Role: "user", Content: fmt.Sprintf("Create a blog post outline for: %s", data.Topic)},
			},
		})
		if err != nil {
			return data, athyr.Unavailable("outline generation failed: %v", err)
		}

		data.Outline = resp.Content
		data.TotalTokens += resp.Usage.TotalTokens

		run.Log(agentName, "output: %s", run.Truncate(data.Outline, 200))
		run.Log(agentName, "done (%d tokens, %v)", resp.Usage.TotalTokens, time.Since(start).Round(time.Millisecond))

		return data, nil
	}
}

const systemPrompt = `You are a content strategist. Create concise blog post outlines with:
- A title
- Introduction hook
- 3-5 main sections with subpoints
- Conclusion and call-to-action

Format as markdown.`