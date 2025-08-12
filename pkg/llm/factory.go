package llm

import (
	"fmt"

	"github.com/vhbfernandes/fitbit-agent/pkg/agent"
	"github.com/vhbfernandes/fitbit-agent/pkg/config"
)

// ProviderFactory creates LLM providers based on configuration
type ProviderFactory struct {
	config       *config.Config
	toolRegistry agent.ToolRegistry
}

// NewProviderFactory creates a new provider factory
func NewProviderFactory(config *config.Config, toolRegistry agent.ToolRegistry) *ProviderFactory {
	return &ProviderFactory{
		config:       config,
		toolRegistry: toolRegistry,
	}
}

// CreateProvider creates an LLM provider based on the configuration
func (f *ProviderFactory) CreateProvider() (agent.LLMProvider, error) {
	systemPrompt := f.config.SystemPrompt.GetContent()

	switch f.config.LLMProvider {
	case "deepseek":
		// DeepSeek via Ollama - validate connection
		provider := NewDeepSeekProvider(f.toolRegistry, systemPrompt)
		if err := provider.ValidateConnection(); err != nil {
			return nil, fmt.Errorf("DeepSeek (Ollama) connection failed: %w", err)
		}
		return provider, nil

	case "gemini":
		if f.config.GeminiAPIKey == "" {
			return nil, fmt.Errorf("GEMINI_API_KEY environment variable is required for Gemini provider")
		}
		return NewGeminiProvider(f.config.GeminiAPIKey, f.toolRegistry, systemPrompt), nil

	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s. Supported providers: deepseek, gemini", f.config.LLMProvider)
	}
}
