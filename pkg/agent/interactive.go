package agent

import (
	"context"
	"fmt"
	"strings"
)

// Implementation of the main agent
type InteractiveAgent struct {
	llmProvider   LLMProvider
	toolRegistry  ToolRegistry
	inputProvider UserInputProvider
}

// NewInteractiveAgent creates a new interactive agent
func NewInteractiveAgent(llm LLMProvider, registry ToolRegistry, input UserInputProvider) *InteractiveAgent {
	return &InteractiveAgent{
		llmProvider:   llm,
		toolRegistry:  registry,
		inputProvider: input,
	}
}

// Run starts the interactive agent loop
func (a *InteractiveAgent) Run(ctx context.Context) error {
	conversation := []Message{}

	fmt.Printf("🥗 Welcome to Fitbit Agent! Chat with %s to log your meals (use 'ctrl-c' to quit)\n", a.llmProvider.Name())
	fmt.Println("Try saying: 'I had scrambled eggs and toast for breakfast'")

	readUserInput := true
	for {
		if readUserInput {
			fmt.Print("\u001b[94mYou\u001b[0m: ")
			userInput, ok := a.inputProvider.GetInput()
			if !ok {
				break
			}

			conversation = append(conversation, Message{
				Role:    "user",
				Content: userInput,
			})
		}

		response, err := a.llmProvider.GenerateResponse(ctx, conversation)
		if err != nil {
			// Check for specific API errors and handle gracefully
			if a.isRecoverableError(err) {
				fmt.Printf("\u001b[91m❌ %s API Error\u001b[0m: %s\n", a.llmProvider.Name(), err.Error())
				fmt.Printf("\u001b[93m💡 Suggestion\u001b[0m: ")

				if strings.Contains(err.Error(), "quota") {
					fmt.Printf("You've exceeded your API quota. Please:\n")
					fmt.Printf("   1. Check your billing plan\n")
					fmt.Printf("   2. Wait for quota reset\n")
					fmt.Printf("   3. Try using a different provider (deepseek with Ollama)\n")
				} else if strings.Contains(err.Error(), "rate limit") {
					fmt.Printf("API rate limited. Please wait a moment and try again.\n")
				} else if strings.Contains(err.Error(), "API key") {
					fmt.Printf("Invalid API key. Please check your %s_API_KEY environment variable.\n", strings.ToUpper(a.llmProvider.Name()))
				} else if strings.Contains(err.Error(), "service unavailable") {
					fmt.Printf("Service temporarily unavailable. Please try again later.\n")
				} else {
					fmt.Printf("Try again or switch to a different LLM provider.\n")
				}

				// Continue the conversation loop instead of crashing
				fmt.Print("\nPress Enter to continue or Ctrl+C to quit...")
				a.inputProvider.GetInput()
				readUserInput = true
				continue
			}

			// For non-recoverable errors, still return them
			return fmt.Errorf("LLM error: %w", err)
		}

		// Add assistant response to conversation
		conversation = append(conversation, Message{
			Role:    "assistant",
			Content: response.Content,
		})

		// Display assistant response if there's text content
		if response.Content != "" {
			fmt.Printf("\u001b[93mFitbit Agent\u001b[0m: %s\n", response.Content)
		}

		// Execute any tool calls
		toolResults := []string{}
		for _, toolCall := range response.ToolCalls {
			result := a.executeTool(ctx, toolCall)
			toolResults = append(toolResults, result)
		}

		if len(toolResults) == 0 {
			readUserInput = true
			continue
		}

		// Display tool results to user and add them to conversation
		readUserInput = false
		for i, result := range toolResults {
			// Check if it's an error or success
			if strings.HasPrefix(result, "Error") {
				fmt.Printf("\u001b[91m❌ Tool Error %d\u001b[0m:\n%s\n\n", i+1, result)
			} else {
				fmt.Printf("\u001b[92m✅ Tool Success %d\u001b[0m:\n%s\n\n", i+1, result)
			}

			// Add tool result to conversation with clear formatting for the LLM
			conversation = append(conversation, Message{
				Role:    "user",
				Content: fmt.Sprintf("Tool result: %s", result),
			})

			// Check if tool result contains suggested tool calls
			if strings.Contains(result, "TOOL_CALL:") {
				fmt.Printf("\u001b[96m🔧 Tool suggested another action, processing...\u001b[0m\n")
				readUserInput = false // Force LLM to process the suggested tool call
			}
		}
	}

	return nil
}

// isRecoverableError checks if an error is recoverable (API quota, rate limits, etc.)
func (a *InteractiveAgent) isRecoverableError(err error) bool {
	errStr := strings.ToLower(err.Error())

	// Check for common recoverable API errors
	recoverableKeywords := []string{
		"quota",
		"rate limit",
		"429",
		"api key",
		"401",
		"403",
		"service unavailable",
		"502",
		"503",
		"504",
		"timeout",
		"temporary",
	}

	for _, keyword := range recoverableKeywords {
		if strings.Contains(errStr, keyword) {
			return true
		}
	}

	return false
}

func (a *InteractiveAgent) executeTool(ctx context.Context, toolCall ToolCall) string {
	tool, found := a.toolRegistry.GetTool(toolCall.Name)
	if !found {
		return fmt.Sprintf("Error: tool '%s' not found", toolCall.Name)
	}

	fmt.Printf("\u001b[92mtool\u001b[0m: %s(%s)\n", toolCall.Name, string(toolCall.Input))

	result, err := tool.Execute(ctx, toolCall.Input)
	if err != nil {
		return fmt.Sprintf("Error executing tool '%s': %s", toolCall.Name, err.Error())
	}

	return result
}
