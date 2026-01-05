package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/athyr-tech/athyr-sdk-go/pkg/athyr"
)

// HandoffRouter provides dynamic task routing where a triage agent
// classifies requests and delegates to appropriate specialist agents.
//
// Example:
//
//	router := NewHandoffRouter("support-triage").
//	    Triage("agent.triage.invoke").
//	    Route("billing", "agent.billing.invoke").
//	    Route("technical", "agent.technical.invoke").
//	    Route("account", "agent.account.invoke").
//	    MaxHandoffs(5).
//	    Fallback("agent.escalation.invoke")
//
//	result, err := router.Handle(ctx, client, customerRequest)
type HandoffRouter struct {
	name        string
	triage      string
	routes      map[string]string
	maxHandoffs int
	fallback    string
}

// NewHandoffRouter creates a new handoff router.
func NewHandoffRouter(name string) *HandoffRouter {
	return &HandoffRouter{
		name:        name,
		routes:      make(map[string]string),
		maxHandoffs: 10, // sensible default
	}
}

// Triage sets the triage agent that classifies requests.
// The triage agent must return a HandoffDecision as JSON.
func (r *HandoffRouter) Triage(subject string) *HandoffRouter {
	r.triage = subject
	return r
}

// Route adds a route from a decision name to an agent subject.
func (r *HandoffRouter) Route(name, subject string) *HandoffRouter {
	r.routes[name] = subject
	return r
}

// MaxHandoffs sets the maximum number of handoffs allowed.
// Prevents infinite loops when agents keep re-routing.
// Default is 10.
func (r *HandoffRouter) MaxHandoffs(n int) *HandoffRouter {
	r.maxHandoffs = n
	return r
}

// Fallback sets the fallback agent for unhandled routes.
// Called when triage returns an unknown route name.
func (r *HandoffRouter) Fallback(subject string) *HandoffRouter {
	r.fallback = subject
	return r
}

// HandoffDecision is the expected response format from triage agents.
// Triage agents must return this as JSON.
type HandoffDecision struct {
	// Route is the name of the route to hand off to (e.g., "billing", "technical")
	Route string `json:"route"`

	// Context is additional context to pass to the next agent (optional)
	Context json.RawMessage `json:"context,omitempty"`

	// Handled indicates triage handled the request directly without routing
	Handled bool `json:"handled,omitempty"`

	// Response is the final response if Handled is true
	Response json.RawMessage `json:"response,omitempty"`
}

// Handle processes a request through the handoff router.
func (r *HandoffRouter) Handle(ctx context.Context, client athyr.Agent, input []byte) ([]byte, error) {
	trace, err := r.HandleWithTrace(ctx, client, input)
	if err != nil {
		return nil, err
	}
	return trace.Output, nil
}

// HandleWithTrace processes a request and returns detailed routing trace.
func (r *HandoffRouter) HandleWithTrace(ctx context.Context, client athyr.Agent, input []byte) (*HandoffTrace, error) {
	if r.triage == "" {
		return nil, &HandoffError{Err: fmt.Errorf("no triage agent configured")}
	}

	trace := &HandoffTrace{
		Router:    r.name,
		StartedAt: time.Now(),
		Path:      make([]HandoffStep, 0),
		Input:     input,
	}

	current := input

	for i := 0; i < r.maxHandoffs; i++ {
		step := HandoffStep{
			Index:     i,
			StartedAt: time.Now(),
		}

		// First iteration uses triage, subsequent use the routed agent
		var subject string
		if i == 0 {
			step.Agent = "triage"
			subject = r.triage
		} else {
			// Continue with current data to current subject
			step.Agent = trace.Path[len(trace.Path)-1].Route
			subject = r.routes[step.Agent]
		}
		step.Subject = subject

		// Call the agent
		output, err := client.Request(ctx, subject, current)
		step.Duration = time.Since(step.StartedAt)

		if err != nil {
			step.Error = err
			trace.Path = append(trace.Path, step)
			trace.Duration = time.Since(trace.StartedAt)
			trace.Error = &HandoffError{
				Agent: step.Agent,
				Err:   err,
				Path:  copyPath(trace.Path),
			}
			return trace, trace.Error
		}

		// Parse the decision
		var decision HandoffDecision
		if err := json.Unmarshal(output, &decision); err != nil {
			// If not a decision, treat as final response
			step.Route = "final"
			step.Output = output
			trace.Path = append(trace.Path, step)
			trace.Duration = time.Since(trace.StartedAt)
			trace.Output = output
			return trace, nil
		}

		step.Route = decision.Route

		// Check if agent handled directly
		if decision.Handled {
			step.Output = decision.Response
			trace.Path = append(trace.Path, step)
			trace.Duration = time.Since(trace.StartedAt)
			trace.Output = decision.Response
			return trace, nil
		}

		// If no route specified, treat as final response (not a handoff)
		if decision.Route == "" {
			step.Route = "final"
			step.Output = output
			trace.Path = append(trace.Path, step)
			trace.Duration = time.Since(trace.StartedAt)
			trace.Output = output
			return trace, nil
		}

		// Route to next agent
		if _, ok := r.routes[decision.Route]; !ok {
			// Unknown route - use fallback if available
			if r.fallback != "" {
				step.Route = "fallback"
				trace.Path = append(trace.Path, step)

				// Call fallback
				fallbackStep := HandoffStep{
					Index:     i + 1,
					Agent:     "fallback",
					Subject:   r.fallback,
					StartedAt: time.Now(),
				}

				fallbackOutput, err := client.Request(ctx, r.fallback, current)
				fallbackStep.Duration = time.Since(fallbackStep.StartedAt)

				if err != nil {
					fallbackStep.Error = err
					trace.Path = append(trace.Path, fallbackStep)
					trace.Duration = time.Since(trace.StartedAt)
					trace.Error = &HandoffError{
						Agent: "fallback",
						Err:   err,
						Path:  copyPath(trace.Path),
					}
					return trace, trace.Error
				}

				fallbackStep.Output = fallbackOutput
				fallbackStep.Route = "final"
				trace.Path = append(trace.Path, fallbackStep)
				trace.Duration = time.Since(trace.StartedAt)
				trace.Output = fallbackOutput
				return trace, nil
			}

			// No fallback - error
			trace.Path = append(trace.Path, step)
			trace.Duration = time.Since(trace.StartedAt)
			trace.Error = &HandoffError{
				Agent: step.Agent,
				Route: decision.Route,
				Err:   fmt.Errorf("unknown route %q and no fallback configured", decision.Route),
				Path:  copyPath(trace.Path),
			}
			return trace, trace.Error
		}

		// Prepare context for next agent
		if len(decision.Context) > 0 {
			current = decision.Context
		}

		trace.Path = append(trace.Path, step)

		// Route to the specialist agent
		specialistStep := HandoffStep{
			Index:     i + 1,
			Agent:     decision.Route,
			Subject:   r.routes[decision.Route],
			StartedAt: time.Now(),
		}

		specialistOutput, err := client.Request(ctx, r.routes[decision.Route], current)
		specialistStep.Duration = time.Since(specialistStep.StartedAt)

		if err != nil {
			specialistStep.Error = err
			trace.Path = append(trace.Path, specialistStep)
			trace.Duration = time.Since(trace.StartedAt)
			trace.Error = &HandoffError{
				Agent: decision.Route,
				Err:   err,
				Path:  copyPath(trace.Path),
			}
			return trace, trace.Error
		}

		// Check if specialist returns another handoff decision
		var nextDecision HandoffDecision
		if err := json.Unmarshal(specialistOutput, &nextDecision); err != nil {
			// Not a decision, treat as final response
			specialistStep.Output = specialistOutput
			specialistStep.Route = "final"
			trace.Path = append(trace.Path, specialistStep)
			trace.Duration = time.Since(trace.StartedAt)
			trace.Output = specialistOutput
			return trace, nil
		}

		// Specialist returned a decision - handle it
		if nextDecision.Handled {
			specialistStep.Output = nextDecision.Response
			specialistStep.Route = "final"
			trace.Path = append(trace.Path, specialistStep)
			trace.Duration = time.Since(trace.StartedAt)
			trace.Output = nextDecision.Response
			return trace, nil
		}

		// If no route specified, treat as final response
		if nextDecision.Route == "" {
			specialistStep.Output = specialistOutput
			specialistStep.Route = "final"
			trace.Path = append(trace.Path, specialistStep)
			trace.Duration = time.Since(trace.StartedAt)
			trace.Output = specialistOutput
			return trace, nil
		}

		// Another handoff - continue loop
		specialistStep.Route = nextDecision.Route
		trace.Path = append(trace.Path, specialistStep)
		current = input
		if len(nextDecision.Context) > 0 {
			current = nextDecision.Context
		}
	}

	// Max handoffs exceeded
	trace.Duration = time.Since(trace.StartedAt)
	trace.Error = &HandoffError{
		Err:  fmt.Errorf("max handoffs (%d) exceeded", r.maxHandoffs),
		Path: copyPath(trace.Path),
	}
	return trace, trace.Error
}

// HandoffTrace contains detailed routing information.
type HandoffTrace struct {
	Router    string
	Path      []HandoffStep
	Input     []byte
	Output    []byte
	StartedAt time.Time
	Duration  time.Duration
	Error     *HandoffError
}

// RouteNames returns the sequence of route names traversed.
func (t *HandoffTrace) RouteNames() []string {
	names := make([]string, 0, len(t.Path))
	for _, step := range t.Path {
		names = append(names, step.Agent)
	}
	return names
}

// HandoffStep contains details about a single handoff.
type HandoffStep struct {
	Index     int
	Agent     string // Agent name (e.g., "triage", "billing")
	Subject   string // Athyr subject
	Route     string // Route returned by agent
	Output    []byte
	StartedAt time.Time
	Duration  time.Duration
	Error     error
}

// HandoffError indicates routing failure.
type HandoffError struct {
	Agent string        // Agent that caused the failure
	Route string        // Route that was attempted
	Err   error         // Underlying error
	Path  []HandoffStep // Path up to failure
}

func (e *HandoffError) Error() string {
	if e.Agent != "" && e.Route != "" {
		return fmt.Sprintf("handoff failed: agent %q tried to route to %q: %v", e.Agent, e.Route, e.Err)
	}
	if e.Agent != "" {
		return fmt.Sprintf("handoff failed: agent %q: %v", e.Agent, e.Err)
	}
	return fmt.Sprintf("handoff failed: %v", e.Err)
}

func (e *HandoffError) Unwrap() error {
	return e.Err
}

// copyPath creates a copy of the path slice.
func copyPath(path []HandoffStep) []HandoffStep {
	if path == nil {
		return nil
	}
	result := make([]HandoffStep, len(path))
	copy(result, path)
	return result
}
