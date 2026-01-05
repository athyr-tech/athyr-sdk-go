// Package orchestration provides helpers for common agent orchestration patterns.
// These patterns simplify building multi-agent systems with Athyr.
package orchestration

import (
	"context"
	"fmt"
	"time"

	"github.com/athyr-tech/athyr-sdk-go/pkg/athyr"
)

// Pipeline chains agents in sequential order where each agent
// processes the output from the previous agent.
//
// Example:
//
//	pipeline := NewPipeline("doc-pipeline").
//	    Step("draft", "agent.draft.invoke").
//	    Step("review", "agent.review.invoke").
//	    Step("polish", "agent.polish.invoke")
//
//	result, err := pipeline.Execute(ctx, client, input)
type Pipeline struct {
	name  string
	steps []pipelineStep
}

type pipelineStep struct {
	name    string
	subject string
	opts    stepOptions
}

type stepOptions struct {
	timeout   time.Duration
	transform func([]byte) ([]byte, error)
}

// StepOption configures a pipeline step.
type StepOption func(*stepOptions)

// WithTimeout sets a timeout for this step.
// If not set, the context timeout applies.
func WithTimeout(d time.Duration) StepOption {
	return func(o *stepOptions) {
		o.timeout = d
	}
}

// WithTransform applies a transformation to the step output
// before passing it to the next step.
func WithTransform(fn func([]byte) ([]byte, error)) StepOption {
	return func(o *stepOptions) {
		o.transform = fn
	}
}

// NewPipeline creates a new sequential pipeline.
func NewPipeline(name string) *Pipeline {
	return &Pipeline{
		name:  name,
		steps: make([]pipelineStep, 0),
	}
}

// Step adds a step to the pipeline.
// Steps are executed in the order they are added.
func (p *Pipeline) Step(name, subject string, opts ...StepOption) *Pipeline {
	step := pipelineStep{
		name:    name,
		subject: subject,
	}
	for _, opt := range opts {
		opt(&step.opts)
	}
	p.steps = append(p.steps, step)
	return p
}

// Execute runs the pipeline and returns the final result.
// Each step receives the output of the previous step as input.
func (p *Pipeline) Execute(ctx context.Context, client athyr.Agent, input []byte) ([]byte, error) {
	trace, err := p.ExecuteWithTrace(ctx, client, input)
	if err != nil {
		return nil, err
	}
	if len(trace.Steps) == 0 {
		return input, nil
	}
	return trace.Steps[len(trace.Steps)-1].Output, nil
}

// ExecuteWithTrace runs the pipeline and returns detailed execution trace.
// Useful for debugging and monitoring.
func (p *Pipeline) ExecuteWithTrace(ctx context.Context, client athyr.Agent, input []byte) (*PipelineTrace, error) {
	trace := &PipelineTrace{
		Pipeline:  p.name,
		StartedAt: time.Now(),
		Steps:     make([]StepTrace, 0, len(p.steps)),
	}

	current := input

	for i, step := range p.steps {
		stepTrace := StepTrace{
			Index:     i,
			Name:      step.name,
			Subject:   step.subject,
			Input:     current,
			StartedAt: time.Now(),
		}

		// Apply step-level timeout if configured
		stepCtx := ctx
		var cancelStep context.CancelFunc
		if step.opts.timeout > 0 {
			stepCtx, cancelStep = context.WithTimeout(ctx, step.opts.timeout)
		}

		// Execute the step
		output, err := client.Request(stepCtx, step.subject, current)

		// Cancel step context immediately to release resources
		if cancelStep != nil {
			cancelStep()
		}
		stepTrace.Duration = time.Since(stepTrace.StartedAt)

		if err != nil {
			stepTrace.Error = err
			trace.Steps = append(trace.Steps, stepTrace)
			trace.Duration = time.Since(trace.StartedAt)
			trace.Error = &PipelineError{
				Step:    step.name,
				Index:   i,
				Subject: step.subject,
				Err:     err,
			}
			return trace, trace.Error
		}

		// Apply transform if configured
		if step.opts.transform != nil {
			output, err = step.opts.transform(output)
			if err != nil {
				stepTrace.Error = err
				trace.Steps = append(trace.Steps, stepTrace)
				trace.Duration = time.Since(trace.StartedAt)
				trace.Error = &PipelineError{
					Step:    step.name,
					Index:   i,
					Subject: step.subject,
					Err:     fmt.Errorf("transform failed: %w", err),
				}
				return trace, trace.Error
			}
		}

		stepTrace.Output = output
		trace.Steps = append(trace.Steps, stepTrace)
		current = output
	}

	trace.Duration = time.Since(trace.StartedAt)
	return trace, nil
}

// PipelineTrace contains detailed execution information.
type PipelineTrace struct {
	Pipeline  string
	Steps     []StepTrace
	StartedAt time.Time
	Duration  time.Duration
	Error     *PipelineError
}

// Output returns the final output of the pipeline.
// Returns nil if pipeline has no steps or failed.
func (t *PipelineTrace) Output() []byte {
	if len(t.Steps) == 0 {
		return nil
	}
	lastStep := t.Steps[len(t.Steps)-1]
	if lastStep.Error != nil {
		return nil
	}
	return lastStep.Output
}

// StepTrace contains execution details for a single step.
type StepTrace struct {
	Index     int
	Name      string
	Subject   string
	Input     []byte
	Output    []byte
	StartedAt time.Time
	Duration  time.Duration
	Error     error
}

// PipelineError indicates which step failed.
type PipelineError struct {
	Step    string
	Index   int
	Subject string
	Err     error
}

func (e *PipelineError) Error() string {
	return fmt.Sprintf("pipeline step %q (index %d, subject %s) failed: %v",
		e.Step, e.Index, e.Subject, e.Err)
}

func (e *PipelineError) Unwrap() error {
	return e.Err
}
