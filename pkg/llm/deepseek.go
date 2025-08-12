package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/vhbfernandes/fitbit-agent/pkg/agent"
)

// DeepSeekProvider implements the LLMProvider interface for DeepSeek via Ollama
type DeepSeekProvider struct {
	ollamaHost   string
	toolRegistry agent.ToolRegistry
	model        string
	client       *http.Client
	systemPrompt string
}

// NewDeepSeekProvider creates a new DeepSeek LLM provider using Ollama
func NewDeepSeekProvider(toolRegistry agent.ToolRegistry, systemPrompt string) *DeepSeekProvider {
	ollamaHost := os.Getenv("OLLAMA_HOST")
	if ollamaHost == "" {
		ollamaHost = "http://localhost:11434"
	}

	model := os.Getenv("LLM_MODEL")
	if model == "" {
		model = "deepseek-r1:7b"
	}

	return &DeepSeekProvider{
		ollamaHost:   ollamaHost,
		toolRegistry: toolRegistry,
		model:        model,
		client:       &http.Client{},
		systemPrompt: systemPrompt,
	}
}

// Name returns the provider name
func (d *DeepSeekProvider) Name() string {
	return "DeepSeek (Ollama)"
}

// OllamaRequest represents the request structure for Ollama API
type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

// OllamaResponse represents the response structure from Ollama API
type OllamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
	Error    string `json:"error,omitempty"`
}

// GenerateResponse generates a response using DeepSeek via Ollama
func (d *DeepSeekProvider) GenerateResponse(ctx context.Context, conversation []agent.Message) (*agent.Response, error) {
	prompt := d.buildPrompt(conversation)

	request := OllamaRequest{
		Model:  d.model,
		Prompt: prompt,
		Stream: false,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/generate", d.ollamaHost)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request to Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var ollamaResp OllamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if ollamaResp.Error != "" {
		return nil, fmt.Errorf("ollama error: %s", ollamaResp.Error)
	}

	toolCalls := d.ParseToolCalls(ollamaResp.Response)

	return &agent.Response{
		Content:   ollamaResp.Response,
		ToolCalls: toolCalls,
	}, nil
}

func (d *DeepSeekProvider) buildPrompt(conversation []agent.Message) string {
	var prompt string

	// START WITH TOOL CALL REQUIREMENT - FIRST THING THE LLM SEES
	tools := d.toolRegistry.GetAllTools()
	if len(tools) > 0 {
		prompt += "🚨🚨🚨 CRITICAL: YOU MUST USE TOOLS! 🚨🚨🚨\n"
		prompt += "When user asks to log meals, you MUST use this EXACT format:\n"
		prompt += "TOOL_CALL: fitbit_log_meal({\"meal_type\": \"breakfast\", \"foods\": [{\"name\": \"scrambled eggs\", \"quantity\": 2, \"unit\": \"large\", \"calories\": 140}]})\n\n"
		prompt += "DO NOT just say 'I'll log it' - ACTUALLY CALL THE TOOL!\n\n"
	}

	if d.systemPrompt != "" {
		prompt += fmt.Sprintf("System: %s\n\n", d.systemPrompt)
	}

	if len(tools) > 0 {
		prompt += "🚨 AVAILABLE TOOLS:\n"
		for _, tool := range tools {
			prompt += fmt.Sprintf("- %s: %s\n", tool.Name(), tool.Description())
		}
		prompt += "\n"
	}

	for _, msg := range conversation {
		switch msg.Role {
		case "user":
			content := fmt.Sprintf("%s", msg.Content)
			if strings.HasPrefix(content, "Tool result: ") {
				result := strings.TrimPrefix(content, "Tool result: ")
				prompt += fmt.Sprintf("Tool Result:\n%s\n\n", result)

				// If tool result contains a suggested tool call, make it very explicit
				if strings.Contains(result, "TOOL_CALL:") {
					prompt += "🚨 The tool result above contains a suggested TOOL_CALL. You MUST execute it immediately using the exact format shown!\n"
					prompt += "Copy the TOOL_CALL line exactly as written in the tool result.\n\n"
				}
			} else {
				prompt += fmt.Sprintf("Human: %s\n", content)
			}
		case "assistant":
			prompt += fmt.Sprintf("Assistant: %s\n", msg.Content)
		}
	}

	// END WITH TOOL CALL REQUIREMENT - LAST THING THE LLM SEES
	if len(tools) > 0 {
		prompt += "\n🚨🚨🚨 TOOL CALL FORMAT RULES - FOLLOW EXACTLY! 🚨🚨🚨\n"
		prompt += "1. Make ONLY ONE tool call per response\n"
		prompt += "2. Use EXACT format: TOOL_CALL: tool_name(json)\n"
		prompt += "3. NO extra text after the closing parenthesis )\n"
		prompt += "4. NO semicolons, commas, or explanations after )\n"
		prompt += "5. JSON must be valid and complete\n"
		prompt += "6. End the line immediately after the )\n"
		prompt += "7. DO NOT repeat tool calls multiple times\n"
		prompt += "Example: TOOL_CALL: fitbit_log_meal({\"meal_type\": \"breakfast\", \"foods\": [{\"name\": \"eggs\", \"quantity\": 2, \"unit\": \"large\", \"calories\": 140}]})\n"
		prompt += "WRONG: TOOL_CALL: fitbit_log_meal({...}); followed by explanation\n"
		prompt += "WRONG: Making multiple identical tool calls\n"
		prompt += "RIGHT: TOOL_CALL: fitbit_log_meal({...})\n"
	}

	prompt += "Assistant: "
	return prompt
}

func (d *DeepSeekProvider) ParseToolCalls(response string) []agent.ToolCall {
	var toolCalls []agent.ToolCall

	// Primary pattern: TOOL_CALL: tool_name(json) - find the tool call start and manually parse the content
	re := regexp.MustCompile(`TOOL_CALL:\s*(\w+)\s*\(`)
	matches := re.FindAllStringSubmatchIndex(response, -1)

	for i, match := range matches {
		if len(match) >= 4 {
			toolName := response[match[2]:match[3]]

			// Find the opening parenthesis position
			openParenPos := match[1] - 1 // match[1] is after the (, so -1 to get the ( position

			// Manually parse from the opening parenthesis to find the matching closing one
			jsonContent := extractJSONManually(response, openParenPos)

			// Clean up the JSON content - only remove trailing semicolons
			jsonContent = fixCommonJSONIssues(jsonContent)

			if jsonContent == "" {
				continue
			}

			var input json.RawMessage
			if json.Valid([]byte(jsonContent)) {
				input = json.RawMessage(jsonContent)
			} else {
				// If still invalid, wrap in input field for debugging
				escapedInput := fmt.Sprintf("{\"input\": %q}", jsonContent)
				input = json.RawMessage(escapedInput)
			}

			toolCall := agent.ToolCall{
				ID:       fmt.Sprintf("call_%d", i),
				Name:     toolName,
				Function: toolName,
				Input:    input,
			}

			toolCalls = append(toolCalls, toolCall)
		}
	} // Fallback patterns for other formats
	if len(toolCalls) == 0 {
		// Try: Call tool_name with {...}
		re2 := regexp.MustCompile(`Call\s+(\w+)\s+with\s+({[^}]*})`)
		matches2 := re2.FindAllStringSubmatch(response, -1)

		for i, match := range matches2 {
			if len(match) >= 3 {
				toolName := match[1]
				inputStr := match[2]

				var input json.RawMessage
				if json.Valid([]byte(inputStr)) {
					input = json.RawMessage(inputStr)
				} else {
					escapedInput := fmt.Sprintf("{\"input\": %q}", inputStr)
					input = json.RawMessage(escapedInput)
				}

				toolCall := agent.ToolCall{
					ID:       fmt.Sprintf("call_%d", i),
					Name:     toolName,
					Function: toolName,
					Input:    input,
				}

				toolCalls = append(toolCalls, toolCall)
			}
		}
	}

	return toolCalls
}

// extractJSONManually extracts JSON content from a tool call, handling nested parentheses properly
func extractJSONManually(text string, startPos int) string {
	if startPos >= len(text) || text[startPos] != '(' {
		return ""
	}

	// Start after the opening parenthesis
	pos := startPos + 1
	parenCount := 1
	inString := false
	escaped := false

	for pos < len(text) {
		char := text[pos]

		if escaped {
			escaped = false
			pos++
			continue
		}

		if char == '\\' {
			escaped = true
			pos++
			continue
		}

		if char == '"' {
			inString = !inString
			pos++
			continue
		}

		if !inString {
			if char == '(' {
				parenCount++
			} else if char == ')' {
				parenCount--
				if parenCount == 0 {
					// Found the matching closing parenthesis
					return strings.TrimSpace(text[startPos+1 : pos])
				}
			}
		}

		pos++
	}

	// If we didn't find a matching closing parenthesis, return what we have
	return strings.TrimSpace(text[startPos+1:])
}

// fixCommonJSONIssues attempts to fix common JSON formatting issues
func fixCommonJSONIssues(jsonStr string) string {
	// Trim whitespace
	jsonStr = strings.TrimSpace(jsonStr)

	// Only remove trailing semicolons - don't be too aggressive
	if strings.HasSuffix(jsonStr, ";") {
		jsonStr = strings.TrimSuffix(jsonStr, ";")
		jsonStr = strings.TrimSpace(jsonStr)
	}

	return jsonStr
}

func (d *DeepSeekProvider) ValidateConnection() error {
	resp, err := d.client.Get(d.ollamaHost + "/api/tags")
	if err != nil {
		return fmt.Errorf("cannot connect to Ollama at %s: %w", d.ollamaHost, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read Ollama response: %w", err)
	}

	var modelsResp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return fmt.Errorf("failed to parse Ollama models response: %w", err)
	}

	modelFound := false
	for _, model := range modelsResp.Models {
		if model.Name == d.model {
			modelFound = true
			break
		}
	}

	if !modelFound {
		modelNames := make([]string, len(modelsResp.Models))
		for i, model := range modelsResp.Models {
			modelNames[i] = model.Name
		}
		return fmt.Errorf("model '%s' not found. Available: %v", d.model, modelNames)
	}

	return nil
}
