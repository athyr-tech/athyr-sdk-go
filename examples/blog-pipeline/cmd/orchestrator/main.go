// Orchestrator CLI - Interactive interface for the blog pipeline.
//
// This is a thin CLI wrapper around the pipeline package.
// See internal/pipeline for the core orchestration logic.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/athyr-tech/athyr-sdk-go/examples/blog-pipeline/internal/pipeline"
	"github.com/athyr-tech/athyr-sdk-go/examples/blog-pipeline/internal/run"
)

var athyrAddr = flag.String("athyr", "localhost:9090", "Athyr server address")

func main() {
	flag.Parse()

	err := run.UntilSignal(func(ctx context.Context) error {
		return runCLI(ctx, *athyrAddr)
	})

	if err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runCLI(ctx context.Context, athyrAddr string) error {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("Blog Pipeline")
	fmt.Println("Type a topic to generate a blog post, or 'quit' to exit.")
	fmt.Println()

	for {
		fmt.Print("topic> ")
		if !scanner.Scan() {
			break
		}

		topic := strings.TrimSpace(scanner.Text())
		if topic == "" {
			continue
		}
		if topic == "quit" || topic == "exit" {
			fmt.Println("bye")
			break
		}

		result, err := pipeline.Run(ctx, athyrAddr, topic)
		if err != nil {
			fmt.Printf("error: %v\n\n", err)
			continue
		}

		printResult(result)
	}

	return scanner.Err()
}

func printResult(r *pipeline.Result) {
	fmt.Println()

	// Print each step with truncated output
	for i, step := range r.Steps {
		fmt.Printf("[%d/%d] %s (%v)\n", i+1, len(r.Steps), step.Name, step.Duration.Round(time.Millisecond))
		fmt.Println("---")
		fmt.Println(run.Truncate(step.Output, 500))
		fmt.Println("---")
		fmt.Println()
	}

	// Summary
	fmt.Printf("complete: %v, %d tokens\n", r.Duration.Round(time.Millisecond), r.TotalTokens)
	fmt.Println()
}
