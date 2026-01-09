package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/athyr-tech/athyr-sdk-go/pkg/athyr"
)

// Tools defines the functions available to the LLM.
// Each tool has a name, description, and JSON Schema for its parameters.
var Tools = []athyr.Tool{
	{
		Name:        "get_weather",
		Description: "Get the current weather for a city",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"city": {"type": "string", "description": "City name"}
			},
			"required": ["city"]
		}`),
	},
	{
		Name:        "add",
		Description: "Add two numbers together",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"a": {"type": "number", "description": "First number"},
				"b": {"type": "number", "description": "Second number"}
			},
			"required": ["a", "b"]
		}`),
	},
	{
		Name:        "multiply",
		Description: "Multiply two numbers",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"a": {"type": "number", "description": "First number"},
				"b": {"type": "number", "description": "Second number"}
			},
			"required": ["a", "b"]
		}`),
	},
}

// ExecuteTool runs a tool and returns the result as JSON.
func ExecuteTool(call athyr.ToolCall) string {
	switch call.Name {
	case "get_weather":
		var args struct{ City string }
		json.Unmarshal(call.Arguments, &args)
		return getWeather(args.City)

	case "add":
		var args struct{ A, B float64 }
		json.Unmarshal(call.Arguments, &args)
		return fmt.Sprintf(`{"result": %g}`, args.A+args.B)

	case "multiply":
		var args struct{ A, B float64 }
		json.Unmarshal(call.Arguments, &args)
		return fmt.Sprintf(`{"result": %g}`, args.A*args.B)

	default:
		return fmt.Sprintf(`{"error": "unknown tool: %s"}`, call.Name)
	}
}

func getWeather(city string) string {
	// Simulated data - replace with real API in production
	data := map[string]string{
		"paris":    `{"city": "Paris", "temp": "18°C", "conditions": "partly cloudy"}`,
		"london":   `{"city": "London", "temp": "14°C", "conditions": "rainy"}`,
		"new york": `{"city": "New York", "temp": "22°C", "conditions": "sunny"}`,
		"tokyo":    `{"city": "Tokyo", "temp": "26°C", "conditions": "humid"}`,
	}
	if result, ok := data[strings.ToLower(city)]; ok {
		return result
	}
	return fmt.Sprintf(`{"city": "%s", "temp": "20°C", "conditions": "unknown"}`, city)
}
