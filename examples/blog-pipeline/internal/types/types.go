// Package types defines shared data structures for the blog pipeline.
// All agents in the pipeline import this package for consistent data exchange.
package types

import "os"

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

// DefaultModel returns the LLM model to use.
// Checks MODEL environment variable first, falls back to qwen3:4b.
func DefaultModel() string {
	if model := os.Getenv("MODEL"); model != "" {
		return model
	}
	return "qwen3:4b"
}

// DefaultAthyrAddr returns the Athyr server address.
// Checks ATHYR_ADDR environment variable first, falls back to localhost:9090.
func DefaultAthyrAddr() string {
	if addr := os.Getenv("ATHYR_ADDR"); addr != "" {
		return addr
	}
	return "localhost:9090"
}

// DefaultOutputPath returns the output file path.
// Checks OUTPUT_PATH environment variable first, falls back to blog-post.md.
func DefaultOutputPath() string {
	if path := os.Getenv("OUTPUT_PATH"); path != "" {
		return path
	}
	return "blog-post.md"
}
