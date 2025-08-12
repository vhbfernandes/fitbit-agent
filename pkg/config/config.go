package config

import (
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// Config holds application configuration
type Config struct {
	// LLM Configuration
	GeminiAPIKey   string
	DeepSeekAPIKey string
	LLMProvider    string // "deepseek", "gemini"

	// Ollama Configuration
	OllamaHost string

	// Fitbit Configuration
	FitbitClientID     string
	FitbitClientSecret string
	FitbitRedirectURL  string

	// Agent Configuration
	MaxTokens    int64
	Model        string
	SystemPrompt *SystemPrompt
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	// Try to load .env file (ignore error if file doesn't exist)
	if _, err := os.Stat(".env"); err == nil {
		_ = godotenv.Load(".env")
	}

	// Also try to load from common locations
	homeDir, _ := os.UserHomeDir()
	envPaths := []string{
		".env",
		filepath.Join(homeDir, ".fitbit-agent", ".env"),
		"/etc/fitbit-agent/.env",
	}

	for _, path := range envPaths {
		if _, err := os.Stat(path); err == nil {
			_ = godotenv.Load(path)
			break
		}
	}

	return &Config{
		GeminiAPIKey:       os.Getenv("GEMINI_API_KEY"),
		DeepSeekAPIKey:     os.Getenv("DEEPSEEK_API_KEY"),
		LLMProvider:        getEnvWithDefault("LLM_PROVIDER", "deepseek"),
		OllamaHost:         getEnvWithDefault("OLLAMA_HOST", "http://localhost:11434"),
		FitbitClientID:     os.Getenv("FITBIT_CLIENT_ID"),
		FitbitClientSecret: os.Getenv("FITBIT_CLIENT_SECRET"),
		FitbitRedirectURL:  getEnvWithDefault("FITBIT_REDIRECT_URL", "http://localhost:8000/redirect"),
		MaxTokens:          4096,
		Model:              getEnvWithDefault("LLM_MODEL", "deepseek-r1:7b"),
		SystemPrompt:       LoadSystemPrompt(),
	}
}

func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
