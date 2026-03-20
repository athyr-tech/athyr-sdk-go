package athyr

import (
	"context"
	"errors"
	"io"
	"time"

	athyr "github.com/athyr-tech/athyr-sdk-go/api/v1"
)

// Complete performs a blocking LLM completion.
func (c *agent) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	if err := c.checkConnected(); err != nil {
		return nil, err
	}

	protoReq := c.buildCompletionRequest(req)

	resp, err := c.athyr.Complete(ctx, protoReq)
	if err != nil {
		return nil, wrapError("Complete", err)
	}

	result := &CompletionResponse{
		Content:      resp.Content,
		Model:        resp.Model,
		Backend:      resp.Backend,
		FinishReason: resp.FinishReason,
		Latency:      time.Duration(resp.LatencyMs) * time.Millisecond,
		Usage: TokenUsage{
			PromptTokens:     int(resp.Usage.PromptTokens),
			CompletionTokens: int(resp.Usage.CompletionTokens),
			TotalTokens:      int(resp.Usage.TotalTokens),
		},
	}

	// Parse tool calls from response
	for _, tc := range resp.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			ID:        tc.Id,
			Name:      tc.Name,
			Arguments: []byte(tc.Arguments),
		})
	}

	return result, nil
}

// CompleteStream performs a streaming LLM completion.
func (c *agent) CompleteStream(ctx context.Context, req CompletionRequest, handler StreamHandler) error {
	if err := c.checkConnected(); err != nil {
		return err
	}

	protoReq := c.buildCompletionRequest(req)

	stream, err := c.athyr.CompleteStream(ctx, protoReq)
	if err != nil {
		return wrapError("CompleteStream", err)
	}

	// Track last error chunk for StreamError context
	var lastErrorChunk *athyr.StreamChunk

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			// If we received an error chunk before the gRPC error,
			// wrap it in a StreamError for agent retry decisions
			if lastErrorChunk != nil && lastErrorChunk.PartialResponse {
				return &StreamError{
					Err:                errors.New(lastErrorChunk.Error),
					Backend:            lastErrorChunk.Backend,
					AccumulatedContent: lastErrorChunk.AccumulatedContent,
					PartialResponse:    lastErrorChunk.PartialResponse,
				}
			}
			return wrapError("CompleteStream", err)
		}

		// Check for error chunk (sent before stream closes on failure)
		if chunk.Error != "" && chunk.Done {
			lastErrorChunk = chunk
			// Don't call handler for error chunks - the error will be returned
			continue
		}

		sdkChunk := StreamChunk{
			Content:  chunk.Content,
			Done:     chunk.Done,
			Model:    chunk.Model,
			Backend:  chunk.Backend,
			Error:    chunk.Error,
			Sequence: chunk.Sequence,
		}
		if chunk.StreamInfo != nil {
			sdkChunk.StreamInfo = &DurableStreamInfo{
				RequestID: chunk.StreamInfo.RequestId,
				Subject:   chunk.StreamInfo.Subject,
				Stream:    chunk.StreamInfo.Stream,
			}
		}
		if chunk.Usage != nil {
			sdkChunk.Usage = &TokenUsage{
				PromptTokens:     int(chunk.Usage.PromptTokens),
				CompletionTokens: int(chunk.Usage.CompletionTokens),
				TotalTokens:      int(chunk.Usage.TotalTokens),
			}
		}
		// Parse tool calls (complete on final chunk)
		for _, tc := range chunk.ToolCalls {
			sdkChunk.ToolCalls = append(sdkChunk.ToolCalls, ToolCall{
				ID:        tc.Id,
				Name:      tc.Name,
				Arguments: []byte(tc.Arguments),
			})
		}

		if err := handler(sdkChunk); err != nil {
			return err
		}
	}
}

// ResumeStream resumes a durable stream from a given sequence number.
// Use this to recover from disconnects during streaming completions initiated with Durable: true.
func (c *agent) ResumeStream(ctx context.Context, requestID string, lastSequence uint64, handler StreamHandler) error {
	if err := c.checkConnected(); err != nil {
		return err
	}

	stream, err := c.athyr.ResumeStream(ctx, &athyr.ResumeStreamRequest{
		AgentId:      c.agentID,
		RequestId:    requestID,
		LastSequence: lastSequence,
	})
	if err != nil {
		return wrapError("ResumeStream", err)
	}

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return wrapError("ResumeStream", err)
		}

		sdkChunk := StreamChunk{
			Content:  chunk.Content,
			Done:     chunk.Done,
			Model:    chunk.Model,
			Backend:  chunk.Backend,
			Error:    chunk.Error,
			Sequence: chunk.Sequence,
		}
		if chunk.Usage != nil {
			sdkChunk.Usage = &TokenUsage{
				PromptTokens:     int(chunk.Usage.PromptTokens),
				CompletionTokens: int(chunk.Usage.CompletionTokens),
				TotalTokens:      int(chunk.Usage.TotalTokens),
			}
		}
		for _, tc := range chunk.ToolCalls {
			sdkChunk.ToolCalls = append(sdkChunk.ToolCalls, ToolCall{
				ID:        tc.Id,
				Name:      tc.Name,
				Arguments: []byte(tc.Arguments),
			})
		}

		if err := handler(sdkChunk); err != nil {
			return err
		}
	}
}

// Models returns available LLM models.
func (c *agent) Models(ctx context.Context) ([]Model, error) {
	if err := c.checkConnected(); err != nil {
		return nil, err
	}

	resp, err := c.athyr.ListModels(ctx, &athyr.ListModelsRequest{})
	if err != nil {
		return nil, wrapError("Models", err)
	}

	models := make([]Model, len(resp.Models))
	for i, m := range resp.Models {
		models[i] = Model{
			ID:        m.Id,
			Name:      m.Name,
			Backend:   m.Backend,
			Available: m.Available,
		}
	}

	return models, nil
}

// buildCompletionRequest converts SDK request to proto request.
func (c *agent) buildCompletionRequest(req CompletionRequest) *athyr.CompletionRequest {
	protoReq := &athyr.CompletionRequest{
		AgentId:       c.agentID,
		Model:         req.Model,
		SessionId:     req.SessionID,
		IncludeMemory: req.IncludeMemory,
		ToolChoice:    req.ToolChoice,
		Durable:       req.Durable,
		Config: &athyr.CompletionConfig{
			Temperature: req.Config.Temperature,
			MaxTokens:   int32(req.Config.MaxTokens),
			TopP:        req.Config.TopP,
			Stop:        req.Config.Stop,
		},
	}

	// Convert messages, including tool calls
	for _, msg := range req.Messages {
		protoMsg := &athyr.Message{
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCallId: msg.ToolCallID,
		}
		// Convert tool calls for assistant messages
		for _, tc := range msg.ToolCalls {
			protoMsg.ToolCalls = append(protoMsg.ToolCalls, &athyr.ToolCall{
				Id:        tc.ID,
				Name:      tc.Name,
				Arguments: string(tc.Arguments),
			})
		}
		protoReq.Messages = append(protoReq.Messages, protoMsg)
	}

	// Convert tools
	for _, tool := range req.Tools {
		protoReq.Tools = append(protoReq.Tools, &athyr.Tool{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  string(tool.Parameters),
		})
	}

	return protoReq
}
