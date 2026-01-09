package main

import (
	"context"
	"fmt"

	"github.com/athyr-tech/athyr-sdk-go/pkg/athyr"
)

// RunToolLoop executes the agentic tool-calling loop.
//
// The loop:
//  1. Sends messages + tools to the LLM
//  2. If LLM requests tools → execute them, add results, goto 1
//  3. If LLM responds with text → return it
func RunToolLoop(ctx context.Context, agent athyr.Agent, model string, userMessage string) (string, error) {
	messages := []athyr.Message{
		{Role: "system", Content: "You are a helpful assistant. Use tools when needed."},
		{Role: "user", Content: userMessage},
	}

	for i := 0; i < 5; i++ { // safety limit
		resp, err := agent.Complete(ctx, athyr.CompletionRequest{
			Model:      model,
			Messages:   messages,
			Tools:      Tools,
			ToolChoice: "auto",
		})
		if err != nil {
			return "", err
		}

		// No tool calls = done
		if len(resp.ToolCalls) == 0 {
			return resp.Content, nil
		}

		// LLM wants to use tools
		fmt.Printf("\n  [%d tool call(s)]\n", len(resp.ToolCalls))

		// Add assistant's tool request to history
		messages = append(messages, athyr.Message{
			Role:      "assistant",
			ToolCalls: resp.ToolCalls,
		})

		// Execute tools and add results
		for _, call := range resp.ToolCalls {
			result := ExecuteTool(call)
			fmt.Printf("  → %s(%s) = %s\n", call.Name, string(call.Arguments), result)

			messages = append(messages, athyr.Message{
				Role:       "tool",
				ToolCallID: call.ID,
				Content:    result,
			})
		}
	}

	return "", fmt.Errorf("max iterations reached")
}
