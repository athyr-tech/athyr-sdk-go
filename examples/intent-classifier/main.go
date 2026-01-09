// Example: Intent Classifier (HandoffRouter Pattern)
//
// Demonstrates dynamic request routing based on classification.
// A router agent classifies user intent and hands off to specialists:
//   - Weather agent (handles weather queries)
//   - Calculator agent (handles math)
//   - Joke agent (handles entertainment)
//
// Usage:
//
//	# Terminal 1: Start each agent (or run all in background)
//	go run ./examples/intent-classifier -role=router &
//	go run ./examples/intent-classifier -role=weather &
//	go run ./examples/intent-classifier -role=calculator &
//	go run ./examples/intent-classifier -role=joke &
//
//	# Terminal 2: Run the orchestrator
//	go run ./examples/intent-classifier -role=orchestrator
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/athyr-tech/athyr-sdk-go/pkg/athyr"
	"github.com/athyr-tech/athyr-sdk-go/pkg/orchestration"
)

const athyrAddr = "localhost:9090"

// Subjects for each agent
const (
	SubjectRouter     = "intent.router"
	SubjectWeather    = "intent.weather"
	SubjectCalculator = "intent.calculator"
	SubjectJoke       = "intent.joke"
)

// UserQuery is the input from users
type UserQuery struct {
	Message string `json:"message"`
}

// AgentResponse is the final response to users
type AgentResponse struct {
	Answer string `json:"answer"`
	Agent  string `json:"agent"`
}

func main() {
	role := flag.String("role", "orchestrator", "Role: orchestrator, router, weather, calculator, or joke")
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
	case "router":
		err = runRouterAgent(ctx)
	case "weather":
		err = runSpecialistAgent(ctx, "weather", SubjectWeather, handleWeather)
	case "calculator":
		err = runSpecialistAgent(ctx, "calculator", SubjectCalculator, handleCalculator)
	case "joke":
		err = runSpecialistAgent(ctx, "joke", SubjectJoke, handleJoke)
	default:
		fmt.Fprintf(os.Stderr, "Unknown role: %s\n", *role)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// runOrchestrator demonstrates the HandoffRouter pattern
func runOrchestrator(ctx context.Context) error {
	fmt.Println("Intent Classifier - HandoffRouter Pattern Demo")
	fmt.Println("==============================================")

	// Connect to Athyr
	agent, err := athyr.NewAgent(athyrAddr,
		athyr.WithAgentCard(athyr.AgentCard{
			Name:        "intent-orchestrator",
			Description: "Routes user queries to appropriate specialists",
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

	fmt.Println("Connected! Processing queries...")
	fmt.Println()

	// Build the HandoffRouter
	router := orchestration.NewHandoffRouter("intent-classifier").
		Triage(SubjectRouter).
		Route("weather", SubjectWeather).
		Route("calculator", SubjectCalculator).
		Route("joke", SubjectJoke).
		MaxHandoffs(3)

	// Test queries
	testQueries := []string{
		"What's the weather like in Paris?",
		"Calculate 15 * 7 + 3",
		"Tell me a funny joke",
		"How hot is it in Tokyo?",
	}

	for i, query := range testQueries {
		fmt.Printf("--- Query %d ---\n", i+1)
		fmt.Printf("User: %q\n\n", query)

		// Prepare input
		input, _ := json.Marshal(UserQuery{Message: query})

		// Route the request
		trace, err := router.HandleWithTrace(ctx, agent, input)
		if err != nil {
			fmt.Printf("Error: %v\n\n", err)
			continue
		}

		// Parse and display result
		var resp AgentResponse
		if err := json.Unmarshal(trace.Output, &resp); err != nil {
			resp.Answer = string(trace.Output)
		}

		fmt.Printf("Route: %s\n", strings.Join(trace.RouteNames(), " -> "))
		fmt.Printf("Answer: %s\n", resp.Answer)
		fmt.Printf("Duration: %v\n\n", trace.Duration)
	}

	return nil
}

// runRouterAgent runs the triage/classification agent
func runRouterAgent(ctx context.Context) error {
	fmt.Println("Starting router agent...")

	agent, err := athyr.NewAgent(athyrAddr,
		athyr.WithAgentCard(athyr.AgentCard{
			Name:        "intent-router",
			Description: "Classifies user intent",
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

	// Subscribe to router requests
	_, err = agent.Subscribe(ctx, SubjectRouter, func(msg athyr.SubscribeMessage) {
		var query UserQuery
		if err := json.Unmarshal(msg.Data, &query); err != nil {
			return
		}

		// Classify intent (mock - real implementation would use LLM)
		route := classifyIntent(query.Message)

		// Return handoff decision
		decision := orchestration.HandoffDecision{
			Route:   route,
			Context: msg.Data, // Pass original query to specialist
		}
		resp, _ := json.Marshal(decision)

		if msg.Reply != "" {
			_ = agent.Publish(ctx, msg.Reply, resp)
		}
	})
	if err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}

	fmt.Printf("Router agent ready on %s\n", SubjectRouter)
	<-ctx.Done()
	return nil
}

// classifyIntent determines which specialist should handle the query
func classifyIntent(message string) string {
	msg := strings.ToLower(message)

	// Simple keyword matching (replace with LLM in production)
	weatherWords := []string{"weather", "temperature", "rain", "sunny", "hot", "cold", "forecast"}
	for _, word := range weatherWords {
		if strings.Contains(msg, word) {
			return "weather"
		}
	}

	mathWords := []string{"calculate", "compute", "+", "-", "*", "/", "sum", "multiply"}
	for _, word := range mathWords {
		if strings.Contains(msg, word) {
			return "calculator"
		}
	}

	jokeWords := []string{"joke", "funny", "laugh", "humor"}
	for _, word := range jokeWords {
		if strings.Contains(msg, word) {
			return "joke"
		}
	}

	return "joke" // Default to joke for unknown queries
}

// runSpecialistAgent runs a specialist agent
func runSpecialistAgent(ctx context.Context, name, subject string, handler func(string) string) error {
	fmt.Printf("Starting %s agent...\n", name)

	agent, err := athyr.NewAgent(athyrAddr,
		athyr.WithAgentCard(athyr.AgentCard{
			Name:        fmt.Sprintf("%s-specialist", name),
			Description: fmt.Sprintf("Handles %s queries", name),
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

	// Subscribe to specialist requests
	_, err = agent.Subscribe(ctx, subject, func(msg athyr.SubscribeMessage) {
		var query UserQuery
		if err := json.Unmarshal(msg.Data, &query); err != nil {
			return
		}

		// Handle the query
		answer := handler(query.Message)
		resp, _ := json.Marshal(AgentResponse{
			Answer: answer,
			Agent:  name,
		})

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

// Mock handlers (replace with real LLM calls in production)

func handleWeather(message string) string {
	// Extract city (simple mock)
	cities := map[string]string{
		"paris": "Paris: 18°C, partly cloudy",
		"tokyo": "Tokyo: 28°C, sunny and humid",
		"london": "London: 12°C, rainy as usual",
		"new york": "New York: 22°C, clear skies",
	}

	msg := strings.ToLower(message)
	for city, weather := range cities {
		if strings.Contains(msg, city) {
			return weather
		}
	}
	return "Weather data not available for that location"
}

func handleCalculator(message string) string {
	// Simple expression parser (mock)
	// Look for pattern like "15 * 7 + 3"
	msg := strings.ToLower(message)

	// Extract numbers
	var nums []float64
	var ops []string

	parts := strings.Fields(msg)
	for _, p := range parts {
		if n, err := strconv.ParseFloat(p, 64); err == nil {
			nums = append(nums, n)
		} else if p == "+" || p == "-" || p == "*" || p == "/" {
			ops = append(ops, p)
		}
	}

	if len(nums) < 2 || len(ops) < 1 {
		return "I couldn't parse that calculation"
	}

	// Simple left-to-right evaluation (no precedence)
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

	return fmt.Sprintf("Result: %.2f", result)
}

func handleJoke(message string) string {
	jokes := []string{
		"Why do programmers prefer dark mode? Because light attracts bugs!",
		"Why did the developer go broke? Because he used up all his cache!",
		"There are only 10 types of people: those who understand binary and those who don't.",
	}
	// Return a "random" joke (using message length as seed)
	return jokes[len(message)%len(jokes)]
}
