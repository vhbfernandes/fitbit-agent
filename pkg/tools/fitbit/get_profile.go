package fitbit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// GetProfileTool retrieves user profile and daily nutrition stats from Fitbit
type GetProfileTool struct{}

// NewGetProfileTool creates a new profile tool
func NewGetProfileTool() *GetProfileTool {
	return &GetProfileTool{}
}

// Name returns the tool name
func (t *GetProfileTool) Name() string {
	return "fitbit_get_profile"
}

// Description returns the tool description
func (t *GetProfileTool) Description() string {
	return "Get user's Fitbit profile information and daily nutrition progress including calorie goals and current intake."
}

// InputSchema returns the input schema for the tool
func (t *GetProfileTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"date": map[string]interface{}{
				"type":        "string",
				"description": "Date to get nutrition info for (YYYY-MM-DD format, defaults to today)",
				"pattern":     "^\\d{4}-\\d{2}-\\d{2}$",
			},
		},
	}
}

// ProfileInput represents the input for the profile tool
type ProfileInput struct {
	Date string `json:"date,omitempty"`
}

// Execute retrieves the user's profile and nutrition information
func (t *GetProfileTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var profileInput ProfileInput
	if err := json.Unmarshal(input, &profileInput); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// Check if user is authenticated
	token := os.Getenv("FITBIT_ACCESS_TOKEN")
	if token == "" {
		return "❌ Not authenticated with Fitbit. Please run fitbit_login first to connect your account.", nil
	}

	// Use today's date if not specified
	date := profileInput.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	// For now, simulate the profile data since we need OAuth setup
	// In a real implementation, this would call the Fitbit API
	result := fmt.Sprintf(`👤 Fitbit Profile & Daily Progress (%s)

🎯 Daily Goals:
- Calories: 2,000 cal
- Protein: 150g
- Carbs: 250g
- Fat: 67g

📊 Current Progress:
- Calories consumed: 1,250 / 2,000 (63%%)
- Remaining: 750 calories
- Protein: 45g / 150g (30%%)
- Carbs: 125g / 250g (50%%)
- Fat: 35g / 67g (52%%)

🍽️ Today's Meals:
- Breakfast: 350 cal
- Lunch: 550 cal  
- Dinner: 350 cal
- Snacks: 0 cal

💡 You're doing great! You have room for a healthy dinner or snacks to reach your calorie goal.

Note: This is simulated data. Connect your real Fitbit account for actual statistics.`, date)

	return result, nil
}

// validateToken checks if the access token is still valid
func (t *GetProfileTool) validateToken(token string) error {
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
