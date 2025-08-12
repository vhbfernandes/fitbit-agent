# Fitbit Agent Makefile

.PHONY: help build run test clean deps setup-dev run-deepseek run-gemini demo

# Default target
help:
	@echo "🥗 Fitbit Agent - Make Commands"
	@echo "================================"
	@echo ""
	@echo "📦 Setup & Dependencies:"
	@echo "  setup-dev     - Initialize development environment"
	@echo "  deps          - Download and tidy dependencies"
	@echo "  build         - Build the fitbit-agent binary"
	@echo ""
	@echo "🚀 Running:"
	@echo "  run           - Run with default provider (deepseek)"
	@echo "  run-deepseek  - Run with DeepSeek via Ollama"
	@echo "  run-gemini    - Run with Google Gemini"
	@echo "  demo          - Show demo"
	@echo ""
	@echo "🔧 Development:"
	@echo "  test          - Run tests"
	@echo "  clean         - Clean build artifacts"
	@echo "  fmt           - Format code"
	@echo "  vet           - Run go vet"
	@echo ""
	@echo "📱 Ollama Setup (for DeepSeek):"
	@echo "  ollama-start  - Start Ollama server"
	@echo "  ollama-pull   - Pull DeepSeek model"
	@echo "  ollama-test   - Test Ollama connection"
	@echo ""
	@echo "🔐 Fitbit Setup:"
	@echo "  setup-fitbit  - Guide for Fitbit API setup"

# Setup development environment
setup-dev:
	@echo "🏗️  Setting up development environment..."
	go mod init github.com/vhbfernandes/fitbit-agent 2>/dev/null || true
	$(MAKE) deps

# Download and tidy dependencies
deps:
	@echo "📦 Downloading dependencies..."
	go mod download
	go mod tidy

# Build the binary
build:
	@echo "🔨 Building fitbit-agent..."
	go build -o bin/fitbit-agent ./cmd/agent

# Run with default provider
run: build
	@echo "🚀 Starting Fitbit Agent with DeepSeek..."
	./bin/fitbit-agent

# Run with DeepSeek via Ollama
run-deepseek: build
	@echo "🚀 Starting Fitbit Agent with DeepSeek..."
	LLM_PROVIDER=deepseek ./bin/fitbit-agent

# Run with Gemini
run-gemini: build
	@echo "🚀 Starting Fitbit Agent with Gemini..."
	@if [ -z "$$GEMINI_API_KEY" ]; then \
		echo "❌ GEMINI_API_KEY environment variable is required"; \
		echo "💡 Get your API key from: https://makersuite.google.com/app/apikey"; \
		echo "💡 Then run: export GEMINI_API_KEY='your-key-here'"; \
		exit 1; \
	fi
	LLM_PROVIDER=gemini ./bin/fitbit-agent

# Show demo
demo: build
	@echo "🔍 Running demo..."
	./bin/fitbit-agent demo

# Run tests
test:
	@echo "🧪 Running tests..."
	go test -v ./...

# Format code
fmt:
	@echo "🎨 Formatting code..."
	go fmt ./...

# Run go vet
vet:
	@echo "🔍 Running go vet..."
	go vet ./...

# Clean build artifacts
clean:
	@echo "🧹 Cleaning up..."
	rm -rf bin/
	go clean

# Ollama setup commands
ollama-start:
	@echo "🚀 Starting Ollama server..."
	@if command -v ollama >/dev/null 2>&1; then \
		ollama serve & \
		sleep 2; \
		echo "✅ Ollama server started"; \
	else \
		echo "❌ Ollama not found. Please install it:"; \
		echo "💡 Visit: https://ollama.ai/"; \
	fi

ollama-pull:
	@echo "📥 Pulling DeepSeek R1 model..."
	@if command -v ollama >/dev/null 2>&1; then \
		ollama pull deepseek-r1:7b; \
		echo "✅ DeepSeek model ready"; \
	else \
		echo "❌ Ollama not found. Run 'make ollama-start' first"; \
	fi

ollama-test:
	@echo "🔍 Testing Ollama connection..."
	@if command -v ollama >/dev/null 2>&1; then \
		ollama list; \
		echo "✅ Ollama is working"; \
	else \
		echo "❌ Ollama not found or not running"; \
	fi

# Fitbit setup guide
setup-fitbit:
	@echo "🔐 Fitbit API Setup Guide"
	@echo "========================="
	@echo ""
	@echo "1. 🌐 Go to https://dev.fitbit.com/"
	@echo "2. 📝 Create a new application with these settings:"
	@echo "   - Application Name: Fitbit Agent"
	@echo "   - Description: Natural language meal logging"
	@echo "   - Application Website: http://localhost:8000"
	@echo "   - Organization: Personal"
	@echo "   - OAuth 2.0 Application Type: Personal"
	@echo "   - Callback URL: http://localhost:8000/redirect"
	@echo "   - Default Access Type: Read & Write"
	@echo ""
	@echo "3. 📋 Copy your credentials:"
	@echo "   export FITBIT_CLIENT_ID='your-client-id'"
	@echo "   export FITBIT_CLIENT_SECRET='your-client-secret'"
	@echo ""
	@echo "4. 🚀 Run the agent and use 'fitbit_login' to authenticate"

# CI/CD targets
ci: deps vet test build
	@echo "✅ CI pipeline completed successfully"

# Install binary to PATH
install: build
	@echo "📦 Installing fitbit-agent to /usr/local/bin..."
	sudo cp bin/fitbit-agent /usr/local/bin/
	@echo "✅ fitbit-agent installed. You can now run 'fitbit-agent' from anywhere"

# Uninstall binary from PATH
uninstall:
	@echo "🗑️  Uninstalling fitbit-agent..."
	sudo rm -f /usr/local/bin/fitbit-agent
	@echo "✅ fitbit-agent uninstalled"
