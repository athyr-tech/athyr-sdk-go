// Package pipeline provides the blog creation pipeline logic.
//
// This package demonstrates how to build and execute a multi-stage
// pipeline using the Athyr SDK's orchestration package.
//
// Pipeline flow:
//
//	Topic → [Outline] → [Draft] → [Edit] → [SEO] → Final Post
package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/athyr-tech/athyr-sdk-go/examples/blog-pipeline/internal/types"
	"github.com/athyr-tech/athyr-sdk-go/pkg/athyr"
	"github.com/athyr-tech/athyr-sdk-go/pkg/orchestration"
)

// StepResult holds the outcome of a single pipeline step.
type StepResult struct {
	Name     string
	Duration time.Duration
	Output   string
}

// Result holds the complete pipeline execution result.
type Result struct {
	Steps       []StepResult
	Final       string
	TotalTokens int
	Duration    time.Duration
}

// Run executes the blog creation pipeline for the given topic.
func Run(ctx context.Context, athyrAddr, topic string) (*Result, error) {
	// 1. Connect to Athyr
	agent, err := athyr.NewAgent(athyrAddr,
		athyr.WithAgentCard(athyr.AgentCard{
			Name:        "blog-orchestrator",
			Description: "Coordinates the blog creation pipeline",
			Version:     "1.0.0",
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("create agent: %w", err)
	}

	if err := agent.Connect(ctx); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	defer agent.Close()

	// 2. Build the pipeline with four stages
	pipe := orchestration.NewPipeline("blog-creation").
		Step("outline", types.SubjectOutline).
		Step("draft", types.SubjectDraft).
		Step("edit", types.SubjectEdit).
		Step("seo", types.SubjectSEO)

	// 3. Execute pipeline
	input := types.PipelineData{Topic: topic}
	inputBytes, _ := json.Marshal(input)

	startTime := time.Now()
	trace, err := pipe.ExecuteWithTrace(ctx, agent, inputBytes)
	if err != nil {
		return nil, fmt.Errorf("execute: %w", err)
	}

	// 4. Parse output
	var data types.PipelineData
	if err := json.Unmarshal(trace.Output(), &data); err != nil {
		return nil, fmt.Errorf("parse output: %w", err)
	}

	// 5. Build result
	result := &Result{
		Final:       data.Final,
		TotalTokens: data.TotalTokens,
		Duration:    time.Since(startTime),
	}

	outputs := []string{data.Outline, data.Draft, data.Edited, data.Final}
	for i, step := range trace.Steps {
		result.Steps = append(result.Steps, StepResult{
			Name:     step.Name,
			Duration: step.Duration,
			Output:   outputs[i],
		})
	}

	return result, nil
}
