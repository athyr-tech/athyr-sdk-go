// Example: GroupChat Pattern
//
// Demonstrates multi-agent collaborative discussion with turn management.
// Participants take turns contributing to a shared conversation.
// Use case: Debate simulation with pro, con, and moderator agents.
//
// Usage:
//
//	# Start agents
//	go run . -role=pro
//	go run . -role=con
//	go run . -role=moderator
//
//	# Run orchestrator
//	go run . -role=orchestrator
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/athyr-tech/athyr-sdk-go/pkg/athyr"
	"github.com/athyr-tech/athyr-sdk-go/pkg/orchestration"
)

const addr = "localhost:9090"

// Subjects
const (
	SubjectPro       = "debate.pro"
	SubjectCon       = "debate.con"
	SubjectModerator = "debate.moderator"
)

func main() {
	role := flag.String("role", "orchestrator", "Role: orchestrator|pro|con|moderator")
	flag.Parse()

	var err error
	switch *role {
	case "orchestrator":
		err = runOrchestrator()
	case "pro":
		err = runWorker("debate-pro", SubjectPro, proHandler)
	case "con":
		err = runWorker("debate-con", SubjectCon, conHandler)
	case "moderator":
		err = runWorker("debate-moderator", SubjectModerator, moderatorHandler)
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
		athyr.WithAgentDescription("Debate club participant"),
	)
	athyr.Handle(server, subject, handler)
	fmt.Printf("%s ready on %s\n", name, subject)
	return server.Run(context.Background())
}

// ============================================================================
// Orchestrator - demonstrates GroupChat pattern
// ============================================================================

func runOrchestrator() error {
	fmt.Println("Debate Club - GroupChat Pattern")
	fmt.Println("================================")

	ctx := context.Background()
	agent, err := athyr.NewAgent(addr,
		athyr.WithAgentCard(athyr.AgentCard{Name: "debate-orchestrator"}),
	)
	if err != nil {
		return err
	}
	if err := agent.Connect(ctx); err != nil {
		return err
	}
	defer agent.Close()

	// Build GroupChat - this is what we're demonstrating
	chat := orchestration.NewGroupChat("debate-club").
		Participant("pro", SubjectPro).
		Participant("con", SubjectCon).
		Participant("moderator", SubjectModerator).
		Manager(orchestration.FuncManager(turnManager)).
		MaxRounds(5)

	topic := "AI will create more jobs than it eliminates"
	fmt.Printf("\nTopic: %q\n\n", topic)

	// Run the debate
	trace, err := chat.DiscussWithTrace(ctx, agent, []byte(topic))
	if err != nil {
		return err
	}

	// Print transcript
	for _, msg := range trace.Messages {
		fmt.Printf("[%s]: %s\n\n", msg.Speaker, msg.Content)
	}

	fmt.Printf("--- %d rounds, %v ---\n", trace.RoundsCompleted, trace.Duration)
	return nil
}

// turnManager controls debate flow: pro, con, pro, con, moderator
func turnManager(_ []string, messages []orchestration.Message) string {
	switch len(messages) {
	case 0:
		return "pro"
	case 1:
		return "con"
	case 2:
		return "pro"
	case 3:
		return "con"
	case 4:
		return "moderator"
	default:
		return "" // End
	}
}

// ============================================================================
// Handlers
// ============================================================================

func proHandler(_ athyr.Context, req orchestration.ChatRequest) (orchestration.ChatResponse, error) {
	arguments := []string{
		"AI will revolutionize industries and create entirely new job categories. " +
			"Just as the industrial revolution created more jobs than it displaced, " +
			"AI will spawn new roles in AI training, oversight, and human-AI collaboration.",
		"History shows technology creates jobs. The ATM didn't eliminate bank tellers - " +
			"it enabled banks to open more branches. AI handles routine tasks while " +
			"humans focus on creative, strategic, and interpersonal work.",
	}

	idx := len(req.Messages) / 2
	if idx >= len(arguments) {
		idx = len(arguments) - 1
	}
	return orchestration.ChatResponse{Content: arguments[idx]}, nil
}

func conHandler(_ athyr.Context, req orchestration.ChatRequest) (orchestration.ChatResponse, error) {
	arguments := []string{
		"Unlike previous revolutions, AI directly targets cognitive work. " +
			"Automation previously replaced physical labor, but AI can write, analyze, " +
			"and create - threatening white-collar jobs that were once considered safe.",
		"The pace of AI advancement outstrips human adaptation. Previous transitions " +
			"took generations; AI disruption is happening in years. Many workers won't " +
			"have time to reskill before their jobs become obsolete.",
	}

	idx := len(req.Messages) / 2
	if idx >= len(arguments) {
		idx = len(arguments) - 1
	}
	return orchestration.ChatResponse{Content: arguments[idx]}, nil
}

func moderatorHandler(_ athyr.Context, req orchestration.ChatRequest) (orchestration.ChatResponse, error) {
	proCount, conCount := 0, 0
	for _, m := range req.Messages {
		if m.Speaker == "pro" {
			proCount++
		} else if m.Speaker == "con" {
			conCount++
		}
	}

	summary := fmt.Sprintf("SUMMARY: Both sides made %d arguments each. "+
		"Pro emphasized historical precedent and new job creation. "+
		"Con highlighted AI's unique threat to cognitive work and rapid pace. "+
		"The truth likely requires proactive adaptation policies.",
		proCount)

	return orchestration.ChatResponse{Content: summary}, nil
}
