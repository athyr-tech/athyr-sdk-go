package main

// PipelineData flows through all pipeline stages.
// Each stage reads the accumulated data and adds its contribution.
type PipelineData struct {
	// Topic is the user-provided blog topic (input)
	Topic string `json:"topic"`

	// Outline is produced by the outline stage
	Outline string `json:"outline,omitempty"`

	// Draft is produced by the draft stage
	Draft string `json:"draft,omitempty"`

	// Edited is produced by the edit stage
	Edited string `json:"edited,omitempty"`

	// Final is the SEO-optimized content (final output)
	Final string `json:"final,omitempty"`

	// TotalTokens tracks cumulative token usage across stages
	TotalTokens int `json:"total_tokens"`
}

// Stage subjects define the NATS subjects for each pipeline stage.
// These subjects are used for both subscription and routing.
const (
	SubjectOutline = "demo.blog.outline"
	SubjectDraft   = "demo.blog.draft"
	SubjectEdit    = "demo.blog.edit"
	SubjectSEO     = "demo.blog.seo"
)
