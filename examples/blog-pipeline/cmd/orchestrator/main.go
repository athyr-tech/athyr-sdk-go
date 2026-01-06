// Orchestrator - Coordinates the blog pipeline stages.
//
// This is the pipeline controller that sends requests to each agent
// in sequence: Outline → Draft → Edit → SEO.
//
// The orchestrator doesn't process content itself - it coordinates
// the distributed agents via Athyr messaging.
//
// Usage:
//
//	orchestrator --athyr=localhost:9090 --topic="Your Blog Topic" --output=blog.md
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/athyr-tech/athyr-sdk-go/examples/blog-pipeline/internal/types"
	"github.com/athyr-tech/athyr-sdk-go/pkg/athyr"
	"github.com/athyr-tech/athyr-sdk-go/pkg/orchestration"
)

var (
	athyrAddr = flag.String("athyr", types.DefaultAthyrAddr(), "Athyr server address")
	topic     = flag.String("topic", "", "Topic for the blog post (required)")
	output    = flag.String("output", types.DefaultOutputPath(), "Output file")
)

func main() {
	flag.Parse()

	if *topic == "" {
		fmt.Fprintln(os.Stderr, "Error: --topic is required")
		fmt.Fprintln(os.Stderr, "Usage: orchestrator --topic \"Your Blog Topic\"")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n\nInterrupted, shutting down...")
		cancel()
	}()

	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	printHeader()

	// Connect to Athyr
	agent, err := athyr.NewAgent(*athyrAddr,
		athyr.WithAgentCard(athyr.AgentCard{
			Name:         "blog-orchestrator",
			Description:  "Coordinates the blog post creation pipeline",
			Version:      "1.0.0",
			Capabilities: []string{"orchestration", "pipeline"},
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	fmt.Printf("⚡ Connecting to Athyr at %s... ", *athyrAddr)
	if err := agent.Connect(ctx); err != nil {
		fmt.Println("✗")
		return fmt.Errorf("failed to connect: %w", err)
	}
	fmt.Printf("✓ (Agent: %s)\n", agent.AgentID())
	defer agent.Close()

	fmt.Println()
	fmt.Printf("📝 Topic: %s\n", *topic)
	fmt.Println()
	fmt.Println("─────────────────────────────────────────")

	// Build the pipeline: Outline → Draft → Edit → SEO
	pipeline := orchestration.NewPipeline("blog-creation").
		Step("outline", types.SubjectOutline).
		Step("draft", types.SubjectDraft).
		Step("edit", types.SubjectEdit).
		Step("seo", types.SubjectSEO)

	// Prepare initial input
	input := types.PipelineData{Topic: *topic}
	inputBytes, _ := json.Marshal(input)

	// Execute with trace for detailed progress
	startTime := time.Now()
	trace, err := pipeline.ExecuteWithTrace(ctx, agent, inputBytes)
	totalDuration := time.Since(startTime)

	if err != nil {
		return fmt.Errorf("pipeline failed: %w", err)
	}

	// Extract and save final content
	var finalData types.PipelineData
	if err := json.Unmarshal(trace.Output(), &finalData); err != nil {
		return fmt.Errorf("failed to parse final output: %w", err)
	}

	printSummary(trace, totalDuration, finalData)

	// Save output
	if err := os.WriteFile(*output, []byte(finalData.Final), 0644); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	fmt.Printf("   Output saved: %s\n", *output)

	return nil
}

func printHeader() {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════╗")
	fmt.Println("║      Blog Pipeline Orchestrator                  ║")
	fmt.Println("║                                                  ║")
	fmt.Println("║   📋 Outline → ✍️ Draft → 🔍 Edit → 🔎 SEO       ║")
	fmt.Println("╚══════════════════════════════════════════════════╝")
	fmt.Println()
}

func printSummary(trace *orchestration.PipelineTrace, duration time.Duration, data types.PipelineData) {
	fmt.Println()
	fmt.Println("─────────────────────────────────────────")
	fmt.Println()
	fmt.Println("✅ Pipeline Complete!")
	fmt.Println()

	for _, step := range trace.Steps {
		status := "✓"
		if step.Error != nil {
			status = "✗"
		}
		fmt.Printf("   %s %-10s %v\n", status, step.Name, step.Duration.Round(time.Millisecond))
	}

	fmt.Println()
	fmt.Printf("   Total time:   %v\n", duration.Round(time.Millisecond))
	fmt.Printf("   Total tokens: ~%d\n", data.TotalTokens)
}
