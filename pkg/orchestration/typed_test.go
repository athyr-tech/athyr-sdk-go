package orchestration

import (
	"context"
	"testing"
)

type typedInput struct {
	Query string `json:"query"`
}

type typedOutput struct {
	Result string `json:"result"`
}

func TestExecutePipeline_Typed(t *testing.T) {
	agent := newMockAgent().
		OnRequest("step1", []byte(`{"result":"processed"}`))

	p := NewPipeline("test").Step("step1", "step1")

	out, err := ExecutePipeline[typedInput, typedOutput](p, context.Background(), agent, typedInput{Query: "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Result != "processed" {
		t.Errorf("expected result 'processed', got %q", out.Result)
	}
}

func TestExecuteFanOut_Typed(t *testing.T) {
	// FanOut with a custom aggregator that produces valid JSON output
	agent := newMockAgent().
		OnRequest("agent1", []byte(`{"result":"r1"}`))

	f := NewFanOut("test").
		Agent("a1", "agent1")

	out, err := ExecuteFanOut[typedInput, typedOutput](
		f, context.Background(), agent, typedInput{Query: "test"},
		WithAggregator(func(results map[string]AgentResult) ([]byte, error) {
			// Return the first successful result directly
			for _, r := range results {
				if r.Error == nil {
					return r.Output, nil
				}
			}
			return nil, nil
		}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Result != "r1" {
		t.Errorf("expected result 'r1', got %q", out.Result)
	}
}

func TestHandleHandoff_Typed(t *testing.T) {
	agent := newMockAgent().
		OnRequest("triage", []byte(`{"handled":true,"response":{"result":"triaged"}}`))

	r := NewHandoffRouter("test").Triage("triage")

	out, err := HandleHandoff[typedInput, typedOutput](r, context.Background(), agent, typedInput{Query: "help"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Result != "triaged" {
		t.Errorf("expected result 'triaged', got %q", out.Result)
	}
}

func TestDiscussGroupChat_Typed(t *testing.T) {
	// GroupChat conclusion is the last message's Content as raw bytes.
	// Use a JSON-formatted content so it can be unmarshaled.
	agent := newMockAgent().
		OnRequest("p1", []byte(`{"content":"{\"result\":\"consensus\"}"}`))

	g := NewGroupChat("test").
		Participant("p1", "p1").
		MaxRounds(1)

	out, err := DiscussGroupChat[typedInput, typedOutput](g, context.Background(), agent, typedInput{Query: "topic"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Result != "consensus" {
		t.Errorf("expected result 'consensus', got %q", out.Result)
	}
}

func TestExecutePipeline_MarshalError(t *testing.T) {
	p := NewPipeline("test").Step("s1", "s1")
	agent := newMockAgent()

	// channels can't be JSON-marshaled
	type bad struct {
		Ch chan int `json:"ch"`
	}
	_, err := ExecutePipeline[bad, typedOutput](p, context.Background(), agent, bad{})
	if err == nil {
		t.Error("expected marshal error")
	}
}

func TestExecutePipeline_UnmarshalError(t *testing.T) {
	agent := newMockAgent().
		OnRequest("s1", []byte(`not valid json`))

	p := NewPipeline("test").Step("s1", "s1")

	_, err := ExecutePipeline[typedInput, typedOutput](p, context.Background(), agent, typedInput{Query: "x"})
	if err == nil {
		t.Error("expected unmarshal error")
	}
}
