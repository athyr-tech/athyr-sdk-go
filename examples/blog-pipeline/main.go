// Example: Blog Pipeline
//
// Demonstrates the Pipeline orchestration pattern with a blog post creation workflow.
// See README.md for detailed documentation.
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

	"github.com/athyr-tech/athyr-sdk-go/pkg/athyr"
	"github.com/athyr-tech/athyr-sdk-go/pkg/orchestration"
)

var (
	topic     = flag.String("topic", "", "Topic for the blog post (required)")
	model     = flag.String("model", "qwen3:4b", "LLM model to use")
	output    = flag.String("output", "blog-post.md", "Output file")
	athyrAddr = flag.String("athyr", "localhost:9090", "Athyr server address")
)

func main() {
	flag.Parse()

	if *topic == "" {
		fmt.Fprintln(os.Stderr, "Error: --topic is required")
		fmt.Fprintln(os.Stderr, "Usage: blog-pipeline --topic \"Your Blog Topic\"")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
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
	agent, err := connectToAthyr(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = agent.Close() }()

	// Register all pipeline stages as handlers
	if err := registerStages(ctx, agent); err != nil {
		return err
	}

	// Small delay to ensure subscriptions are ready
	time.Sleep(100 * time.Millisecond)

	// Execute the pipeline
	return executePipeline(ctx, agent)
}

// connectToAthyr establishes connection to the Athyr server.
func connectToAthyr(ctx context.Context) (athyr.Agent, error) {
	agent, err := athyr.NewAgent(*athyrAddr,
		athyr.WithAgentCard(athyr.AgentCard{
			Name:         "blog-pipeline-demo",
			Description:  "Blog post creation pipeline demonstrating orchestration patterns",
			Version:      "1.0.0",
			Capabilities: []string{"orchestration", "blog", "pipeline"},
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	fmt.Printf("⚡ Connecting to Athyr at %s... ", *athyrAddr)
	if err := agent.Connect(ctx); err != nil {
		fmt.Println("✗")
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	fmt.Printf("✓ (Agent: %s)\n", agent.AgentID())

	return agent, nil
}

// registerStages registers all pipeline stage handlers with Athyr.
// Each stage is a typed handler that processes PipelineData.
func registerStages(ctx context.Context, agent athyr.Agent) error {
	fmt.Print("⚡ Registering pipeline stages... ")

	// Define all stages with their handlers
	stages := []struct {
		subject string
		handler athyr.Handler[PipelineData, PipelineData]
	}{
		{SubjectOutline, OutlineHandler(*model)},
		{SubjectDraft, DraftHandler(*model)},
		{SubjectEdit, EditHandler(*model)},
		{SubjectSEO, SEOHandler(*model)},
	}

	// Register each stage
	for _, stage := range stages {
		if err := registerHandler(ctx, agent, stage.subject, stage.handler); err != nil {
			fmt.Println("✗")
			return fmt.Errorf("failed to register %s: %w", stage.subject, err)
		}
	}

	fmt.Println("✓")
	return nil
}

// registerHandler wraps a typed handler and registers it as a subscription.
func registerHandler(ctx context.Context, agent athyr.Agent, subject string, handler athyr.Handler[PipelineData, PipelineData]) error {
	// Use the Service abstraction to build the handler
	svc := athyr.NewService(subject, handler)
	builtHandler := svc.BuildHandler(nil)

	// Subscribe with a message handler that bridges to our typed handler
	_, err := agent.Subscribe(ctx, subject, func(msg athyr.SubscribeMessage) {
		svcCtx := &handlerContext{
			Context: ctx,
			agent:   agent,
			subject: msg.Subject,
			reply:   msg.Reply,
		}

		resp, err := builtHandler(svcCtx, msg.Data)
		if msg.Reply != "" {
			var respData []byte
			if err != nil {
				respData = formatError(err)
			} else {
				respData = resp
			}
			_ = agent.Publish(ctx, msg.Reply, respData)
		}
	})

	return err
}

// executePipeline creates and runs the blog creation pipeline.
func executePipeline(ctx context.Context, agent athyr.Agent) error {
	fmt.Println()
	fmt.Printf("📝 Topic: %s\n", *topic)
	fmt.Printf("🤖 Model: %s\n", *model)
	fmt.Println()
	fmt.Println("─────────────────────────────────────────")

	// Build the pipeline: Outline → Draft → Edit → SEO
	pipeline := orchestration.NewPipeline("blog-creation").
		Step("outline", SubjectOutline).
		Step("draft", SubjectDraft).
		Step("edit", SubjectEdit).
		Step("seo", SubjectSEO)

	// Prepare initial input
	input := PipelineData{Topic: *topic}
	inputBytes, _ := json.Marshal(input)

	// Execute with trace for detailed progress
	startTime := time.Now()
	trace, err := pipeline.ExecuteWithTrace(ctx, agent, inputBytes)
	totalDuration := time.Since(startTime)

	if err != nil {
		return fmt.Errorf("pipeline failed: %w", err)
	}

	// Extract and save final content
	var finalData PipelineData
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

// printHeader displays the demo banner.
func printHeader() {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════╗")
	fmt.Println("║         Blog Pipeline - Orchestration Demo       ║")
	fmt.Println("║                                                  ║")
	fmt.Println("║   📋 Outline → ✍️ Draft → 🔍 Edit → 🔎 SEO       ║")
	fmt.Println("╚══════════════════════════════════════════════════╝")
	fmt.Println()
}

// printSummary displays execution results.
func printSummary(trace *orchestration.PipelineTrace, duration time.Duration, data PipelineData) {
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

// handlerContext implements athyr.Context for message handlers.
type handlerContext struct {
	context.Context
	agent   athyr.Agent
	subject string
	reply   string
}

func (c *handlerContext) Agent() athyr.Agent      { return c.agent }
func (c *handlerContext) Subject() string       { return c.subject }
func (c *handlerContext) ReplySubject() string  { return c.reply }

// formatError creates a JSON error response.
func formatError(err error) []byte {
	resp := struct {
		Error string `json:"error"`
	}{Error: err.Error()}
	data, _ := json.Marshal(resp)
	return data
}
