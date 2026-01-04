// Example: Echo Agent
//
// A minimal agent that demonstrates pub/sub messaging:
// - Subscribes to a subject
// - Echoes messages back with a prefix
// - Uses request/reply pattern
//
// Usage:
//
//	# Start Athyr server first
//	go run ./cmd/athyr serve
//
//	# Run the echo agent
//	go run ./examples/echo-agent
//
//	# Test with another client or use the HTTP API:
//	curl -X POST http://localhost:8080/v1/request \
//	  -H "Content-Type: application/json" \
//	  -d '{"subject": "echo.request", "data": "SGVsbG8gV29ybGQ="}'
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	sdk "github.com/athyr-tech/athyr-sdk-go/pkg/agent"
)

const athyrAddr = "localhost:9090"

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nShutting down...")
		cancel()
	}()

	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	fmt.Println("Echo Agent - Athyr Pub/Sub Example")
	fmt.Println("===================================")

	// Create SDK client
	client, err := sdk.NewAgent(athyrAddr,
		sdk.WithAgentCard(sdk.AgentCard{
			Name:         "echo-agent",
			Description:  "Echoes messages back with a prefix",
			Version:      "1.0.0",
			Capabilities: []string{"echo"},
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Connect to Athyr
	fmt.Print("Connecting... ")
	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	fmt.Printf("Connected! (Agent ID: %s)\n", client.AgentID())
	defer func() { _ = client.Close() }()

	// Subscribe to echo requests
	fmt.Println("Subscribing to 'echo.>' ...")

	messageCount := 0
	sub, err := client.Subscribe(ctx, "echo.>", func(msg sdk.SubscribeMessage) {
		messageCount++
		fmt.Printf("\n[%s] Received on %s: %s\n",
			time.Now().Format("15:04:05"),
			msg.Subject,
			string(msg.Data))

		// If there's a reply subject, respond
		if msg.Reply != "" {
			response := fmt.Sprintf("ECHO: %s", string(msg.Data))
			if err := client.Publish(ctx, msg.Reply, []byte(response)); err != nil {
				fmt.Printf("  Error replying: %v\n", err)
			} else {
				fmt.Printf("  Replied: %s\n", response)
			}
		}
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}
	defer func() { _ = sub.Unsubscribe() }()

	fmt.Println("Listening for messages on 'echo.>' ...")
	fmt.Println("Press Ctrl+C to exit")
	fmt.Println()

	// Also demonstrate publishing
	go func() {
		time.Sleep(2 * time.Second)
		fmt.Println("Publishing test message to 'echo.test' ...")
		data, _ := json.Marshal(map[string]string{
			"message": "Hello from echo agent!",
			"time":    time.Now().Format(time.RFC3339),
		})
		if err := client.Publish(ctx, "echo.test", data); err != nil {
			fmt.Printf("Error publishing: %v\n", err)
		}
	}()

	// Wait for shutdown
	<-ctx.Done()

	fmt.Printf("\nProcessed %d messages\n", messageCount)
	return nil
}
