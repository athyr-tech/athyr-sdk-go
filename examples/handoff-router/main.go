// Example: HandoffRouter Pattern
//
// Demonstrates dynamic request routing based on classification.
// A triage agent analyzes input and routes to the appropriate specialist.
// Use case: Intent classification routing to weather, calculator, or joke agents.
//
// Usage:
//
//	# Start agents
//	go run . -role=router
//	go run . -role=weather
//	go run . -role=calculator
//	go run . -role=joke
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
	"strconv"
	"strings"

	"github.com/athyr-tech/athyr-sdk-go/pkg/athyr"
	"github.com/athyr-tech/athyr-sdk-go/pkg/orchestration"
)

const addr = "localhost:9090"

// Subjects
const (
	SubjectRouter     = "intent.router"
	SubjectWeather    = "intent.weather"
	SubjectCalculator = "intent.calculator"
	SubjectJoke       = "intent.joke"
)

// Types
type Query struct {
	Message string `json:"message"`
}

type Response struct {
	Answer string `json:"answer"`
}

func main() {
	role := flag.String("role", "orchestrator", "Role: orchestrator|router|weather|calculator|joke")
	flag.Parse()

	var err error
	switch *role {
	case "orchestrator":
		err = runOrchestrator()
	case "router":
		err = runWorker("intent-router", SubjectRouter, routerHandler)
	case "weather":
		err = runWorker("weather-agent", SubjectWeather, weatherHandler)
	case "calculator":
		err = runWorker("calculator-agent", SubjectCalculator, calculatorHandler)
	case "joke":
		err = runWorker("joke-agent", SubjectJoke, jokeHandler)
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
		athyr.WithAgentDescription("Intent classifier worker"),
	)
	athyr.Handle(server, subject, handler)
	fmt.Printf("%s ready on %s\n", name, subject)
	return server.Run(context.Background())
}

// ============================================================================
// Orchestrator - demonstrates HandoffRouter pattern
// ============================================================================

func runOrchestrator() error {
	fmt.Println("Intent Classifier - HandoffRouter Pattern")
	fmt.Println("==========================================")

	ctx := context.Background()
	agent := athyr.MustConnect(addr,
		athyr.WithAgentCard(athyr.AgentCard{Name: "intent-orchestrator"}),
	)
	defer agent.Close()

	// Build HandoffRouter - this is what we're demonstrating
	router := orchestration.NewHandoffRouter("intent-classifier").
		Triage(SubjectRouter).
		Route("weather", SubjectWeather).
		Route("calculator", SubjectCalculator).
		Route("joke", SubjectJoke)

	// Test queries
	tests := []string{
		"What's the weather like in Paris?",
		"Calculate 15 * 7 + 3",
		"Tell me a funny joke",
		"How hot is it in Tokyo?",
	}

	for i, query := range tests {
		fmt.Printf("\nQuery %d: %q\n", i+1, query)

		input, _ := json.Marshal(Query{Message: query})
		trace, err := router.HandleWithTrace(ctx, agent, input)
		if err != nil {
			fmt.Printf("  Error: %v\n", err)
			continue
		}

		var resp Response
		json.Unmarshal(trace.Output, &resp)

		fmt.Printf("  Route: %s\n", strings.Join(trace.RouteNames(), " → "))
		fmt.Printf("  Answer: %s\n", resp.Answer)
		fmt.Printf("  (took %v)\n", trace.Duration)
	}

	return nil
}

// ============================================================================
// Handlers
// ============================================================================

// routerHandler classifies intent and returns a HandoffDecision
func routerHandler(_ athyr.Context, req Query) (orchestration.HandoffDecision, error) {
	route := classifyIntent(req.Message)
	return orchestration.HandoffDecision{
		Route:   route,
		Context: mustMarshal(req), // Pass original query to specialist
	}, nil
}

func classifyIntent(msg string) string {
	lower := strings.ToLower(msg)

	weatherWords := []string{"weather", "temperature", "rain", "sunny", "hot", "cold"}
	for _, w := range weatherWords {
		if strings.Contains(lower, w) {
			return "weather"
		}
	}

	mathWords := []string{"calculate", "compute", "+", "-", "*", "/"}
	for _, w := range mathWords {
		if strings.Contains(lower, w) {
			return "calculator"
		}
	}

	return "joke" // Default
}

func weatherHandler(_ athyr.Context, req Query) (Response, error) {
	cities := map[string]string{
		"paris": "Paris: 18°C, partly cloudy",
		"tokyo": "Tokyo: 28°C, sunny and humid",
		"london": "London: 12°C, rainy",
	}

	lower := strings.ToLower(req.Message)
	for city, weather := range cities {
		if strings.Contains(lower, city) {
			return Response{Answer: weather}, nil
		}
	}
	return Response{Answer: "Weather data not available"}, nil
}

func calculatorHandler(_ athyr.Context, req Query) (Response, error) {
	// Simple parser: extract numbers and operators
	var nums []float64
	var ops []string

	for _, p := range strings.Fields(req.Message) {
		if n, err := strconv.ParseFloat(p, 64); err == nil {
			nums = append(nums, n)
		} else if p == "+" || p == "-" || p == "*" || p == "/" {
			ops = append(ops, p)
		}
	}

	if len(nums) < 2 {
		return Response{Answer: "Couldn't parse calculation"}, nil
	}

	result := nums[0]
	for i, op := range ops {
		if i+1 >= len(nums) {
			break
		}
		switch op {
		case "+":
			result += nums[i+1]
		case "-":
			result -= nums[i+1]
		case "*":
			result *= nums[i+1]
		case "/":
			if nums[i+1] != 0 {
				result /= nums[i+1]
			}
		}
	}

	return Response{Answer: fmt.Sprintf("Result: %.2f", result)}, nil
}

func jokeHandler(_ athyr.Context, req Query) (Response, error) {
	jokes := []string{
		"Why do programmers prefer dark mode? Because light attracts bugs!",
		"Why did the developer go broke? Used up all his cache!",
		"There are 10 types of people: those who understand binary and those who don't.",
	}
	return Response{Answer: jokes[len(req.Message)%len(jokes)]}, nil
}

func mustMarshal(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}