// Example: Content Moderator (FanOut Pattern)
//
// Demonstrates parallel agent execution with result aggregation.
// Three specialized agents analyze content simultaneously:
//   - Toxicity checker
//   - Spam detector
//   - Relevance scorer
//
// Usage:
//
//	# Terminal 1: Start each agent (or run all in background)
//	go run ./examples/content-moderator -role=toxicity &
//	go run ./examples/content-moderator -role=spam &
//	go run ./examples/content-moderator -role=relevance &
//
//	# Terminal 2: Run the orchestrator
//	go run ./examples/content-moderator -role=orchestrator
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/athyr-tech/athyr-sdk-go/pkg/athyr"
	"github.com/athyr-tech/athyr-sdk-go/pkg/orchestration"
)

const athyrAddr = "localhost:9090"

// Subjects for each agent
const (
	SubjectToxicity  = "moderator.toxicity"
	SubjectSpam      = "moderator.spam"
	SubjectRelevance = "moderator.relevance"
)

// ModerationRequest is the input to each analyzer
type ModerationRequest struct {
	Content string `json:"content"`
}

// ModerationScore is returned by each analyzer
type ModerationScore struct {
	Score  float64 `json:"score"`  // 0.0 (safe) to 1.0 (flagged)
	Reason string  `json:"reason"` // Brief explanation
}

// ModerationResult is the aggregated result
type ModerationResult struct {
	Approved bool               `json:"approved"`
	Scores   map[string]float64 `json:"scores"`
	Reasons  map[string]string  `json:"reasons"`
}

func main() {
	role := flag.String("role", "orchestrator", "Role: orchestrator, toxicity, spam, or relevance")
	flag.Parse()

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

	var err error
	switch *role {
	case "orchestrator":
		err = runOrchestrator(ctx)
	case "toxicity":
		err = runAgent(ctx, "toxicity", SubjectToxicity, analyzeToxicity)
	case "spam":
		err = runAgent(ctx, "spam", SubjectSpam, analyzeSpam)
	case "relevance":
		err = runAgent(ctx, "relevance", SubjectRelevance, analyzeRelevance)
	default:
		fmt.Fprintf(os.Stderr, "Unknown role: %s\n", *role)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// runOrchestrator demonstrates the FanOut pattern
func runOrchestrator(ctx context.Context) error {
	fmt.Println("Content Moderator - FanOut Pattern Demo")
	fmt.Println("=======================================")

	// Connect to Athyr
	agent, err := athyr.NewAgent(athyrAddr,
		athyr.WithAgentCard(athyr.AgentCard{
			Name:        "content-moderator",
			Description: "Orchestrates content moderation analysis",
			Version:     "1.0.0",
		}),
	)
	if err != nil {
		return fmt.Errorf("create agent: %w", err)
	}

	if err := agent.Connect(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer agent.Close()

	fmt.Println("Connected! Running moderation analysis...")
	fmt.Println()

	// Build the FanOut orchestrator
	fanout := orchestration.NewFanOut("content-moderation").
		Agent("toxicity", SubjectToxicity).
		Agent("spam", SubjectSpam).
		Agent("relevance", SubjectRelevance)

	// Test content to moderate
	testCases := []string{
		"Check out this amazing product! Buy now!",
		"I really enjoyed this article about Go programming.",
		"You are stupid and I hate everything you wrote.",
	}

	for i, content := range testCases {
		fmt.Printf("--- Test %d ---\n", i+1)
		fmt.Printf("Content: %q\n\n", content)

		// Prepare input
		input, _ := json.Marshal(ModerationRequest{Content: content})

		// Execute fan-out (all agents run in parallel)
		trace, err := fanout.ExecuteWithTrace(ctx, agent, input, orchestration.AllMustSucceed())
		if err != nil {
			fmt.Printf("Error: %v\n\n", err)
			continue
		}

		// Parse and display results
		result := aggregateResults(trace)
		fmt.Printf("Results:\n")
		fmt.Printf("  Toxicity:  %.2f (%s)\n", result.Scores["toxicity"], result.Reasons["toxicity"])
		fmt.Printf("  Spam:      %.2f (%s)\n", result.Scores["spam"], result.Reasons["spam"])
		fmt.Printf("  Relevance: %.2f (%s)\n", result.Scores["relevance"], result.Reasons["relevance"])
		fmt.Printf("\n  Decision: %s\n", decisionString(result.Approved))
		fmt.Printf("  Duration: %v\n\n", trace.Duration)
	}

	return nil
}

// aggregateResults combines scores from all agents
func aggregateResults(trace *orchestration.FanOutTrace) ModerationResult {
	result := ModerationResult{
		Approved: true,
		Scores:   make(map[string]float64),
		Reasons:  make(map[string]string),
	}

	for name, agentTrace := range trace.Agents {
		var score ModerationScore
		if err := json.Unmarshal(agentTrace.Output, &score); err != nil {
			continue
		}

		result.Scores[name] = score.Score
		result.Reasons[name] = score.Reason

		// Flag if any score exceeds threshold (0.7)
		if score.Score > 0.7 {
			result.Approved = false
		}
	}

	return result
}

func decisionString(approved bool) string {
	if approved {
		return "APPROVED"
	}
	return "FLAGGED FOR REVIEW"
}

// runAgent starts a simple analyzer agent
func runAgent(ctx context.Context, name, subject string, analyze func(string) ModerationScore) error {
	fmt.Printf("Starting %s analyzer...\n", name)

	agent, err := athyr.NewAgent(athyrAddr,
		athyr.WithAgentCard(athyr.AgentCard{
			Name:        fmt.Sprintf("%s-analyzer", name),
			Description: fmt.Sprintf("Analyzes content for %s", name),
			Version:     "1.0.0",
		}),
	)
	if err != nil {
		return fmt.Errorf("create agent: %w", err)
	}

	if err := agent.Connect(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer agent.Close()

	// Subscribe to requests
	_, err = agent.Subscribe(ctx, subject, func(msg athyr.SubscribeMessage) {
		var req ModerationRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			return
		}

		// Analyze and respond
		score := analyze(req.Content)
		resp, _ := json.Marshal(score)

		if msg.Reply != "" {
			_ = agent.Publish(ctx, msg.Reply, resp)
		}
	})
	if err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}

	fmt.Printf("%s analyzer ready on %s\n", name, subject)
	<-ctx.Done()
	return nil
}

// Simple mock analyzers (replace with real LLM calls in production)

func analyzeToxicity(content string) ModerationScore {
	// Mock: check for negative words
	toxicWords := []string{"hate", "stupid", "idiot", "terrible"}
	for _, word := range toxicWords {
		if contains(content, word) {
			return ModerationScore{Score: 0.9, Reason: "Contains toxic language"}
		}
	}
	return ModerationScore{Score: 0.1, Reason: "No toxic content detected"}
}

func analyzeSpam(content string) ModerationScore {
	// Mock: check for spam patterns
	spamWords := []string{"buy now", "click here", "amazing deal", "limited time"}
	for _, word := range spamWords {
		if contains(content, word) {
			return ModerationScore{Score: 0.85, Reason: "Spam patterns detected"}
		}
	}
	return ModerationScore{Score: 0.1, Reason: "No spam detected"}
}

func analyzeRelevance(content string) ModerationScore {
	// Mock: check content length and keywords
	if len(content) < 20 {
		return ModerationScore{Score: 0.6, Reason: "Content too short"}
	}
	return ModerationScore{Score: 0.2, Reason: "Content appears relevant"}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsLower(s, substr))
}

func containsLower(s, substr string) bool {
	s = toLower(s)
	substr = toLower(substr)
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}
