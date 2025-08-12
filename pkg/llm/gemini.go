package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/vhbfernandes/fitbit-agent/pkg/agent"
)

// API error types for better error handling
var (
	ErrQuotaExceeded  = errors.New("API quota exceeded")
	ErrRateLimited    = errors.New("API rate limited")
	ErrAPIKey         = errors.New("invalid API key")
	ErrServiceDown    = errors.New("service unavailable")
	ErrInvalidRequest = errors.New("invalid request")
)

// GeminiProvider implements the LLMProvider interface for Google Gemini
type GeminiProvider struct {
	apiKey       string
	toolRegistry agent.ToolRegistry
	model        string
	client       *http.Client
	systemPrompt string
}

// NewGeminiProvider creates a new Gemini LLM provider
func NewGeminiProvider(apiKey string, toolRegistry agent.ToolRegistry, systemPrompt string) *GeminiProvider {
	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = "gemini-1.5-flash"
	}

	return &GeminiProvider{
		apiKey:       apiKey,
		toolRegistry: toolRegistry,
		model:        model,
		client:       &http.Client{},
		systemPrompt: systemPrompt,
	}
}

// Name returns the provider name
func (g *GeminiProvider) Name() string {
	return "Gemini"
}

// GeminiRequest represents the request structure for Gemini API
type GeminiRequest struct {
	Contents []GeminiContent `json:"contents"`
}

// GeminiContent represents content in Gemini format
type GeminiContent struct {
	Role  string       `json:"role"`
	Parts []GeminiPart `json:"parts"`
}

// GeminiPart represents a part of content
type GeminiPart struct {
	Text string `json:"text"`
}

// GeminiResponse represents the response from Gemini API
type GeminiResponse struct {
	Candidates []GeminiCandidate `json:"candidates"`
	Error      *GeminiError      `json:"error,omitempty"`
}

// GeminiCandidate represents a response candidate
type GeminiCandidate struct {
	Content GeminiContent `json:"content"`
}

// GeminiError represents an error from Gemini API
type GeminiError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// handleAPIError converts Gemini API errors to user-friendly errors
func (g *GeminiProvider) handleAPIError(statusCode int, geminiErr *GeminiError) error {
	if geminiErr == nil {
		switch statusCode {
		case 429:
			return fmt.Errorf("%w: please check your plan and billing details", ErrRateLimited)
		case 401, 403:
			return fmt.Errorf("%w: please check your API key", ErrAPIKey)
		case 500, 502, 503, 504:
			return fmt.Errorf("%w: try again later", ErrServiceDown)
		default:
			return fmt.Errorf("HTTP error %d", statusCode)
		}
	}

	// Handle specific error codes from Gemini
	switch geminiErr.Code {
	case 429:
		if strings.Contains(strings.ToLower(geminiErr.Message), "quota") {
			return fmt.Errorf("%w: %s", ErrQuotaExceeded, geminiErr.Message)
		}
		return fmt.Errorf("%w: %s", ErrRateLimited, geminiErr.Message)
	case 400:
		return fmt.Errorf("%w: %s", ErrInvalidRequest, geminiErr.Message)
	case 401, 403:
		return fmt.Errorf("%w: %s", ErrAPIKey, geminiErr.Message)
	case 500, 502, 503, 504:
		return fmt.Errorf("%w: %s", ErrServiceDown, geminiErr.Message)
	default:
		return fmt.Errorf("gemini API error (%d): %s", geminiErr.Code, geminiErr.Message)
	}
}

// GenerateResponse generates a response using Gemini
func (g *GeminiProvider) GenerateResponse(ctx context.Context, conversation []agent.Message) (*agent.Response, error) {
	contents := g.buildContents(conversation)

	request := GeminiRequest{
		Contents: contents,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", g.model, g.apiKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request to Gemini: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Handle HTTP errors first
	if resp.StatusCode != http.StatusOK {
		var geminiResp GeminiResponse
		// Try to parse error response, but don't fail if we can't
		json.Unmarshal(body, &geminiResp)
		return nil, g.handleAPIError(resp.StatusCode, geminiResp.Error)
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Handle API errors from response
	if geminiResp.Error != nil {
		return nil, g.handleAPIError(200, geminiResp.Error)
	}

	if len(geminiResp.Candidates) == 0 {
		return nil, fmt.Errorf("no response candidates received")
	}

	responseText := ""
	if len(geminiResp.Candidates[0].Content.Parts) > 0 {
		responseText = geminiResp.Candidates[0].Content.Parts[0].Text
	}

	toolCalls := g.ParseToolCalls(responseText)

	return &agent.Response{
		Content:   responseText,
		ToolCalls: toolCalls,
	}, nil
}

func (g *GeminiProvider) buildContents(conversation []agent.Message) []GeminiContent {
	var contents []GeminiContent

	// Add system prompt as first user message if available
	if g.systemPrompt != "" {
		systemContent := g.buildSystemPrompt()
		contents = append(contents, GeminiContent{
			Role: "user",
			Parts: []GeminiPart{
				{Text: systemContent},
			},
		})
		// Add a model response acknowledging the system prompt
		contents = append(contents, GeminiContent{
			Role: "model",
			Parts: []GeminiPart{
				{Text: "I understand. I'm your Fitbit nutrition assistant and I'm ready to help you log meals and track your nutrition using natural language. Just describe what you ate!"},
			},
		})
	}

	// Add conversation history
	for _, msg := range conversation {
		role := "user"
		if msg.Role == "assistant" {
			role = "model"
		}

		content := fmt.Sprintf("%s", msg.Content)
		if strings.HasPrefix(content, "Tool result: ") {
			result := strings.TrimPrefix(content, "Tool result: ")
			content = fmt.Sprintf("Tool Result:\n%s\n\nPlease present this information to the user.", result)

			// If tool result contains a suggested tool call, make it very explicit
			if strings.Contains(result, "TOOL_CALL:") {
				content += "\n🚨 The tool result above contains a suggested TOOL_CALL. You MUST execute it immediately using the exact format shown!\n"
				content += "Copy the TOOL_CALL line exactly as written in the tool result."
			}
		}

		contents = append(contents, GeminiContent{
			Role: role,
			Parts: []GeminiPart{
				{Text: content},
			},
		})
	}

	return contents
}

func (g *GeminiProvider) buildSystemPrompt() string {
	var prompt string

	// START WITH TOOL CALL REQUIREMENT - FIRST THING THE LLM SEES
	tools := g.toolRegistry.GetAllTools()
	if len(tools) > 0 {
		prompt += "🚨🚨🚨 CRITICAL: YOU MUST USE TOOLS! 🚨🚨🚨\n"
		prompt += "When user asks to log meals, you MUST use this EXACT format:\n"
		prompt += "TOOL_CALL: fitbit_log_meal({\"meal_type\": \"breakfast\", \"foods\": [{\"name\": \"scrambled eggs\", \"quantity\": 2, \"unit\": \"large\", \"calories\": 140}]})\n\n"
		prompt += "DO NOT just say 'I'll log it' - ACTUALLY CALL THE TOOL!\n\n"
	}

	prompt += fmt.Sprintf("System: %s\n\n", g.systemPrompt)

	if len(tools) > 0 {
		prompt += "🚨 AVAILABLE TOOLS:\n"
		for _, tool := range tools {
			prompt += fmt.Sprintf("- %s: %s\n", tool.Name(), tool.Description())
		}
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
		prompt += "RIGHT: TOOL_CALL: fitbit_log_meal({...})\n\n"
	}

	return prompt
}

func (g *GeminiProvider) ParseToolCalls(response string) []agent.ToolCall {
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
	}

	// Fallback patterns for other formats
	if len(toolCalls) == 0 {
		// Pattern 2: ```tool_call\nfitbit_log_meal({"meal_type": "breakfast"})\n```
		re2 := regexp.MustCompile("```tool_call\\s*\\n([\\w_]+)\\s*\\(([^)]*)\\)\\s*\\n```")
		matches2 := re2.FindAllStringSubmatch(response, -1)

		for i, match := range matches2 {
			if len(match) >= 3 {
				toolName := match[1]
				inputStr := match[2]

				var input json.RawMessage
				if inputStr == "" {
					input = json.RawMessage("{}")
				} else if json.Valid([]byte(inputStr)) {
					input = json.RawMessage(inputStr)
				} else {
					escapedInput := fmt.Sprintf("{\"input\": %q}", inputStr)
					input = json.RawMessage(escapedInput)
				}

				toolCall := agent.ToolCall{
					ID:       fmt.Sprintf("call_%d", len(toolCalls)+i),
					Name:     toolName,
					Function: toolName,
					Input:    input,
				}

				toolCalls = append(toolCalls, toolCall)
			}
		}
	}

	// Pattern 3: Try: Call tool_name with {...}
	if len(toolCalls) == 0 {
		re3 := regexp.MustCompile(`Call\s+(\w+)\s+with\s+({[^}]*})`)
		matches3 := re3.FindAllStringSubmatch(response, -1)

		for i, match := range matches3 {
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

	// Pattern 4: Just function calls in the text without special markers (only for known tools)
	if len(toolCalls) == 0 {
		re4 := regexp.MustCompile(`(\w+)\((\{[^}]*\})\)`)
		matches4 := re4.FindAllStringSubmatch(response, -1)

		// Only use pattern 4 if the function name looks like a tool
		tools := g.toolRegistry.GetAllTools()
		toolNames := make(map[string]bool)
		for _, tool := range tools {
			toolNames[tool.Name()] = true
		}

		for i, match := range matches4 {
			if len(match) >= 3 {
				toolName := match[1]
				inputStr := match[2]

				// Only treat as tool call if it's a known tool name
				if toolNames[toolName] {
					var input json.RawMessage
					if json.Valid([]byte(inputStr)) {
						input = json.RawMessage(inputStr)
					} else {
						continue // Skip invalid JSON
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
	}

	return toolCalls
}
