// Example: Tool Agent
//
// Demonstrates LLM tool calling with the Athyr SDK.
//
// Files:
//   - main.go  : Entry point and chat loop
//   - tools.go : Tool definitions and implementations
//   - loop.go  : The agentic tool-calling loop
//
// Usage:
//
//	go run ./examples/tool-agent
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
	athyrAddr = "localhost:9090"
	model     = "qwen3:4b"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	// Connect
	agent, err := athyr.NewAgent(athyrAddr,
		athyr.WithAgentCard(athyr.AgentCard{Name: "tool-agent"}),
		athyr.WithHeartbeatInterval(30*time.Second),
	)
	if err != nil {
		return err
	}
	if err := agent.Connect(ctx); err != nil {
		return err
	}
	defer agent.Close()

	fmt.Println("Tool Agent Example")
	fmt.Println("==================")
	fmt.Printf("Connected: %s\n\n", agent.AgentID())
	fmt.Println("Tools: get_weather, add, multiply")
	fmt.Println("Type /quit to exit")

	// Chat loop
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("You: ")
		if !scanner.Scan() {
			return nil
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "/quit" {
			return nil
		}

		select {
		case <-ctx.Done():
			return nil
		default:
		}

		response, err := RunToolLoop(ctx, agent, model, input)
		if err != nil {
			fmt.Printf("Error: %v\n\n", err)
			continue
		}
		fmt.Printf("Assistant: %s\n\n", response)
	}
}
