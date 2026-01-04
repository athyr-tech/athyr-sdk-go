package main

import (
	"fmt"
	"time"

	sdk "github.com/athyr-tech/athyr-sdk-go"
)

// EditStage improves blog posts for clarity and engagement.
// This is the third stage - it receives the draft and
// produces a polished, edited version.

// EditHandler returns a typed handler for the edit stage.
func EditHandler(model string) sdk.Handler[PipelineData, PipelineData] {
	return func(ctx sdk.Context, data PipelineData) (PipelineData, error) {
		fmt.Printf("\n🔍 Stage: EDIT\n")
		fmt.Printf("   Improving draft...\n")

		// Build the user prompt with the draft to edit
		userPrompt := fmt.Sprintf(`Edit and improve this blog post for clarity, flow, and engagement:

%s`, data.Draft)

		// Call the LLM via Athyr
		startTime := time.Now()
		resp, err := ctx.Agent().Complete(ctx, sdk.CompletionRequest{
			Model: model,
			Messages: []sdk.Message{
				{Role: "system", Content: editSystemPrompt},
				{Role: "user", Content: userPrompt},
			},
		})
		duration := time.Since(startTime)

		if err != nil {
			fmt.Printf("   ✗ Error: %v\n", err)
			return data, sdk.Unavailable("edit failed: %v", err)
		}

		fmt.Printf("   ✓ Generated (%d tokens, %v)\n", resp.Usage.TotalTokens, duration.Round(time.Millisecond))

		// Store the edited version in pipeline data
		data.Edited = resp.Content
		data.TotalTokens += resp.Usage.TotalTokens

		return data, nil
	}
}

// editSystemPrompt instructs the LLM on how to edit content.
const editSystemPrompt = `You are an expert editor. Improve blog posts for clarity and engagement.

Focus on:
- Fixing grammar and spelling
- Improving sentence flow
- Enhancing readability
- Strengthening the introduction and conclusion
- Ensuring consistent tone

Return the improved version in markdown format. Do not add commentary, just return the edited post.`
