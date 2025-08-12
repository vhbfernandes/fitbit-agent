package agent

import (
	"context"
	"encoding/json"
)

// Agent represents the main agent interface
type Agent interface {
	Run(ctx context.Context) error
}

// LLMProvider represents any LLM service (Claude, OpenAI, etc.)
type LLMProvider interface {
	GenerateResponse(ctx context.Context, conversation []Message) (*Response, error)
	Name() string
}

// ToolRegistry manages available tools
type ToolRegistry interface {
	GetTool(name string) (Tool, bool)
	GetAllTools() []Tool
	RegisterTool(tool Tool)
	GetToolDefinitions() []ToolDefinition
}

// Tool represents a single executable tool
type Tool interface {
	Name() string
	Description() string
	InputSchema() map[string]interface{}
	Execute(ctx context.Context, input json.RawMessage) (string, error)
}

// Message represents a conversation message
type Message struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

// Response represents an LLM response
type Response struct {
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ToolCall represents a tool invocation request from the LLM
type ToolCall struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Function string          `json:"function"`
	Input    json.RawMessage `json:"input"`
}

// ToolDefinition represents the schema for a tool
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// UserInputProvider provides user input functionality
type UserInputProvider interface {
	GetInput() (string, bool)
}
