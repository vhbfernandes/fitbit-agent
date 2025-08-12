package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ViewSummaryTool shows daily meal summary from local storage
type ViewSummaryTool struct {
	dataDir string
}

// NewViewSummaryTool creates a new summary viewing tool
func NewViewSummaryTool() *ViewSummaryTool {
	homeDir, _ := os.UserHomeDir()
	dataDir := filepath.Join(homeDir, ".fitbit-agent", "meals")

	return &ViewSummaryTool{
		dataDir: dataDir,
	}
}

// Name returns the tool name
func (t *ViewSummaryTool) Name() string {
	return "view_daily_summary"
}

// Description returns the tool description
func (t *ViewSummaryTool) Description() string {
	return "View daily meal summary and calorie totals from local storage. Shows breakdown by meal type."
}

// InputSchema returns the input schema for the tool
func (t *ViewSummaryTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"date": map[string]interface{}{
				"type":        "string",
				"description": "Date to view summary for (YYYY-MM-DD format, defaults to today)",
			},
		},
	}
}

// ViewSummaryInput represents the input for viewing summary
type ViewSummaryInput struct {
	Date string `json:"date,omitempty"`
}

// Execute shows the daily meal summary
func (t *ViewSummaryTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var summaryInput ViewSummaryInput
	if err := json.Unmarshal(input, &summaryInput); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// Use today's date if not specified
	date := summaryInput.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	// Read meals for the day
	filename := fmt.Sprintf("meals_%s.json", date)
	filepath := filepath.Join(t.dataDir, filename)

	data, err := os.ReadFile(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Sprintf("📅 No meals logged for %s\n💡 Start by saying: 'I had [food] for [meal type]'", date), nil
		}
		return "", fmt.Errorf("failed to read meals: %w", err)
	}

	var meals []MealRecord
	if err := json.Unmarshal(data, &meals); err != nil {
		return "", fmt.Errorf("failed to parse meals: %w", err)
	}

	if len(meals) == 0 {
		return fmt.Sprintf("📅 No meals logged for %s\n💡 Start by saying: 'I had [food] for [meal type]'", date), nil
	}

	// Organize meals by type and calculate totals
	mealsByType := make(map[string][]MealRecord)
	totalCalories := 0.0

	for _, meal := range meals {
		if mealData, ok := meal.MealData["meal_type"].(string); ok {
			mealsByType[mealData] = append(mealsByType[mealData], meal)
		}

		// Extract calories if available
		if foods, ok := meal.MealData["foods"].([]interface{}); ok {
			for _, food := range foods {
				if foodMap, ok := food.(map[string]interface{}); ok {
					if calories, ok := foodMap["calories"].(float64); ok {
						totalCalories += calories
					}
				}
			}
		}
	}

	// Build summary
	summary := fmt.Sprintf("📅 Daily Summary for %s\n", date)
	summary += "================================\n\n"

	// Show meals by type
	for _, mealType := range []string{"breakfast", "lunch", "dinner", "snack"} {
		if typeMeals, exists := mealsByType[mealType]; exists {
			summary += fmt.Sprintf("🍽️  **%s** (%d meal%s):\n",
				capitalizeFirst(mealType),
				len(typeMeals),
				pluralize(len(typeMeals)))

			for i, meal := range typeMeals {
				timestamp := meal.Timestamp.Format("15:04")
				summary += fmt.Sprintf("   %d. %s", i+1, timestamp)

				// Show foods if available
				if foods, ok := meal.MealData["foods"].([]interface{}); ok {
					var foodNames []string
					mealCalories := 0.0

					for _, food := range foods {
						if foodMap, ok := food.(map[string]interface{}); ok {
							if name, ok := foodMap["name"].(string); ok {
								foodNames = append(foodNames, name)
							}
							if calories, ok := foodMap["calories"].(float64); ok {
								mealCalories += calories
							}
						}
					}

					if len(foodNames) > 0 {
						summary += fmt.Sprintf(" - %s", joinStrings(foodNames, ", "))
					}
					if mealCalories > 0 {
						summary += fmt.Sprintf(" (~%.0f cal)", mealCalories)
					}
				}
				summary += "\n"
			}
			summary += "\n"
		}
	}

	// Show totals
	summary += "📊 **Daily Totals:**\n"
	summary += fmt.Sprintf("   Total meals: %d\n", len(meals))
	if totalCalories > 0 {
		summary += fmt.Sprintf("   Total calories: ~%.0f cal\n", totalCalories)

		// Add goal comparison if reasonable
		if totalCalories > 500 && totalCalories < 3000 {
			remaining := 2000 - totalCalories // Assume 2000 cal goal
			if remaining > 0 {
				summary += fmt.Sprintf("   Remaining (est.): ~%.0f cal\n", remaining)
			} else {
				summary += fmt.Sprintf("   Over goal (est.): ~%.0f cal\n", -remaining)
			}
		}
	}

	summary += fmt.Sprintf("\n📂 Data stored in: %s", filepath)

	return summary, nil
}

// Helper functions
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return string(s[0]-32) + s[1:] // Simple capitalize
}

func pluralize(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
