package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SaveMealTool saves meals to local file storage
type SaveMealTool struct {
	dataDir string
}

// NewSaveMealTool creates a new meal saving tool
func NewSaveMealTool() *SaveMealTool {
	homeDir, _ := os.UserHomeDir()
	dataDir := filepath.Join(homeDir, ".fitbit-agent", "meals")

	// Ensure directory exists
	os.MkdirAll(dataDir, 0755)

	return &SaveMealTool{
		dataDir: dataDir,
	}
}

// Name returns the tool name
func (t *SaveMealTool) Name() string {
	return "save_meal_locally"
}

// Description returns the tool description
func (t *SaveMealTool) Description() string {
	return "Save meal data to local file storage. Useful when Fitbit is not available or for backup purposes."
}

// InputSchema returns the input schema for the tool
func (t *SaveMealTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"meal_data": map[string]interface{}{
				"type":        "object",
				"description": "Complete meal data to save",
			},
			"date": map[string]interface{}{
				"type":        "string",
				"description": "Date for the meal (YYYY-MM-DD format, defaults to today)",
			},
		},
		"required": []string{"meal_data"},
	}
}

// SaveMealInput represents the input for saving meals
type SaveMealInput struct {
	MealData map[string]interface{} `json:"meal_data"`
	Date     string                 `json:"date,omitempty"`
}

// MealRecord represents a saved meal record
type MealRecord struct {
	Timestamp time.Time              `json:"timestamp"`
	Date      string                 `json:"date"`
	MealData  map[string]interface{} `json:"meal_data"`
}

// Execute saves the meal to local storage
func (t *SaveMealTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var saveInput SaveMealInput
	if err := json.Unmarshal(input, &saveInput); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// Use today's date if not specified
	date := saveInput.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	// Create meal record
	record := MealRecord{
		Timestamp: time.Now(),
		Date:      date,
		MealData:  saveInput.MealData,
	}

	// Save to file (one file per day)
	filename := fmt.Sprintf("meals_%s.json", date)
	filepath := filepath.Join(t.dataDir, filename)

	// Read existing meals for the day
	var meals []MealRecord
	if existingData, err := os.ReadFile(filepath); err == nil {
		json.Unmarshal(existingData, &meals)
	}

	// Append new meal
	meals = append(meals, record)

	// Write back to file
	data, err := json.MarshalIndent(meals, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal meal data: %w", err)
	}

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to save meal: %w", err)
	}

	return fmt.Sprintf("✅ Meal saved locally to %s\n📂 File: %s\n🕒 Total meals today: %d",
		date, filepath, len(meals)), nil
}
