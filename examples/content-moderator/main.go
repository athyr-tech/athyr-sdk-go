// Example: Content Moderator (FanOut Pattern)
//
// Demonstrates parallel agent execution with result aggregation.
// Three agents analyze content simultaneously, then aggregate scores.
//
// Usage:
//
//	# Start agents (each in separate terminal, or use &)
//	go run . -role=toxicity
//	go run . -role=spam
//	go run . -role=relevance
//
//	# Run orchestrator
//	go run . -role=orchestrator
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/athyr-tech/athyr-sdk-go/pkg/athyr"
	"github.com/athyr-tech/athyr-sdk-go/pkg/orchestration"
)

const addr = "localhost:9090"

// Subjects
const (
	SubjectToxicity  = "moderator.toxicity"
	SubjectSpam      = "moderator.spam"
	SubjectRelevance = "moderator.relevance"
)

// Request/Response types
type Request struct {
	Content string `json:"content"`
}

type Score struct {
	Score  float64 `json:"score"`
	Reason string  `json:"reason"`
}

func main() {
	role := flag.String("role", "orchestrator", "Role: orchestrator|toxicity|spam|relevance")
	flag.Parse()

	var err error
	switch *role {
	case "orchestrator":
		err = runOrchestrator()
	case "toxicity":
		err = runWorker("toxicity-analyzer", SubjectToxicity, toxicityHandler)
	case "spam":
		err = runWorker("spam-analyzer", SubjectSpam, spamHandler)
	case "relevance":
		err = runWorker("relevance-analyzer", SubjectRelevance, relevanceHandler)
	default:
		fmt.Fprintf(os.Stderr, "Unknown role: %s\n", *role)
		os.Exit(1)
	}

	if err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runWorker[Req, Resp any](name, subject string, handler athyr.Handler[Req, Resp]) error {
	server := athyr.NewServer(addr,
		athyr.WithAgentName(name),
		athyr.WithAgentDescription("Content moderation worker"),
	)
	athyr.Handle(server, subject, handler)
	fmt.Printf("%s ready on %s\n", name, subject)
	return server.Run(context.Background())
}

// ============================================================================
// Orchestrator - demonstrates FanOut pattern
// ============================================================================

func runOrchestrator() error {
	fmt.Println("Content Moderator - FanOut Pattern")
	fmt.Println("===================================")

	ctx := context.Background()
	agent, err := athyr.NewAgent(addr,
		athyr.WithAgentCard(athyr.AgentCard{Name: "content-moderator"}),
	)
	if err != nil {
		return err
	}
	if err := agent.Connect(ctx); err != nil {
		return err
	}
	defer agent.Close()

	// Build FanOut - this is what we're demonstrating
	fanout := orchestration.NewFanOut("content-moderation").
		Agent("toxicity", SubjectToxicity).
		Agent("spam", SubjectSpam).
		Agent("relevance", SubjectRelevance)

	// Test cases
	tests := []string{
		"Check out this amazing product! Buy now!",
		"I really enjoyed this article about Go programming.",
		"You are stupid and I hate everything you wrote.",
	}

	for i, content := range tests {
		fmt.Printf("Test %d: %q\n", i+1, content)

		input, _ := json.Marshal(Request{Content: content})
		trace, err := fanout.ExecuteWithTrace(ctx, agent, input, orchestration.AllMustSucceed())
		if err != nil {
			fmt.Printf("  Error: %v\n\n", err)
			continue
		}

		// Show results
		approved := true
		for name, at := range trace.Agents {
			var s Score
			json.Unmarshal(at.Output, &s)
			fmt.Printf("  %s: %.2f (%s)\n", name, s.Score, s.Reason)
			if s.Score > 0.7 {
				approved = false
			}
		}

		status := "APPROVED"
		if !approved {
			status = "FLAGGED"
		}
		fmt.Printf("  → %s (took %v)\n\n", status, trace.Duration)
	}

	return nil
}

// ============================================================================
// Handlers - simple mock analyzers
// ============================================================================

func toxicityHandler(_ athyr.Context, req Request) (Score, error) {
	toxic := []string{"hate", "stupid", "idiot", "terrible"}
	for _, word := range toxic {
		if strings.Contains(strings.ToLower(req.Content), word) {
			return Score{0.9, "toxic language detected"}, nil
		}
	}
	return Score{0.1, "no toxic content"}, nil
}

func spamHandler(_ athyr.Context, req Request) (Score, error) {
	spam := []string{"buy now", "click here", "amazing deal", "limited time"}
	for _, phrase := range spam {
		if strings.Contains(strings.ToLower(req.Content), phrase) {
			return Score{0.85, "spam patterns detected"}, nil
		}
	}
	return Score{0.1, "no spam detected"}, nil
}

func relevanceHandler(_ athyr.Context, req Request) (Score, error) {
	if len(req.Content) < 20 {
		return Score{0.6, "content too short"}, nil
	}
	return Score{0.2, "content appears relevant"}, nil
}
