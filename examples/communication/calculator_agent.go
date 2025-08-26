package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	framework "github.com/itsneelabh/gomind"
)

// CalculatorAgent is a simple agent that can perform calculations
type CalculatorAgent struct {
	*framework.BaseAgent
}

// Initialize sets up the calculator agent
func (c *CalculatorAgent) Initialize(ctx context.Context) error {
	log.Println("Calculator Agent initialized")
	return nil
}

// ProcessRequest handles natural language calculation requests from other agents
func (c *CalculatorAgent) ProcessRequest(ctx context.Context, instruction string) (string, error) {
	log.Printf("Calculator received request: %s\n", instruction)
	
	// Convert to lowercase for easier parsing
	instruction = strings.ToLower(instruction)
	
	// Simple parsing for basic operations
	if strings.Contains(instruction, "add") || strings.Contains(instruction, "plus") || strings.Contains(instruction, "+") {
		numbers := extractNumbers(instruction)
		if len(numbers) >= 2 {
			result := numbers[0] + numbers[1]
			return fmt.Sprintf("The result of adding %.2f and %.2f is %.2f", numbers[0], numbers[1], result), nil
		}
		return "I need at least two numbers to add", nil
	}
	
	if strings.Contains(instruction, "subtract") || strings.Contains(instruction, "minus") || strings.Contains(instruction, "-") {
		numbers := extractNumbers(instruction)
		if len(numbers) >= 2 {
			result := numbers[0] - numbers[1]
			return fmt.Sprintf("The result of subtracting %.2f from %.2f is %.2f", numbers[1], numbers[0], result), nil
		}
		return "I need at least two numbers to subtract", nil
	}
	
	if strings.Contains(instruction, "multiply") || strings.Contains(instruction, "times") || strings.Contains(instruction, "*") {
		numbers := extractNumbers(instruction)
		if len(numbers) >= 2 {
			result := numbers[0] * numbers[1]
			return fmt.Sprintf("The result of multiplying %.2f by %.2f is %.2f", numbers[0], numbers[1], result), nil
		}
		return "I need at least two numbers to multiply", nil
	}
	
	if strings.Contains(instruction, "divide") || strings.Contains(instruction, "/") {
		numbers := extractNumbers(instruction)
		if len(numbers) >= 2 {
			if numbers[1] == 0 {
				return "Cannot divide by zero", nil
			}
			result := numbers[0] / numbers[1]
			return fmt.Sprintf("The result of dividing %.2f by %.2f is %.2f", numbers[0], numbers[1], result), nil
		}
		return "I need at least two numbers to divide", nil
	}
	
	return "I can help with basic calculations. Please ask me to add, subtract, multiply, or divide numbers.", nil
}

// extractNumbers extracts floating point numbers from a string
func extractNumbers(s string) []float64 {
	var numbers []float64
	
	// Split by common delimiters
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return !((r >= '0' && r <= '9') || r == '.' || r == '-')
	})
	
	for _, part := range parts {
		if num, err := strconv.ParseFloat(part, 64); err == nil {
			numbers = append(numbers, num)
		}
	}
	
	return numbers
}

func main() {
	agent := &CalculatorAgent{
		BaseAgent: &framework.BaseAgent{},
	}
	
	// Run the agent with the framework
	if err := framework.RunAgent(agent,
		framework.WithAgentName("calculator-agent"),
		framework.WithPort(8081),
		framework.WithRedisURL("redis://localhost:6379"),
	); err != nil {
		log.Fatal(err)
	}
}