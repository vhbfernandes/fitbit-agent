package registry

import (
	"sync"

	"github.com/vhbfernandes/fitbit-agent/pkg/agent"
)

// DefaultToolRegistry implements the ToolRegistry interface
type DefaultToolRegistry struct {
	tools map[string]agent.Tool
	mu    sync.RWMutex
}

// NewDefaultToolRegistry creates a new tool registry
func NewDefaultToolRegistry() *DefaultToolRegistry {
	return &DefaultToolRegistry{
		tools: make(map[string]agent.Tool),
	}
}

// RegisterTool adds a tool to the registry
func (r *DefaultToolRegistry) RegisterTool(tool agent.Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name()] = tool
}

// GetTool retrieves a tool by name
func (r *DefaultToolRegistry) GetTool(name string) (agent.Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, exists := r.tools[name]
	return tool, exists
}

// GetAllTools returns all registered tools
func (r *DefaultToolRegistry) GetAllTools() []agent.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]agent.Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// GetToolDefinitions returns tool definitions for LLM
func (r *DefaultToolRegistry) GetToolDefinitions() []agent.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	definitions := make([]agent.ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		definitions = append(definitions, agent.ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			InputSchema: tool.InputSchema(),
		})
	}
	return definitions
}
