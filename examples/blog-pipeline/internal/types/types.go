// Package types defines shared data structures for the blog pipeline.
// All agents in the pipeline import this package for consistent data exchange.
package types

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

// Subject constants define the Athyr messaging subjects for each pipeline stage.
// Each agent subscribes to its subject and responds to requests.
const (
	SubjectOutline = "blog.pipeline.outline"
	SubjectDraft   = "blog.pipeline.draft"
	SubjectEdit    = "blog.pipeline.edit"
	SubjectSEO     = "blog.pipeline.seo"
)

// DefaultModel is the default LLM model used by agents.
const DefaultModel = "qwen3:4b"

// DefaultAthyrAddr is the default Athyr server address.
const DefaultAthyrAddr = "localhost:9090"
