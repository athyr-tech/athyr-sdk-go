// Edit Agent - Third stage: improves blog posts for clarity and engagement.
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

const agentName = "edit-agent"

func main() {
	flag.Parse()

	err := run.UntilSignal(func(ctx context.Context) error {
		server := athyr.NewServer(*athyrAddr,
			athyr.WithAgentName(agentName),
			athyr.WithAgentDescription("Improves blog posts for clarity"),
			athyr.WithVersion("1.0.0"),
		)
		athyr.Handle(server, types.SubjectEdit, handler(*model))

		run.Log(agentName, "listening on %s", types.SubjectEdit)
		return server.Run(ctx)
	})

	if err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func handler(model string) athyr.Handler[types.PipelineData, types.PipelineData] {
	return func(ctx athyr.Context, data types.PipelineData) (types.PipelineData, error) {
		run.Log(agentName, "input: %s", run.Truncate(data.Draft, 200))

		start := time.Now()
		resp, err := ctx.Agent().Complete(ctx, athyr.CompletionRequest{
			Model: model,
			Messages: []athyr.Message{
				{Role: "system", Content: systemPrompt},
				{Role: "user", Content: fmt.Sprintf("Edit and improve this blog post:\n\n%s", data.Draft)},
			},
		})
		if err != nil {
			return data, athyr.Unavailable("edit failed: %v", err)
		}

		data.Edited = resp.Content
		data.TotalTokens += resp.Usage.TotalTokens

		run.Log(agentName, "output: %s", run.Truncate(data.Edited, 200))
		run.Log(agentName, "done (%d tokens, %v)", resp.Usage.TotalTokens, time.Since(start).Round(time.Millisecond))

		return data, nil
	}
}

const systemPrompt = `You are an editor. Improve blog posts for clarity and engagement.
- Fix grammar and spelling
- Improve sentence flow
- Enhance readability
- Strengthen intro and conclusion

Return only the edited post in markdown format.`