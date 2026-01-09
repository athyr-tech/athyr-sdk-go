// Example: Debate Club (GroupChat Pattern)
//
// Demonstrates multi-agent collaborative discussion.
// Three agents debate a topic with structured turn-taking:
//   - Pro agent (argues in favor)
//   - Con agent (argues against)
//   - Moderator (summarizes at the end)
//
// Usage:
//
//	# Terminal 1: Start each agent (or run all in background)
//	go run ./examples/debate-club -role=pro &
//	go run ./examples/debate-club -role=con &
//	go run ./examples/debate-club -role=moderator &
//
//	# Terminal 2: Run the orchestrator
//	go run ./examples/debate-club -role=orchestrator
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
	SubjectPro       = "debate.pro"
	SubjectCon       = "debate.con"
	SubjectModerator = "debate.moderator"
)

func main() {
	role := flag.String("role", "orchestrator", "Role: orchestrator, pro, con, or moderator")
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
	case "pro":
		err = runDebater(ctx, "pro", SubjectPro, generateProArgument)
	case "con":
		err = runDebater(ctx, "con", SubjectCon, generateConArgument)
	case "moderator":
		err = runDebater(ctx, "moderator", SubjectModerator, generateSummary)
	default:
		fmt.Fprintf(os.Stderr, "Unknown role: %s\n", *role)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// runOrchestrator demonstrates the GroupChat pattern
func runOrchestrator(ctx context.Context) error {
	fmt.Println("Debate Club - GroupChat Pattern Demo")
	fmt.Println("====================================")

	// Connect to Athyr
	agent, err := athyr.NewAgent(athyrAddr,
		athyr.WithAgentCard(athyr.AgentCard{
			Name:        "debate-orchestrator",
			Description: "Orchestrates multi-agent debates",
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

	fmt.Println("Connected! Starting debate...")
	fmt.Println()

	// Build the GroupChat with custom turn order
	// Pro speaks, then Con responds, repeat, then Moderator summarizes
	chat := orchestration.NewGroupChat("debate-club").
		Participant("pro", SubjectPro).
		Participant("con", SubjectCon).
		Participant("moderator", SubjectModerator).
		Manager(orchestration.FuncManager(debateTurnManager)).
		MaxRounds(5) // Pro, Con, Pro, Con, Moderator

	// Debate topic
	topic := "AI will create more jobs than it eliminates"

	fmt.Printf("Topic: %q\n", topic)
	fmt.Println()
	fmt.Println("--- Debate Begins ---")
	fmt.Println()

	// Run the discussion
	trace, err := chat.DiscussWithTrace(ctx, agent, []byte(topic))
	if err != nil {
		return fmt.Errorf("debate failed: %w", err)
	}

	// Display the transcript
	for _, msg := range trace.Messages {
		fmt.Printf("[%s]: %s\n\n", msg.Speaker, msg.Content)
	}

	fmt.Println("--- Debate Ends ---")
	fmt.Printf("\nRounds: %d | Duration: %v\n", trace.RoundsCompleted, trace.Duration)

	return nil
}

// debateTurnManager controls who speaks when
// Order: pro, con, pro, con, moderator (then done)
func debateTurnManager(participants []string, messages []orchestration.Message) string {
	round := len(messages)

	switch round {
	case 0:
		return "pro" // Opening argument
	case 1:
		return "con" // Response
	case 2:
		return "pro" // Rebuttal
	case 3:
		return "con" // Counter
	case 4:
		return "moderator" // Summary
	default:
		return "" // End discussion
	}
}

// runDebater starts a debate participant agent
func runDebater(ctx context.Context, name, subject string, respond func(string, []orchestration.Message) string) error {
	fmt.Printf("Starting %s agent...\n", name)

	agent, err := athyr.NewAgent(athyrAddr,
		athyr.WithAgentCard(athyr.AgentCard{
			Name:        fmt.Sprintf("debate-%s", name),
			Description: fmt.Sprintf("Debate participant: %s", name),
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

	// Subscribe to debate requests
	_, err = agent.Subscribe(ctx, subject, func(msg athyr.SubscribeMessage) {
		var req orchestration.ChatRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			return
		}

		// Generate response based on role and history
		content := respond(req.Topic, req.Messages)
		resp, _ := json.Marshal(orchestration.ChatResponse{Content: content})

		if msg.Reply != "" {
			_ = agent.Publish(ctx, msg.Reply, resp)
		}
	})
	if err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}

	fmt.Printf("%s agent ready on %s\n", name, subject)
	<-ctx.Done()
	return nil
}

// Mock argument generators (replace with LLM calls in production)

func generateProArgument(topic string, history []orchestration.Message) string {
	round := len(history)

	switch round {
	case 0:
		return "AI will revolutionize industries and create entirely new job categories. " +
			"Just as the industrial revolution created more jobs than it displaced, " +
			"AI will spawn new roles in AI training, oversight, and human-AI collaboration."
	case 2:
		return "History shows technology creates jobs. The ATM didn't eliminate bank tellers - " +
			"it enabled banks to open more branches. Similarly, AI handles routine tasks " +
			"while humans focus on creative, strategic, and interpersonal work."
	default:
		return "AI is a tool that amplifies human potential, not replaces it."
	}
}

func generateConArgument(topic string, history []orchestration.Message) string {
	round := len(history)

	switch round {
	case 1:
		return "Unlike previous revolutions, AI directly targets cognitive work. " +
			"Automation previously replaced physical labor, but AI can write, analyze, " +
			"and create - threatening white-collar jobs that were once considered safe."
	case 3:
		return "The pace of AI advancement outstrips human adaptation. Previous transitions " +
			"took generations; AI disruption is happening in years. Many workers won't have " +
			"time to reskill before their jobs become obsolete."
	default:
		return "The risk of mass displacement is real and requires proactive policy intervention."
	}
}

func generateSummary(topic string, history []orchestration.Message) string {
	proPoints := 0
	conPoints := 0

	for _, msg := range history {
		if msg.Speaker == "pro" {
			proPoints++
		} else if msg.Speaker == "con" {
			conPoints++
		}
	}

	return fmt.Sprintf("MODERATOR SUMMARY: Both sides presented compelling arguments. "+
		"The Pro side (%d points) emphasized historical precedent and new job creation. "+
		"The Con side (%d points) highlighted AI's unique threat to cognitive work and rapid pace of change. "+
		"The truth likely lies in between - AI will transform work, requiring proactive adaptation.",
		proPoints, conPoints)
}
