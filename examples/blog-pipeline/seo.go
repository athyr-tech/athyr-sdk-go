package main

import (
	"fmt"
	"time"

	sdk "github.com/athyr-tech/athyr-sdk-go/pkg/agent"
)

// SEOStage optimizes blog posts for search engines.
// This is the final stage - it receives the edited post and
// produces the final SEO-optimized version.

// SEOHandler returns a typed handler for the SEO stage.
func SEOHandler(model string) sdk.Handler[PipelineData, PipelineData] {
	return func(ctx sdk.Context, data PipelineData) (PipelineData, error) {
		fmt.Printf("\n🔎 Stage: SEO\n")
		fmt.Printf("   Optimizing for search...\n")

		// Build the user prompt with the edited content
		userPrompt := fmt.Sprintf(`Optimize this blog post for SEO. Add a meta description, suggest keywords, and improve headings:

%s`, data.Edited)

		// Call the LLM via Athyr
		startTime := time.Now()
		resp, err := ctx.Agent().Complete(ctx, sdk.CompletionRequest{
			Model: model,
			Messages: []sdk.Message{
				{Role: "system", Content: seoSystemPrompt},
				{Role: "user", Content: userPrompt},
			},
		})
		duration := time.Since(startTime)

		if err != nil {
			fmt.Printf("   ✗ Error: %v\n", err)
			return data, sdk.Unavailable("SEO optimization failed: %v", err)
		}

		fmt.Printf("   ✓ Generated (%d tokens, %v)\n", resp.Usage.TotalTokens, duration.Round(time.Millisecond))

		// Store the final output in pipeline data
		data.Final = resp.Content
		data.TotalTokens += resp.Usage.TotalTokens

		return data, nil
	}
}

// seoSystemPrompt instructs the LLM on SEO optimization.
const seoSystemPrompt = `You are an SEO specialist. Optimize blog posts for search engines.

Add to the beginning of the post:
- Meta description (150-160 chars)
- Keywords (5-7 relevant terms)

Improvements to make:
- Ensure headings use target keywords naturally
- Add internal linking suggestions as [Link: topic]
- Optimize the title for search

Return the complete optimized post in markdown format.`
