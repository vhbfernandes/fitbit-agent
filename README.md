# Fitbit Agent

A natural language interface for logging Fitbit meals using AI. Built using extensible, modular architecture

## Features

- **Natural Language Meal Logging**: Describe your meals in plain English
- **Fitbit Integration**: Automatically log meals to your Fitbit account
- **Local Storage**: Save meals locally for backup and offline access
- **Food Database**: Built-in calorie lookup for 40+ common foods
- **Multiple LLM Providers**: Support for DeepSeek/Ollama and Google Gemini
- **Extensible Architecture**: Clean dependency injection and tool discovery system

## Quick Start

### Setup
```bash
# Initialize the project
make setup-dev
make deps

# Set up your Fitbit credentials (will prompt for setup)
export FITBIT_CLIENT_ID="your-client-id"
export FITBIT_CLIENT_SECRET="your-client-secret"

# Choose your LLM provider
export LLM_PROVIDER="deepseek"  # or "gemini"
export GEMINI_API_KEY="your-key"  # if using Gemini

# For local DeepSeek with Ollama
make ollama-start
make setup-ollama
```

### Usage
```bash
# Start the agent
make run

# Or with specific provider
make run-deepseek
make run-gemini
```

## Example Conversations

```
You: I had scrambled eggs with toast and orange juice for breakfast
Agent: I'll log that breakfast to your Fitbit account...
✅ Logged: Scrambled eggs (2 large eggs, ~140 cal), Toast (2 slices whole wheat, ~160 cal), Orange juice (8oz, ~110 cal)

You: For lunch I ate a chicken Caesar salad at that restaurant downtown
Agent: I'll estimate the calories and log this lunch...
✅ Logged: Chicken Caesar Salad (~650 cal) - includes grilled chicken, romaine lettuce, parmesan, croutons, Caesar dressing

You: How many calories are in an apple?
Agent: A medium apple (182g) contains approximately 95 calories.

You: Save my breakfast locally instead of logging to Fitbit
Agent: ✅ Saved breakfast locally: Scrambled eggs and toast (~300 calories)

You: Show me what I've eaten today
Agent: Daily Summary for 2024-01-15:
📅 Breakfast: Scrambled eggs and toast (300 cal)
🥗 Lunch: Chicken Caesar Salad (650 cal)
📊 Total: 950 calories
```

## Architecture

### Core Components
- **Agent Interface**: Main conversation loop
- **LLM Providers**: DeepSeek (via Ollama) and Gemini support
- **Tools**: Fitbit authentication and meal logging
- **Dependency Injection**: Clean, testable architecture

### Available Tools
- `fitbit_login`: Authenticate with Fitbit API
- `fitbit_log_meal`: Log meals with automatic calorie estimation  
- `fitbit_get_profile`: Get user profile and daily goals
- `save_meal_locally`: Save meals to local storage for backup
- `view_daily_summary`: View daily meal summary from local storage
- `lookup_food_calories`: Look up calorie estimates for common foods

## Development

### Adding New Tools
1. Create tool in `pkg/tools/{category}/`
2. Implement the `agent.Tool` interface
3. Register in `pkg/registry/container.go`
4. Restart agent to load

### Project Structure
```
cmd/
├── agent/          # Main CLI application
pkg/
├── agent/          # Core agent interfaces
├── llm/            # LLM provider implementations
├── tools/          # Tool implementations
├── registry/       # Dependency injection
├── input/          # User input providers
└── config/         # Configuration management
```

## Configuration

Environment variables:
- `FITBIT_CLIENT_ID` - Your Fitbit app client ID
- `FITBIT_CLIENT_SECRET` - Your Fitbit app client secret
- `LLM_PROVIDER` - AI provider (deepseek/gemini)
- `GEMINI_API_KEY` - Google Gemini API key
- `OLLAMA_HOST` - Ollama server host (for DeepSeek)
- `SYSTEM_PROMPT_FILE` - Path to custom system prompt

## Fitbit API Setup

1. Go to https://dev.fitbit.com/
2. Create a new application
3. Set redirect URL to `http://localhost:8000/redirect`
4. Copy your Client ID and Client Secret
5. Set environment variables

## License

MIT License - see LICENSE file for details.

If you work on Fitbit:
please incorporate this, make my life easy :D