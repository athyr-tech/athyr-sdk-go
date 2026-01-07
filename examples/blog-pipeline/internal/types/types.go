// Package types defines shared data structures for the blog pipeline.
package types

// PipelineData flows through all pipeline stages.
type PipelineData struct {
	Topic       string `json:"topic"`
	Outline     string `json:"outline,omitempty"`
	Draft       string `json:"draft,omitempty"`
	Edited      string `json:"edited,omitempty"`
	Final       string `json:"final,omitempty"`
	TotalTokens int    `json:"total_tokens"`
}

// Subjects for each pipeline stage.
const (
	SubjectOutline = "blog.pipeline.outline"
	SubjectDraft   = "blog.pipeline.draft"
	SubjectEdit    = "blog.pipeline.edit"
	SubjectSEO     = "blog.pipeline.seo"
)
