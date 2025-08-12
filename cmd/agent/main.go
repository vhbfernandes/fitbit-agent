package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vhbfernandes/fitbit-agent/pkg/config"
	"github.com/vhbfernandes/fitbit-agent/pkg/registry"
)

var (
	llmProvider  string
	configFile   string
	verbose      bool
	systemPrompt string
)

var rootCmd = &cobra.Command{
	Use:   "fitbit-agent",
	Short: "A natural language interface for logging Fitbit meals",
	Long: `Fitbit Agent is a nutrition assistant that uses AI to make meal logging effortless.
Just describe what you ate in natural language and it will log it to your Fitbit account.

Supports DeepSeek (via Ollama) and Google Gemini for AI processing.`,
	Run: runAgent,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Fitbit Agent v1.0.0")
		fmt.Println("Built with Go", fmt.Sprintf("%s", "1.24+"))
	},
}

var demoCmd = &cobra.Command{
	Use:   "demo",
	Short: "Run demonstration",
	Long:  "Demonstrates the agent and tool discovery system",
	Run:   runDemo,
}

var createSystemPromptCmd = &cobra.Command{
	Use:   "create-system-prompt [path]",
	Short: "Create a default system prompt file",
	Long:  "Creates a default system_prompt.txt file that you can customize",
	Args:  cobra.MaximumNArgs(1),
	Run:   runCreateSystemPrompt,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&llmProvider, "provider", "p", "", "LLM provider (deepseek, gemini)")
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "config file (default is $HOME/.fitbit-agent.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringVarP(&systemPrompt, "system-prompt", "s", "", "path to system prompt file")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(demoCmd)
	rootCmd.AddCommand(createSystemPromptCmd)
}

func runCreateSystemPrompt(cmd *cobra.Command, args []string) {
	path := "system_prompt.txt"
	if len(args) > 0 {
		path = args[0]
	}

	if err := config.CreateDefaultSystemPromptFile(path); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating system prompt file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Default system prompt created at: %s\n", path)
	fmt.Println("📝 You can now edit this file to customize the system prompt")
	fmt.Println("🔧 The system will automatically load from:")
	fmt.Println("   1. SYSTEM_PROMPT environment variable")
	fmt.Println("   2. system_prompt.txt (current directory)")
	fmt.Println("   3. ~/.fitbit-agent/system_prompt.txt")
	fmt.Println("   4. /etc/fitbit-agent/system_prompt.txt")
}

func runAgent(cmd *cobra.Command, args []string) {
	if verbose {
		log.Println("Starting Fitbit Agent...")
	}

	// Override config with CLI flags
	if llmProvider != "" {
		os.Setenv("LLM_PROVIDER", llmProvider)
	}

	// Override system prompt if specified
	if systemPrompt != "" {
		os.Setenv("SYSTEM_PROMPT_FILE", systemPrompt)
	}

	// Create dependency injection container
	container, err := registry.NewContainer(llmProvider, systemPrompt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating container: %v\n", err)
		os.Exit(1)
	}

	// Get the configured agent
	agent := container.GetAgent()
	if agent == nil {
		// Check if there was an LLM provider error
		if _, err := container.TryGetLLMProvider(); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Cannot start agent: %v\n", err)
			fmt.Println("\n💡 Troubleshooting:")

			cfg := config.LoadConfig()
			switch cfg.LLMProvider {
			case "deepseek":
				fmt.Println("  For DeepSeek (Ollama):")
				fmt.Println("    1. Start Ollama: ollama serve")
				fmt.Println("    2. Pull model: ollama pull deepseek-r1:7b")
				fmt.Println("    3. Test connection: ollama list")
			case "gemini":
				fmt.Println("  For Gemini:")
				fmt.Println("    1. Set API key: export GEMINI_API_KEY='your-key'")
				fmt.Println("    2. Get API key from: https://makersuite.google.com/app/apikey")
			}

			os.Exit(1)
		}

		fmt.Fprintf(os.Stderr, "❌ Agent initialization failed for unknown reason\n")
		os.Exit(1)
	}

	if verbose {
		fmt.Printf("Using LLM provider: %s\n", container.GetLLMProvider().Name())
		fmt.Printf("Available tools: %d\n", len(container.GetToolRegistry().GetAllTools()))

		// Show system prompt info
		cfg := config.LoadConfig()
		if cfg.SystemPrompt.IsDefault() {
			fmt.Println("Using default system prompt")
		} else {
			fmt.Printf("System prompt loaded from: %s\n", cfg.SystemPrompt.GetPath())
		}
	}

	// Run the agent
	if err := agent.Run(context.Background()); err != nil {
		// Check if this is a recoverable error (API issues, etc.)
		if isRecoverableAgentError(err) {
			fmt.Fprintf(os.Stderr, "❌ Agent stopped due to recoverable error: %v\n", err)
			fmt.Println("\n💡 The agent stopped gracefully. You can restart it when the issue is resolved.")
			os.Exit(0) // Exit gracefully, not as a crash
		}

		// For non-recoverable errors, log and exit with error code
		fmt.Fprintf(os.Stderr, "❌ Agent encountered a fatal error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Goodbye! Keep up the healthy eating! 🥗")
}

func runDemo(cmd *cobra.Command, args []string) {
	// Override config with CLI flags
	if llmProvider != "" {
		os.Setenv("LLM_PROVIDER", llmProvider)
	}

	fmt.Println("🥗 Fitbit Agent - Demo")
	fmt.Println("===================================")

	// Load configuration
	cfg := config.LoadConfig()
	fmt.Printf("Configuration: LLM Provider = %s\n", cfg.LLMProvider)

	// Create container (tools will be registered)
	container, err := registry.NewContainer(cfg.LLMProvider, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating container: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n📦 Available Tools:")
	for _, tool := range container.GetToolRegistry().GetAllTools() {
		fmt.Printf("  - %s: %s\n", tool.Name(), tool.Description())
	}

	// Try to get LLM provider info
	fmt.Printf("\n🧠 LLM Provider: ")
	if llmProvider, err := container.TryGetLLMProvider(); err == nil {
		fmt.Printf("%s (ready)\n", llmProvider.Name())
	} else {
		fmt.Printf("%s (not configured - %v)\n", cfg.LLMProvider, err)
	}

	// Show system prompt info
	if cfg.SystemPrompt.IsDefault() {
		fmt.Println("\n📝 System Prompt: Using default (run 'create-system-prompt' to customize)")
	} else {
		fmt.Printf("\n📝 System Prompt: Loaded from %s\n", cfg.SystemPrompt.GetPath())
	}

	// Show Fitbit setup info
	fmt.Println("\n🔗 Fitbit Integration:")
	if cfg.FitbitClientID != "" && cfg.FitbitClientSecret != "" {
		fmt.Println("  ✅ Fitbit credentials configured")
	} else {
		fmt.Println("  ❌ Fitbit credentials not set")
		fmt.Println("     Set FITBIT_CLIENT_ID and FITBIT_CLIENT_SECRET")
	}

	fmt.Println("\n✅ Demo complete!")

	if cfg.GeminiAPIKey == "" {
		fmt.Println("\n💡 To start chatting, set an API key:")
		fmt.Println("   export GEMINI_API_KEY='your-key' (for Gemini)")
		fmt.Println("   Or use --provider deepseek with Ollama for local inference")
	} else {
		fmt.Println("\n🚀 Ready to chat! Use the main command to start:")
		fmt.Printf("   fitbit-agent --provider %s\n", cfg.LLMProvider)
		fmt.Println("\n💬 Try saying: 'I had scrambled eggs and toast for breakfast'")
	}
}

// isRecoverableAgentError checks if an agent error is recoverable (user can retry)
func isRecoverableAgentError(err error) bool {
	errStr := strings.ToLower(err.Error())

	// Check for common recoverable errors
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
		"network",
		"connection",
		"temporary",
	}

	for _, keyword := range recoverableKeywords {
		if strings.Contains(errStr, keyword) {
			return true
		}
	}

	return false
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
