package config

import (
	"os"
	"path/filepath"
	"strings"
)

// SystemPrompt handles loading and managing system prompts
type SystemPrompt struct {
	content string
	path    string
}

// LoadSystemPrompt loads system prompt from various sources
func LoadSystemPrompt() *SystemPrompt {
	sp := &SystemPrompt{}

	// Try loading from environment variable first
	if envPrompt := os.Getenv("SYSTEM_PROMPT"); envPrompt != "" {
		sp.content = envPrompt
		sp.path = "environment"
		return sp
	}

	// Try loading from file specified in environment variable
	if envFile := os.Getenv("SYSTEM_PROMPT_FILE"); envFile != "" {
		if content, err := os.ReadFile(envFile); err == nil {
			sp.content = strings.TrimSpace(string(content))
			sp.path = envFile
			return sp
		}
	}

	// Try loading from file paths in order of preference
	paths := []string{
		"system_prompt.txt",
		".fitbit-agent/system_prompt.txt",
		filepath.Join(os.Getenv("HOME"), ".fitbit-agent", "system_prompt.txt"),
		"/etc/fitbit-agent/system_prompt.txt",
	}

	for _, path := range paths {
		if content, err := os.ReadFile(path); err == nil {
			sp.content = strings.TrimSpace(string(content))
			sp.path = path
			return sp
		}
	}

	// Default system prompt if none found
	sp.content = sp.getDefaultSystemPrompt()
	sp.path = "default"

	return sp
}

// GetContent returns the system prompt content
func (sp *SystemPrompt) GetContent() string {
	return sp.content
}

// GetPath returns where the system prompt was loaded from
func (sp *SystemPrompt) GetPath() string {
	return sp.path
}

// IsDefault returns true if using the default system prompt
func (sp *SystemPrompt) IsDefault() bool {
	return sp.path == "default"
}

// SaveToFile saves the current system prompt to a file
func (sp *SystemPrompt) SaveToFile(path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(sp.content), 0644)
}

// CreateDefaultSystemPromptFile creates a default system prompt file
func CreateDefaultSystemPromptFile(path string) error {
	sp := &SystemPrompt{}
	sp.content = sp.getDefaultSystemPrompt()
	return sp.SaveToFile(path)
}

// getDefaultSystemPrompt returns the default system prompt
func (sp *SystemPrompt) getDefaultSystemPrompt() string {
	return `You are Fitbit Agent, an intelligent personal nutrition assistant with access to Fitbit API tools.

## Your Role
- Help users log meals and track nutrition using natural language
- Make meal logging as simple as saying "I had a turkey sandwich for lunch"
- Provide calorie estimates and nutritional guidance
- Support healthy eating habits through easy tracking

## Available Tools
You have access to several tools:
- **Fitbit Integration**: fitbit_login, fitbit_log_meal, fitbit_get_profile
- **File Operations**: read_file, write_file for meal templates and preferences

## Guidelines
1. **Log Meals Immediately**: When users describe meals, log them right away
2. **Estimate Calories**: Provide reasonable calorie estimates for all foods
3. **Be Encouraging**: Support healthy choices and positive habits
4. **Ask for Clarification**: Only when meal descriptions are unclear
5. **Explain Estimates**: Help users learn about nutrition

## Response Style
- Be friendly and encouraging
- Provide specific calorie breakdowns
- Use emojis to make interactions fun (🥗 🍎 ✅)
- Celebrate healthy choices
- Be helpful without being preachy

Remember: Your goal is to make nutrition tracking effortless and encourage healthy eating habits.`
}
