package fitbit

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/vhbfernandes/fitbit-agent/pkg/config"
)

// LoginTool handles Fitbit OAuth authentication
type LoginTool struct{}

// NewLoginTool creates a new Fitbit login tool
func NewLoginTool() *LoginTool {
	return &LoginTool{}
}

// Name returns the tool name
func (t *LoginTool) Name() string {
	return "fitbit_login"
}

// Description returns the tool description
func (t *LoginTool) Description() string {
	return "Authenticate with Fitbit API to enable meal logging. Guides user through OAuth flow."
}

// InputSchema returns the input schema for the tool
func (t *LoginTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"force_reauth": map[string]interface{}{
				"type":        "boolean",
				"description": "Force re-authentication even if already logged in",
				"default":     false,
			},
		},
	}
}

// LoginInput represents the input for the login tool
type LoginInput struct {
	ForceReauth bool `json:"force_reauth"`
}

// Execute performs the Fitbit login process
func (t *LoginTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var loginInput LoginInput
	if err := json.Unmarshal(input, &loginInput); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// Load configuration to get credentials from .env file
	cfg := config.LoadConfig()

	// Check if credentials are configured
	if cfg.FitbitClientID == "" || cfg.FitbitClientSecret == "" {
		return "", fmt.Errorf("Fitbit credentials not configured. Please set FITBIT_CLIENT_ID and FITBIT_CLIENT_SECRET environment variables.\n\nTo get these:\n1. Go to https://dev.fitbit.com/\n2. Create a new application\n3. Set redirect URL to: %s\n4. Copy your Client ID and Client Secret", cfg.FitbitRedirectURL)
	}

	// Check if already authenticated (unless forcing reauth)
	if !loginInput.ForceReauth {
		if token := os.Getenv("FITBIT_ACCESS_TOKEN"); token != "" {
			// Validate the token
			if err := t.validateToken(token); err == nil {
				return "✅ Already authenticated with Fitbit! You can start logging meals.", nil
			}
			// If token is invalid, continue with authentication
		}
	}

	// Generate OAuth URL
	authURL := fmt.Sprintf(
		"https://www.fitbit.com/oauth2/authorize?response_type=code&client_id=%s&redirect_uri=%s&scope=nutrition",
		cfg.FitbitClientID,
		cfg.FitbitRedirectURL,
	)

	// Start the OAuth callback server
	authCode, err := t.startOAuthServer(ctx, cfg, authURL)
	if err != nil {
		return "", fmt.Errorf("OAuth flow failed: %w", err)
	}

	// Exchange the authorization code for an access token
	accessToken, err := t.exchangeCodeForToken(cfg, authCode)
	if err != nil {
		return "", fmt.Errorf("failed to exchange code for token: %w", err)
	}

	// Save the access token
	if err := os.Setenv("FITBIT_ACCESS_TOKEN", accessToken); err != nil {
		return "", fmt.Errorf("failed to save access token: %w", err)
	}

	return `✅ Successfully authenticated with Fitbit! 

🎉 Your access token has been saved and you're now ready to log meals.
💪 Try saying: "I had oatmeal for breakfast" to test meal logging.

Your authentication will be remembered for future sessions.`, nil
}

// validateToken checks if the access token is still valid
func (t *LoginTool) validateToken(token string) error {
	req, err := http.NewRequest("GET", "https://api.fitbit.com/1/user/-/profile.json", nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token validation failed with status %d", resp.StatusCode)
	}

	return nil
}

// startOAuthServer starts a temporary web server to handle OAuth callback
func (t *LoginTool) startOAuthServer(ctx context.Context, cfg *config.Config, authURL string) (string, error) {
	// Parse the redirect URL to get the port
	redirectURL, err := url.Parse(cfg.FitbitRedirectURL)
	if err != nil {
		return "", fmt.Errorf("invalid redirect URL: %w", err)
	}

	// Channel to receive the authorization code
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	// Create HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errMsg := r.URL.Query().Get("error")
			if errMsg == "" {
				errMsg = "No authorization code received"
			}
			errChan <- fmt.Errorf("OAuth error: %s", errMsg)
			http.Error(w, "Authorization failed", http.StatusBadRequest)
			return
		}

		// Success page
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>Fitbit Authentication Success</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; padding: 50px; background-color: #f0f8ff; }
        .success { color: #4CAF50; font-size: 24px; margin-bottom: 20px; }
        .message { color: #333; font-size: 16px; }
    </style>
</head>
<body>
    <div class="success">Authentication Successful!</div>
    <div class="message">
        You can now close this browser tab and return to the Fitbit Agent.<br>
        Your authentication has been completed successfully.
    </div>
</body>
</html>`)

		// Send the code
		codeChan <- code
	})

	// Start server
	server := &http.Server{
		Addr:    redirectURL.Host,
		Handler: mux,
	}

	// Start server in goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("server error: %w", err)
		}
	}()

	// Give server a moment to start
	time.Sleep(100 * time.Millisecond)

	// Open browser (macOS specific)
	fmt.Printf("🌐 Opening browser for Fitbit authentication...\n")
	if err := exec.Command("open", authURL).Start(); err != nil {
		fmt.Printf("⚠️  Could not automatically open browser. Please visit: %s\n", authURL)
	}

	fmt.Printf("🔄 Waiting for authorization (server running on %s)...\n", redirectURL.Host)

	// Wait for either code or error
	var authCode string
	select {
	case code := <-codeChan:
		authCode = code
	case err := <-errChan:
		server.Shutdown(ctx)
		return "", err
	case <-time.After(5 * time.Minute): // Timeout after 5 minutes
		server.Shutdown(ctx)
		return "", fmt.Errorf("authentication timeout - please try again")
	case <-ctx.Done():
		server.Shutdown(ctx)
		return "", ctx.Err()
	}

	// Shutdown server
	server.Shutdown(ctx)
	return authCode, nil
}

// exchangeCodeForToken exchanges the authorization code for an access token
func (t *LoginTool) exchangeCodeForToken(cfg *config.Config, authCode string) (string, error) {
	// Prepare token exchange request
	data := url.Values{}
	data.Set("client_id", cfg.FitbitClientID)
	data.Set("grant_type", "authorization_code")
	data.Set("redirect_uri", cfg.FitbitRedirectURL)
	data.Set("code", authCode)

	// Create request
	req, err := http.NewRequest("POST", "https://api.fitbit.com/oauth2/token", strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+t.basicAuth(cfg.FitbitClientID, cfg.FitbitClientSecret))

	// Make request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token request failed with status %d", resp.StatusCode)
	}

	// Parse response
	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	return tokenResp.AccessToken, nil
}

// basicAuth creates Basic authentication header value
func (t *LoginTool) basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}
