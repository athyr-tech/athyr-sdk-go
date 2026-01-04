package orchestration

import (
	"context"
	"fmt"
	"sync"
	"time"

	sdk "github.com/athyr-tech/athyr-sdk-go"
)

// FanOut runs multiple agents in parallel on the same input
// and aggregates their results.
//
// Example:
//
//	fanout := NewFanOut("stock-analysis").
//	    Agent("fundamental", "agent.fundamental.invoke").
//	    Agent("technical", "agent.technical.invoke").
//	    Agent("sentiment", "agent.sentiment.invoke")
//
//	result, err := fanout.Execute(ctx, client, input, AllMustSucceed())
type FanOut struct {
	name   string
	agents []fanoutAgent
}

type fanoutAgent struct {
	name    string
	subject string
	opts    agentOptions
}

type agentOptions struct {
	timeout time.Duration
}

// AgentOption configures a fan-out agent.
type AgentOption func(*agentOptions)

// WithAgentTimeout sets a timeout for this specific agent.
func WithAgentTimeout(d time.Duration) AgentOption {
	return func(o *agentOptions) {
		o.timeout = d
	}
}

// NewFanOut creates a new fan-out orchestrator.
func NewFanOut(name string) *FanOut {
	return &FanOut{
		name:   name,
		agents: make([]fanoutAgent, 0),
	}
}

// Agent adds an agent to the fan-out.
// All agents receive the same input and run in parallel.
func (f *FanOut) Agent(name, subject string, opts ...AgentOption) *FanOut {
	agent := fanoutAgent{
		name:    name,
		subject: subject,
	}
	for _, opt := range opts {
		opt(&agent.opts)
	}
	f.agents = append(f.agents, agent)
	return f
}

// Aggregator combines results from multiple agents into a single result.
type Aggregator func(results map[string]AgentResult) ([]byte, error)

// AgentResult holds the result from a single agent.
type AgentResult struct {
	Output   []byte
	Error    error
	Duration time.Duration
}

// Success returns true if the agent completed without error.
func (r AgentResult) Success() bool {
	return r.Error == nil
}

// ExecuteOption configures fan-out execution.
type ExecuteOption func(*executeOptions)

type executeOptions struct {
	aggregator Aggregator
}

// WithAggregator sets a custom aggregator function.
func WithAggregator(agg Aggregator) ExecuteOption {
	return func(o *executeOptions) {
		o.aggregator = agg
	}
}

// AllMustSucceed returns an aggregator that fails if any agent fails.
// On success, returns all results as JSON object.
func AllMustSucceed() ExecuteOption {
	return WithAggregator(func(results map[string]AgentResult) ([]byte, error) {
		combined := make(map[string][]byte)
		for name, result := range results {
			if result.Error != nil {
				return nil, &FanOutError{
					FailedAgent: name,
					Err:         result.Error,
					Results:     results,
				}
			}
			combined[name] = result.Output
		}
		return marshalResults(combined), nil
	})
}

// FirstSuccess returns an aggregator that returns the first successful result.
// Fails only if all agents fail.
func FirstSuccess() ExecuteOption {
	return WithAggregator(func(results map[string]AgentResult) ([]byte, error) {
		var firstSuccess []byte
		var allErrors []error

		for name, result := range results {
			if result.Error == nil {
				if firstSuccess == nil {
					firstSuccess = result.Output
				}
			} else {
				allErrors = append(allErrors, fmt.Errorf("%s: %w", name, result.Error))
			}
		}

		if firstSuccess != nil {
			return firstSuccess, nil
		}

		return nil, &FanOutError{
			Err:     fmt.Errorf("all agents failed"),
			Results: results,
		}
	})
}

// CollectAll returns an aggregator that collects all results (including failures).
// Never fails - returns all results as JSON with success/error status.
func CollectAll() ExecuteOption {
	return WithAggregator(func(results map[string]AgentResult) ([]byte, error) {
		combined := make(map[string][]byte)
		for name, result := range results {
			if result.Error == nil {
				combined[name] = result.Output
			}
			// Skip failed agents in output, but don't fail
		}
		return marshalResults(combined), nil
	})
}

// RequireQuorum returns an aggregator that requires at least n agents to succeed.
func RequireQuorum(n int) ExecuteOption {
	return WithAggregator(func(results map[string]AgentResult) ([]byte, error) {
		combined := make(map[string][]byte)
		successCount := 0

		for name, result := range results {
			if result.Error == nil {
				combined[name] = result.Output
				successCount++
			}
		}

		if successCount < n {
			return nil, &FanOutError{
				Err:     fmt.Errorf("quorum not reached: %d/%d succeeded, need %d", successCount, len(results), n),
				Results: results,
			}
		}

		return marshalResults(combined), nil
	})
}

// Execute runs all agents in parallel and aggregates results.
func (f *FanOut) Execute(ctx context.Context, client sdk.Agent, input []byte, opts ...ExecuteOption) ([]byte, error) {
	trace, err := f.ExecuteWithTrace(ctx, client, input, opts...)
	if err != nil {
		return nil, err
	}
	return trace.Output, nil
}

// ExecuteWithTrace runs all agents and returns detailed execution trace.
func (f *FanOut) ExecuteWithTrace(ctx context.Context, client sdk.Agent, input []byte, opts ...ExecuteOption) (*FanOutTrace, error) {
	// Apply options
	execOpts := &executeOptions{
		aggregator: defaultAggregator,
	}
	for _, opt := range opts {
		opt(execOpts)
	}

	trace := &FanOutTrace{
		FanOut:    f.name,
		StartedAt: time.Now(),
		Agents:    make(map[string]AgentTrace),
	}

	// Run all agents in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make(map[string]AgentResult)

	for _, agent := range f.agents {
		wg.Add(1)
		go func(a fanoutAgent) {
			defer wg.Done()

			agentTrace := AgentTrace{
				Name:      a.name,
				Subject:   a.subject,
				StartedAt: time.Now(),
			}

			// Apply agent-level timeout if configured
			agentCtx := ctx
			if a.opts.timeout > 0 {
				var cancel context.CancelFunc
				agentCtx, cancel = context.WithTimeout(ctx, a.opts.timeout)
				defer cancel()
			}

			output, err := client.Request(agentCtx, a.subject, input)
			agentTrace.Duration = time.Since(agentTrace.StartedAt)

			result := AgentResult{
				Duration: agentTrace.Duration,
			}

			if err != nil {
				agentTrace.Error = err
				result.Error = err
			} else {
				agentTrace.Output = output
				result.Output = output
			}

			mu.Lock()
			trace.Agents[a.name] = agentTrace
			results[a.name] = result
			mu.Unlock()
		}(agent)
	}

	wg.Wait()
	trace.Duration = time.Since(trace.StartedAt)

	// Aggregate results
	output, err := execOpts.aggregator(results)
	if err != nil {
		trace.Error = err
		return trace, err
	}

	trace.Output = output
	return trace, nil
}

// FanOutTrace contains detailed execution information.
type FanOutTrace struct {
	FanOut    string
	Agents    map[string]AgentTrace
	Output    []byte
	StartedAt time.Time
	Duration  time.Duration
	Error     error
}

// SuccessCount returns the number of agents that succeeded.
func (t *FanOutTrace) SuccessCount() int {
	count := 0
	for _, agent := range t.Agents {
		if agent.Error == nil {
			count++
		}
	}
	return count
}

// FailureCount returns the number of agents that failed.
func (t *FanOutTrace) FailureCount() int {
	return len(t.Agents) - t.SuccessCount()
}

// AgentTrace contains execution details for a single agent.
type AgentTrace struct {
	Name      string
	Subject   string
	Output    []byte
	StartedAt time.Time
	Duration  time.Duration
	Error     error
}

// FanOutError indicates fan-out execution failure.
type FanOutError struct {
	FailedAgent string                 // Primary agent that caused failure (if applicable)
	Err         error                  // Underlying error
	Results     map[string]AgentResult // All agent results for inspection
}

func (e *FanOutError) Error() string {
	if e.FailedAgent != "" {
		return fmt.Sprintf("fan-out failed: agent %q: %v", e.FailedAgent, e.Err)
	}
	return fmt.Sprintf("fan-out failed: %v", e.Err)
}

func (e *FanOutError) Unwrap() error {
	return e.Err
}

// defaultAggregator collects all successful results.
func defaultAggregator(results map[string]AgentResult) ([]byte, error) {
	combined := make(map[string][]byte)
	for name, result := range results {
		if result.Error == nil {
			combined[name] = result.Output
		}
	}
	return marshalResults(combined), nil
}

// marshalResults converts results map to simple JSON-like format.
// Using simple format to avoid json package dependency for basic use.
func marshalResults(results map[string][]byte) []byte {
	if len(results) == 0 {
		return []byte("{}")
	}

	// Build simple JSON manually to avoid import
	// Format: {"name1":"base64data1","name2":"base64data2"}
	// For simplicity, we'll use a format that's easy to parse
	// Real implementation might use encoding/json

	// For now, just concatenate with separator
	// Users with custom needs should use WithAggregator
	var out []byte
	out = append(out, '{')
	first := true
	for name, data := range results {
		if !first {
			out = append(out, ',')
		}
		first = false
		out = append(out, '"')
		out = append(out, name...)
		out = append(out, '"', ':', '"')
		out = append(out, data...)
		out = append(out, '"')
	}
	out = append(out, '}')
	return out
}
