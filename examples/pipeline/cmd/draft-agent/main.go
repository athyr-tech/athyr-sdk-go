// Draft Agent - Second stage: writes full blog posts from outlines.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/athyr-tech/athyr-sdk-go/examples/pipeline/internal/run"
	"github.com/athyr-tech/athyr-sdk-go/examples/pipeline/internal/types"
	"github.com/athyr-tech/athyr-sdk-go/pkg/athyr"
)

var (
	athyrAddr = flag.String("athyr", "localhost:9090", "Athyr server address")
	model     = flag.String("model", "qwen3:4b", "LLM model")
)

const agentName = "draft-agent"

func main() {
	flag.Parse()

	err := run.UntilSignal(func(ctx context.Context) error {
		server := athyr.NewServer(*athyrAddr,
			athyr.WithAgentName(agentName),
			athyr.WithAgentDescription("Writes full blog posts from outlines"),
			athyr.WithVersion("1.0.0"),
		)
		athyr.Handle(server, types.SubjectDraft, handler(*model))

		run.Log(agentName, "listening on %s", types.SubjectDraft)
		return server.Run(ctx)
	})

	if err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func handler(model string) athyr.Handler[types.PipelineData, types.PipelineData] {
	return func(ctx athyr.Context, data types.PipelineData) (types.PipelineData, error) {
		run.Log(agentName, "input: %s", run.Truncate(data.Outline, 200))

		start := time.Now()
		resp, err := ctx.Agent().Complete(ctx, athyr.CompletionRequest{
			Model: model,
			Messages: []athyr.Message{
				{Role: "system", Content: systemPrompt},
				{Role: "user", Content: fmt.Sprintf("Topic: %s\n\nOutline:\n%s\n\nWrite a full blog post.", data.Topic, data.Outline)},
			},
		})
		if err != nil {
			return data, athyr.Unavailable("draft generation failed: %v", err)
		}

		data.Draft = resp.Content
		data.TotalTokens += resp.Usage.TotalTokens

		run.Log(agentName, "output: %s", run.Truncate(data.Draft, 200))
		run.Log(agentName, "done (%d tokens, %v)", resp.Usage.TotalTokens, time.Since(start).Round(time.Millisecond))

		return data, nil
	}
}

const systemPrompt = `You are a blog writer. Write engaging, informative posts.
- Conversational but professional tone
- Short, scannable paragraphs
- Follow the provided outline
- Aim for 800-1200 words

Format as markdown.`