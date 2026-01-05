// Example: Resilient Agent
//
// Demonstrates graceful handling of streaming errors using StreamError.
// Shows how agents can:
// - Detect partial responses
// - Retry with the same or different model
// - Fall back to blocking mode
// - Provide good UX during failures
//
// Usage:
//
//	# Start Athyr server first
//	go run ./cmd/athyr serve
//
//	# Run the resilient agent
//	go run ./examples/resilient-agent
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/athyr-tech/athyr-sdk-go/pkg/athyr"
)

const (
	athyrAddr    = "localhost:9090"
	primaryModel = "qwen3:4b"     // Primary model
	backupModel  = "smollm2:135m" // Fast fallback model
	maxRetries   = 2
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n\nShutting down...")
		cancel()
	}()

	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	fmt.Println("╔════════════════════════════════════════════╗")
	fmt.Println("║   Resilient Agent - StreamError Demo       ║")
	fmt.Println("╚════════════════════════════════════════════╝")
	fmt.Println()

	client, err := athyr.NewAgent(athyrAddr,
		athyr.WithAgentCard(athyr.AgentCard{
			Name:         "resilient-agent",
			Description:  "Demonstrates graceful streaming error handling",
			Version:      "1.0.0",
			Capabilities: []string{"chat", "resilient"},
		}),
		athyr.WithHeartbeatInterval(30*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	fmt.Print("Connecting to Athyr... ")
	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	fmt.Printf("Connected! (Agent ID: %s)\n", client.AgentID())
	defer func() { _ = client.Close() }()

	fmt.Println()
	fmt.Println("This agent demonstrates resilient streaming with automatic")
	fmt.Println("error recovery. If a stream fails mid-response, it will:")
	fmt.Println("  1. Show what was received before failure")
	fmt.Println("  2. Retry with the same model")
	fmt.Println("  3. Fall back to a different model")
	fmt.Println("  4. Fall back to blocking mode as last resort")
	fmt.Println()
	fmt.Println("Type your message and press Enter. Type /quit to exit.")
	fmt.Println("─────────────────────────────────────────────────────────")

	scanner := bufio.NewScanner(os.Stdin)
	model := primaryModel

	for {
		fmt.Print("\nYou: ")
		if !scanner.Scan() {
			break
		}

		select {
		case <-ctx.Done():
			return nil
		default:
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "/quit" {
			return nil
		}

		fmt.Print("\nAssistant: ")

		req := athyr.CompletionRequest{
			Model: model,
			Messages: []athyr.Message{
				{Role: "system", Content: "You are a helpful assistant. Be concise."},
				{Role: "user", Content: input},
			},
		}

		// Try resilient completion with retries and fallbacks
		if err := resilientStream(ctx, client, req); err != nil {
			fmt.Printf("\n[Error: %v]\n", err)
		}
	}

	return nil
}

// resilientStream attempts streaming with automatic error recovery.
// It tries: same model retry → backup model → blocking mode.
func resilientStream(ctx context.Context, client athyr.Agent, req athyr.CompletionRequest) error {
	var lastErr error

	// Strategy 1: Try primary model with retries
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := streamWithRecovery(ctx, client, req, attempt)
		if err == nil {
			return nil
		}

		lastErr = err
		var streamErr *athyr.StreamError
		if errors.As(err, &streamErr) {
			if streamErr.PartialResponse {
				// Show recovery message inline
				fmt.Printf("\n[⚠ Stream interrupted after %d chars, retrying...]\n",
					len(streamErr.AccumulatedContent))
			} else if attempt < maxRetries {
				fmt.Printf("\n[⚠ Connection failed, retry %d/%d...]\n", attempt, maxRetries)
			}
		}
	}

	// Strategy 2: Try backup model
	if req.Model != backupModel {
		fmt.Printf("\n[⚠ Trying backup model: %s...]\n", backupModel)
		backupReq := req
		backupReq.Model = backupModel

		err := streamWithRecovery(ctx, client, backupReq, 1)
		if err == nil {
			return nil
		}
		lastErr = err
	}

	// Strategy 3: Fall back to blocking mode
	fmt.Print("\n[⚠ Falling back to non-streaming mode...]\n")
	fmt.Print("Assistant: ")

	resp, err := client.Complete(ctx, req)
	if err != nil {
		return fmt.Errorf("all strategies failed: %w", lastErr)
	}

	fmt.Println(resp.Content)
	fmt.Printf("  [%s via %s, blocking mode]\n", resp.Model, resp.Backend)
	return nil
}

// streamWithRecovery performs a streaming request and handles StreamError.
func streamWithRecovery(ctx context.Context, client athyr.Agent, req athyr.CompletionRequest, attempt int) error {
	var totalContent strings.Builder

	err := client.CompleteStream(ctx, req, func(chunk athyr.StreamChunk) error {
		fmt.Print(chunk.Content)
		totalContent.WriteString(chunk.Content)
		return nil
	})

	if err == nil {
		fmt.Println() // Newline after successful stream
		return nil
	}

	// Check if this is a StreamError with context
	var streamErr *athyr.StreamError
	if errors.As(err, &streamErr) {
		// Log details for debugging (in production, send to observability)
		logStreamError(streamErr, attempt)
		return streamErr
	}

	return err
}

// logStreamError logs details about the streaming failure.
func logStreamError(err *athyr.StreamError, attempt int) {
	// In production, you'd send this to your observability stack
	details := fmt.Sprintf(
		"\n  Backend: %s | Partial: %v | Content: %d chars",
		err.Backend,
		err.PartialResponse,
		len(err.AccumulatedContent),
	)
	_ = details // Suppress unused warning; would be logged in production
}
