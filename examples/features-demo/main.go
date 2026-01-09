// Example: Features Demo
//
// A conversational assistant showcasing Athyr SDK features:
// - Agent registration and lifecycle
// - Memory sessions for conversation context
// - LLM completions (blocking and streaming)
// - KV storage for user preferences
// - Pub/sub messaging
//
// Usage:
//
//	# Start Athyr server first
//	go run ./cmd/athyr serve
//
//	# In another terminal, run the agent
//	go run ./examples/features-demo
package main

import (
	"bufio"
	"context"
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
	defaultModel = "qwen3:4b"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
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
	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Println("║       Athyr Chat Agent Example       ║")
	fmt.Println("╚══════════════════════════════════════╝")
	fmt.Println()

	// Create SDK client
	client, err := athyr.NewAgent(athyrAddr,
		athyr.WithAgentCard(athyr.AgentCard{
			Name:         "chat-agent",
			Description:  "Interactive chat assistant demonstrating Athyr SDK",
			Version:      "1.0.0",
			Capabilities: []string{"chat", "memory", "preferences"},
		}),
		athyr.WithHeartbeatInterval(30*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Connect to Athyr
	fmt.Print("Connecting to Athyr... ")
	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	fmt.Printf("Connected! (Agent ID: %s)\n", client.AgentID())
	defer func() { _ = client.Close() }()

	// Create a memory session for this conversation with a system prompt
	fmt.Print("Creating memory session... ")
	session, err := client.CreateSession(ctx, athyr.DefaultSessionProfile(), "You are a helpful assistant.")
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	fmt.Printf("Session ID: %s\n", session.ID)

	// Load or initialize user preferences from KV
	prefs := client.KV("user-prefs")
	model := defaultModel
	if modelBytes, err := prefs.Get(ctx, "model"); err == nil {
		model = string(modelBytes.Value)
		fmt.Printf("Loaded preferred model: %s\n", model)
	}

	// Check available models
	models, err := client.Models(ctx)
	if err != nil {
		fmt.Printf("Warning: Could not fetch models: %v\n", err)
	} else if len(models) > 0 {
		fmt.Printf("Available models: ")
		for i, m := range models {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Print(m.ID)
		}
		fmt.Println()
	}

	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  /model <name>  - Change the LLM model")
	fmt.Println("  /hint <text>   - Add a memory hint")
	fmt.Println("  /session       - Show session info")
	fmt.Println("  /stream        - Toggle streaming mode")
	fmt.Println("  /quit          - Exit")
	fmt.Println()
	fmt.Println("Type your message and press Enter to chat.")
	fmt.Println("─────────────────────────────────────────")

	streaming := true
	scanner := bufio.NewScanner(os.Stdin)

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

		// Handle commands
		if strings.HasPrefix(input, "/") {
			if handleCommand(ctx, client, session.ID, prefs, input, &model, &streaming) {
				continue
			}
			if input == "/quit" {
				return nil
			}
		}

		// Make LLM completion
		fmt.Print("\nAssistant: ")

		req := athyr.CompletionRequest{
			Model: model,
			Messages: []athyr.Message{
				{Role: "system", Content: "You are a helpful assistant. Be concise."},
				{Role: "user", Content: input},
			},
			SessionID:     session.ID,
			IncludeMemory: true,
		}

		if streaming {
			err = client.CompleteStream(ctx, req, func(chunk athyr.StreamChunk) error {
				if chunk.Error != "" {
					return fmt.Errorf("stream error: %s", chunk.Error)
				}
				fmt.Print(chunk.Content)
				return nil
			})
			fmt.Println()
		} else {
			var resp *athyr.CompletionResponse
			resp, err = client.Complete(ctx, req)
			if err == nil {
				fmt.Println(resp.Content)
				fmt.Printf("  [%s via %s, %d tokens, %v]\n",
					resp.Model, resp.Backend, resp.Usage.TotalTokens, resp.Latency)
			}
		}

		if err != nil {
			fmt.Printf("\nError: %v\n", err)
		}
	}

	return nil
}

func handleCommand(
	ctx context.Context,
	client athyr.Agent,
	sessionID string,
	prefs athyr.KVBucket,
	input string,
	model *string,
	streaming *bool,
) bool {
	parts := strings.SplitN(input, " ", 2)
	cmd := parts[0]
	arg := ""
	if len(parts) > 1 {
		arg = parts[1]
	}

	switch cmd {
	case "/model":
		if arg == "" {
			fmt.Printf("Current model: %s\n", *model)
		} else {
			*model = arg
			_, _ = prefs.Put(ctx, "model", []byte(arg))
			fmt.Printf("Model changed to: %s (saved to preferences)\n", arg)
		}
		return true

	case "/hint":
		if arg == "" {
			fmt.Println("Usage: /hint <important information to remember>")
		} else {
			if err := client.AddHint(ctx, sessionID, arg); err != nil {
				fmt.Printf("Error adding hint: %v\n", err)
			} else {
				fmt.Println("Hint added to memory.")
			}
		}
		return true

	case "/session":
		session, err := client.GetSession(ctx, sessionID)
		if err != nil {
			fmt.Printf("Error getting session: %v\n", err)
		} else {
			fmt.Printf("Session ID: %s\n", session.ID)
			fmt.Printf("Messages: %d\n", len(session.Messages))
			fmt.Printf("Hints: %d\n", len(session.Hints))
			if session.Summary != "" {
				fmt.Printf("Summary: %s\n", truncate(session.Summary, 100))
			}
		}
		return true

	case "/stream":
		*streaming = !*streaming
		if *streaming {
			fmt.Println("Streaming mode: ON")
		} else {
			fmt.Println("Streaming mode: OFF")
		}
		return true

	case "/quit":
		return false

	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		return true
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
