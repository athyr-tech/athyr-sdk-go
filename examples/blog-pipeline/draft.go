package main

import (
	"fmt"
	"time"

	"github.com/athyr-tech/athyr-sdk-go/pkg/athyr"
)

// DraftStage writes full blog posts from outlines.
// This is the second stage - it receives the topic and outline,
// then produces a complete first draft.

// DraftHandler returns a typed handler for the draft stage.
func DraftHandler(model string) athyr.Handler[PipelineData, PipelineData] {
	return func(ctx athyr.Context, data PipelineData) (PipelineData, error) {
		fmt.Printf("\n✍️  Stage: DRAFT\n")
		fmt.Printf("   Writing from outline...\n")

		// Build the user prompt with topic and outline context
		userPrompt := fmt.Sprintf(`Topic: %s

Outline:
%s

Write a full blog post based on this outline.`, data.Topic, data.Outline)

		// Call the LLM via Athyr
		startTime := time.Now()
		resp, err := ctx.Agent().Complete(ctx, athyr.CompletionRequest{
			Model: model,
			Messages: []athyr.Message{
				{Role: "system", Content: draftSystemPrompt},
				{Role: "user", Content: userPrompt},
			},
		})
		duration := time.Since(startTime)

		if err != nil {
			fmt.Printf("   ✗ Error: %v\n", err)
			return data, athyr.Unavailable("draft generation failed: %v", err)
		}

		fmt.Printf("   ✓ Generated (%d tokens, %v)\n", resp.Usage.TotalTokens, duration.Round(time.Millisecond))

		// Store the draft in pipeline data
		data.Draft = resp.Content
		data.TotalTokens += resp.Usage.TotalTokens

		return data, nil
	}
}

// draftSystemPrompt instructs the LLM on how to write blog posts.
const draftSystemPrompt = `You are an expert blog writer. Write engaging, informative blog posts.

Guidelines:
- Use a conversational but professional tone
- Include relevant examples and explanations
- Keep paragraphs short and scannable
- Use headings and subheadings from the outline
- Aim for 800-1200 words

Write in markdown format.`
