package registry

import (
	"fmt"

	"github.com/vhbfernandes/fitbit-agent/pkg/agent"
	"github.com/vhbfernandes/fitbit-agent/pkg/config"
	"github.com/vhbfernandes/fitbit-agent/pkg/input"
	"github.com/vhbfernandes/fitbit-agent/pkg/llm"
	"github.com/vhbfernandes/fitbit-agent/pkg/tools/fitbit"
	"github.com/vhbfernandes/fitbit-agent/pkg/tools/storage"
)

// Container holds all dependencies
type Container struct {
	toolRegistry  agent.ToolRegistry
	llmProvider   agent.LLMProvider
	inputProvider agent.UserInputProvider
	agent         agent.Agent
	llmError      error
}

// NewContainer creates a new dependency injection container
func NewContainer(providerType, systemPrompt string) (*Container, error) {
	// Create tool registry
	toolRegistry := NewDefaultToolRegistry()

	// Auto-discover and register tools
	if err := autoDiscoverTools(toolRegistry); err != nil {
		return nil, fmt.Errorf("failed to auto-discover tools: %w", err)
	}

	// Load system prompt with provided fallback
	systemPromptConfig := config.LoadSystemPrompt()
	if systemPrompt != "" && systemPromptConfig.IsDefault() {
		// Override default with provided system prompt
		systemPromptConfig = &config.SystemPrompt{}
		// We'll use reflection to set the content since it's unexported
		// For now, let's use the config loading approach
	}

	// Create configuration for LLM provider
	cfg := config.LoadConfig()
	if providerType != "" {
		cfg.LLMProvider = providerType
	}
	cfg.SystemPrompt = systemPromptConfig

	// Create LLM provider factory
	factory := llm.NewProviderFactory(cfg, toolRegistry)

	// Create LLM provider
	llmProvider, llmError := factory.CreateProvider()

	// Create input provider
	inputProvider := input.NewConsoleInputProvider()

	container := &Container{
		toolRegistry:  toolRegistry,
		llmProvider:   llmProvider,
		inputProvider: inputProvider,
		llmError:      llmError,
	}

	// Only create agent if LLM provider was created successfully
	if llmError == nil {
		container.agent = agent.NewInteractiveAgent(
			llmProvider,
			toolRegistry,
			inputProvider,
		)
	}

	return container, nil
}

// autoDiscoverTools automatically discovers and registers available tools
func autoDiscoverTools(registry agent.ToolRegistry) error {
	discovery := NewToolDiscovery(registry)

	// Register Fitbit tools
	fitbitLoginTool := fitbit.NewLoginTool()
	fitbitLogMealTool := fitbit.NewLogMealTool()
	fitbitGetProfileTool := fitbit.NewGetProfileTool()

	// Register storage tools
	saveMealTool := storage.NewSaveMealTool()
	viewSummaryTool := storage.NewViewSummaryTool()
	foodDatabaseTool := storage.NewFoodDatabaseTool()

	// Auto-register all tools
	err := discovery.AutoRegisterTools(
		fitbitLoginTool,
		fitbitLogMealTool,
		fitbitGetProfileTool,
		saveMealTool,
		viewSummaryTool,
		foodDatabaseTool,
	)

	return err
}

// GetAgent returns the configured agent
func (c *Container) GetAgent() agent.Agent {
	return c.agent
}

// GetToolRegistry returns the tool registry
func (c *Container) GetToolRegistry() agent.ToolRegistry {
	return c.toolRegistry
}

// GetLLMProvider returns the LLM provider
func (c *Container) GetLLMProvider() agent.LLMProvider {
	return c.llmProvider
}

// TryGetLLMProvider returns the LLM provider and any creation error
func (c *Container) TryGetLLMProvider() (agent.LLMProvider, error) {
	if c.llmError != nil {
		return nil, c.llmError
	}
	if c.llmProvider == nil {
		return nil, fmt.Errorf("LLM provider not initialized")
	}
	return c.llmProvider, nil
}

// GetInputProvider returns the input provider
func (c *Container) GetInputProvider() agent.UserInputProvider {
	return c.inputProvider
}
