package orchestration

import (
	"context"
	"errors"
	"testing"
	"time"

	sdk "github.com/athyr-tech/athyr-sdk-go/pkg/agent"
)

// mockAgent implements sdk.Agent for testing orchestration patterns.
type mockAgent struct {
	// responses maps subject to response data
	responses map[string][]byte
	// errors maps subject to error
	errors map[string]error
	// calls tracks the order of requests
	calls []mockCall
}

type mockCall struct {
	subject string
	data    []byte
}

func newMockAgent() *mockAgent {
	return &mockAgent{
		responses: make(map[string][]byte),
		errors:    make(map[string]error),
		calls:     make([]mockCall, 0),
	}
}

func (m *mockAgent) OnRequest(subject string, response []byte) *mockAgent {
	m.responses[subject] = response
	return m
}

func (m *mockAgent) OnRequestError(subject string, err error) *mockAgent {
	m.errors[subject] = err
	return m
}

func (m *mockAgent) Request(ctx context.Context, subject string, data []byte) ([]byte, error) {
	m.calls = append(m.calls, mockCall{subject: subject, data: data})

	if err, ok := m.errors[subject]; ok {
		return nil, err
	}
	if resp, ok := m.responses[subject]; ok {
		return resp, nil
	}
	return nil, errors.New("no mock response configured for " + subject)
}

// Implement remaining sdk.Agent interface (not used in orchestration tests)
func (m *mockAgent) Connect(ctx context.Context) error { return nil }
func (m *mockAgent) Close() error                      { return nil }
func (m *mockAgent) AgentID() string                   { return "mock-agent" }
func (m *mockAgent) Connected() bool                   { return true }
func (m *mockAgent) Publish(ctx context.Context, subject string, data []byte) error {
	return nil
}
func (m *mockAgent) Subscribe(ctx context.Context, subject string, handler sdk.MessageHandler) (sdk.Subscription, error) {
	return nil, nil
}
func (m *mockAgent) QueueSubscribe(ctx context.Context, subject, queue string, handler sdk.MessageHandler) (sdk.Subscription, error) {
	return nil, nil
}
func (m *mockAgent) Complete(ctx context.Context, req sdk.CompletionRequest) (*sdk.CompletionResponse, error) {
	return nil, nil
}
func (m *mockAgent) CompleteStream(ctx context.Context, req sdk.CompletionRequest, handler sdk.StreamHandler) error {
	return nil
}
func (m *mockAgent) Models(ctx context.Context) ([]sdk.Model, error) { return nil, nil }
func (m *mockAgent) CreateSession(ctx context.Context, profile sdk.SessionProfile) (*sdk.Session, error) {
	return nil, nil
}
func (m *mockAgent) GetSession(ctx context.Context, sessionID string) (*sdk.Session, error) {
	return nil, nil
}
func (m *mockAgent) DeleteSession(ctx context.Context, sessionID string) error { return nil }
func (m *mockAgent) AddHint(ctx context.Context, sessionID, hint string) error { return nil }
func (m *mockAgent) KV(bucket string) sdk.KVBucket                             { return nil }

// Tests

func TestPipeline_Execute_SingleStep(t *testing.T) {
	client := newMockAgent().
		OnRequest("agent.process.invoke", []byte("processed"))

	pipeline := NewPipeline("test").
		Step("process", "agent.process.invoke")

	result, err := pipeline.Execute(context.Background(), client, []byte("input"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(result) != "processed" {
		t.Errorf("expected 'processed', got '%s'", result)
	}

	if len(client.calls) != 1 {
		t.Errorf("expected 1 call, got %d", len(client.calls))
	}
}

func TestPipeline_Execute_MultipleSteps(t *testing.T) {
	client := newMockAgent().
		OnRequest("agent.draft.invoke", []byte("draft content")).
		OnRequest("agent.review.invoke", []byte("reviewed content")).
		OnRequest("agent.polish.invoke", []byte("polished content"))

	pipeline := NewPipeline("doc-pipeline").
		Step("draft", "agent.draft.invoke").
		Step("review", "agent.review.invoke").
		Step("polish", "agent.polish.invoke")

	result, err := pipeline.Execute(context.Background(), client, []byte("topic"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(result) != "polished content" {
		t.Errorf("expected 'polished content', got '%s'", result)
	}

	// Verify chaining: each step receives output of previous
	if len(client.calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(client.calls))
	}
	if string(client.calls[0].data) != "topic" {
		t.Errorf("step 0 input: expected 'topic', got '%s'", client.calls[0].data)
	}
	if string(client.calls[1].data) != "draft content" {
		t.Errorf("step 1 input: expected 'draft content', got '%s'", client.calls[1].data)
	}
	if string(client.calls[2].data) != "reviewed content" {
		t.Errorf("step 2 input: expected 'reviewed content', got '%s'", client.calls[2].data)
	}
}

func TestPipeline_Execute_StepFailure(t *testing.T) {
	expectedErr := errors.New("review agent unavailable")
	client := newMockAgent().
		OnRequest("agent.draft.invoke", []byte("draft")).
		OnRequestError("agent.review.invoke", expectedErr)

	pipeline := NewPipeline("test").
		Step("draft", "agent.draft.invoke").
		Step("review", "agent.review.invoke").
		Step("polish", "agent.polish.invoke")

	_, err := pipeline.Execute(context.Background(), client, []byte("input"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var pipelineErr *PipelineError
	if !errors.As(err, &pipelineErr) {
		t.Fatalf("expected PipelineError, got %T", err)
	}

	if pipelineErr.Step != "review" {
		t.Errorf("expected step 'review', got '%s'", pipelineErr.Step)
	}
	if pipelineErr.Index != 1 {
		t.Errorf("expected index 1, got %d", pipelineErr.Index)
	}
	if !errors.Is(pipelineErr, expectedErr) {
		t.Errorf("expected wrapped error to be %v", expectedErr)
	}
}

func TestPipeline_ExecuteWithTrace(t *testing.T) {
	client := newMockAgent().
		OnRequest("agent.a.invoke", []byte("output-a")).
		OnRequest("agent.b.invoke", []byte("output-b"))

	pipeline := NewPipeline("traced").
		Step("step-a", "agent.a.invoke").
		Step("step-b", "agent.b.invoke")

	trace, err := pipeline.ExecuteWithTrace(context.Background(), client, []byte("start"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if trace.Pipeline != "traced" {
		t.Errorf("expected pipeline name 'traced', got '%s'", trace.Pipeline)
	}
	if len(trace.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(trace.Steps))
	}

	// Check step 0
	if trace.Steps[0].Name != "step-a" {
		t.Errorf("step 0 name: expected 'step-a', got '%s'", trace.Steps[0].Name)
	}
	if string(trace.Steps[0].Input) != "start" {
		t.Errorf("step 0 input: expected 'start', got '%s'", trace.Steps[0].Input)
	}
	if string(trace.Steps[0].Output) != "output-a" {
		t.Errorf("step 0 output: expected 'output-a', got '%s'", trace.Steps[0].Output)
	}
	if trace.Steps[0].Duration == 0 {
		t.Error("step 0 duration should be > 0")
	}

	// Check step 1
	if string(trace.Steps[1].Input) != "output-a" {
		t.Errorf("step 1 input: expected 'output-a', got '%s'", trace.Steps[1].Input)
	}
	if string(trace.Steps[1].Output) != "output-b" {
		t.Errorf("step 1 output: expected 'output-b', got '%s'", trace.Steps[1].Output)
	}

	// Check trace output helper
	if string(trace.Output()) != "output-b" {
		t.Errorf("trace.Output(): expected 'output-b', got '%s'", trace.Output())
	}

	if trace.Duration == 0 {
		t.Error("trace duration should be > 0")
	}
}

func TestPipeline_WithTransform(t *testing.T) {
	client := newMockAgent().
		OnRequest("agent.process.invoke", []byte("result"))

	pipeline := NewPipeline("transform-test").
		Step("process", "agent.process.invoke",
			WithTransform(func(data []byte) ([]byte, error) {
				return []byte("transformed:" + string(data)), nil
			}))

	result, err := pipeline.Execute(context.Background(), client, []byte("input"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(result) != "transformed:result" {
		t.Errorf("expected 'transformed:result', got '%s'", result)
	}
}

func TestPipeline_WithTransformError(t *testing.T) {
	client := newMockAgent().
		OnRequest("agent.process.invoke", []byte("result"))

	transformErr := errors.New("transform failed")
	pipeline := NewPipeline("transform-error-test").
		Step("process", "agent.process.invoke",
			WithTransform(func(data []byte) ([]byte, error) {
				return nil, transformErr
			}))

	_, err := pipeline.Execute(context.Background(), client, []byte("input"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var pipelineErr *PipelineError
	if !errors.As(err, &pipelineErr) {
		t.Fatalf("expected PipelineError, got %T", err)
	}
}

func TestPipeline_WithTimeout(t *testing.T) {
	// Just verify the option is applied without error
	// Actual timeout behavior depends on client implementation
	pipeline := NewPipeline("timeout-test").
		Step("slow", "agent.slow.invoke", WithTimeout(5*time.Second))

	if len(pipeline.steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(pipeline.steps))
	}
	if pipeline.steps[0].opts.timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", pipeline.steps[0].opts.timeout)
	}
}

func TestPipeline_EmptyPipeline(t *testing.T) {
	client := newMockAgent()
	pipeline := NewPipeline("empty")

	result, err := pipeline.Execute(context.Background(), client, []byte("input"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Empty pipeline returns input unchanged
	if string(result) != "input" {
		t.Errorf("expected 'input', got '%s'", result)
	}
}

func TestPipeline_Reusable(t *testing.T) {
	// Pipeline should be reusable across multiple executions
	client := newMockAgent().
		OnRequest("agent.echo.invoke", []byte("echoed"))

	pipeline := NewPipeline("reusable").
		Step("echo", "agent.echo.invoke")

	// Execute twice
	result1, _ := pipeline.Execute(context.Background(), client, []byte("first"))
	result2, _ := pipeline.Execute(context.Background(), client, []byte("second"))

	if string(result1) != "echoed" || string(result2) != "echoed" {
		t.Error("pipeline should be reusable")
	}
	if len(client.calls) != 2 {
		t.Errorf("expected 2 calls, got %d", len(client.calls))
	}
}
