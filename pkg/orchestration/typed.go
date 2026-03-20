package orchestration

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/athyr-tech/athyr-sdk-go/pkg/athyr"
)

// ExecutePipeline runs a pipeline with typed input/output.
// Input is JSON-marshaled before execution, and the final output is
// JSON-unmarshaled into the Out type.
func ExecutePipeline[In, Out any](p *Pipeline, ctx context.Context, a athyr.Agent, input In) (Out, error) {
	var zero Out

	data, err := json.Marshal(input)
	if err != nil {
		return zero, fmt.Errorf("marshal pipeline input: %w", err)
	}

	result, err := p.Execute(ctx, a, data)
	if err != nil {
		return zero, err
	}

	var out Out
	if err := json.Unmarshal(result, &out); err != nil {
		return zero, fmt.Errorf("unmarshal pipeline output: %w", err)
	}
	return out, nil
}

// ExecuteFanOut runs a fan-out with typed input/output.
// Input is JSON-marshaled before execution, and the aggregated output is
// JSON-unmarshaled into the Out type.
func ExecuteFanOut[In, Out any](f *FanOut, ctx context.Context, a athyr.Agent, input In, opts ...ExecuteOption) (Out, error) {
	var zero Out

	data, err := json.Marshal(input)
	if err != nil {
		return zero, fmt.Errorf("marshal fan-out input: %w", err)
	}

	result, err := f.Execute(ctx, a, data, opts...)
	if err != nil {
		return zero, err
	}

	var out Out
	if err := json.Unmarshal(result, &out); err != nil {
		return zero, fmt.Errorf("unmarshal fan-out output: %w", err)
	}
	return out, nil
}

// HandleHandoff runs a handoff router with typed input/output.
// Input is JSON-marshaled before execution, and the routed output is
// JSON-unmarshaled into the Out type.
func HandleHandoff[In, Out any](h *HandoffRouter, ctx context.Context, a athyr.Agent, input In) (Out, error) {
	var zero Out

	data, err := json.Marshal(input)
	if err != nil {
		return zero, fmt.Errorf("marshal handoff input: %w", err)
	}

	result, err := h.Handle(ctx, a, data)
	if err != nil {
		return zero, err
	}

	var out Out
	if err := json.Unmarshal(result, &out); err != nil {
		return zero, fmt.Errorf("unmarshal handoff output: %w", err)
	}
	return out, nil
}

// DiscussGroupChat runs a group chat discussion with typed input/output.
// Input is JSON-marshaled before execution, and the conclusion is
// JSON-unmarshaled into the Out type.
func DiscussGroupChat[In, Out any](g *GroupChat, ctx context.Context, a athyr.Agent, input In) (Out, error) {
	var zero Out

	data, err := json.Marshal(input)
	if err != nil {
		return zero, fmt.Errorf("marshal group chat input: %w", err)
	}

	result, err := g.Discuss(ctx, a, data)
	if err != nil {
		return zero, err
	}

	var out Out
	if err := json.Unmarshal(result, &out); err != nil {
		return zero, fmt.Errorf("unmarshal group chat output: %w", err)
	}
	return out, nil
}
