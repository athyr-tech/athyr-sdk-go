package main

import (
	"fmt"
	"time"

	sdk "github.com/athyr-tech/athyr-sdk-go"
)

// OutlineStage creates blog post outlines from topics.
// This is the first stage in the pipeline - it receives the raw topic
// and produces a structured outline for subsequent stages.

// OutlineHandler returns a typed handler for the outline stage.
// It demonstrates the Handler[Req, Resp] pattern from the SDK.
func OutlineHandler(model string) sdk.Handler[PipelineData, PipelineData] {
	return func(ctx sdk.Context, data PipelineData) (PipelineData, error) {
		fmt.Printf("\n📋 Stage: OUTLINE\n")
		fmt.Printf("   Processing topic: %q\n", data.Topic)

		// Build the user prompt for outline generation
		userPrompt := fmt.Sprintf("Create a blog post outline for the topic: %s", data.Topic)

		// Call the LLM via Athyr
		startTime := time.Now()
		resp, err := ctx.Agent().Complete(ctx, sdk.CompletionRequest{
			Model: model,
			Messages: []sdk.Message{
				{Role: "system", Content: outlineSystemPrompt},
				{Role: "user", Content: userPrompt},
			},
		})
		duration := time.Since(startTime)

		if err != nil {
			fmt.Printf("   ✗ Error: %v\n", err)
			return data, sdk.Unavailable("outline generation failed: %v", err)
		}

		fmt.Printf("   ✓ Generated (%d tokens, %v)\n", resp.Usage.TotalTokens, duration.Round(time.Millisecond))

		// Store the outline in pipeline data
		data.Outline = resp.Content
		data.TotalTokens += resp.Usage.TotalTokens

		return data, nil
	}
}

// outlineSystemPrompt instructs the LLM on how to create outlines.
const outlineSystemPrompt = `You are an expert content strategist. Create well-structured blog post outlines.

Your outline should include:
- A compelling title
- An introduction hook
- 3-5 main sections with subpoints
- A conclusion section
- A call-to-action

Be concise but comprehensive. Format as markdown.`
