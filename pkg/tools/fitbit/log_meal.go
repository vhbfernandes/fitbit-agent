package fitbit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/vhbfernandes/fitbit-agent/pkg/config"
)

// LogMealTool handles logging meals to Fitbit
type LogMealTool struct{}

// NewLogMealTool creates a new meal logging tool
func NewLogMealTool() *LogMealTool {
	return &LogMealTool{}
}

// Name returns the tool name
func (t *LogMealTool) Name() string {
	return "fitbit_log_meal"
}

// Description returns the tool description
func (t *LogMealTool) Description() string {
	return "Log a meal to Fitbit with automatic calorie estimation. Accepts natural language descriptions and converts to structured meal data."
}

// InputSchema returns the input schema for the tool
func (t *LogMealTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"meal_type": map[string]interface{}{
				"type":        "string",
				"description": "Type of meal: breakfast, lunch, dinner, or snack",
				"enum":        []string{"breakfast", "lunch", "dinner", "snack"},
			},
			"foods": map[string]interface{}{
				"type":        "array",
				"description": "List of foods in the meal",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Name of the food item",
						},
						"quantity": map[string]interface{}{
							"type":        "number",
							"description": "Quantity/amount of the food",
						},
						"unit": map[string]interface{}{
							"type":        "string",
							"description": "Unit of measurement (e.g., cups, slices, pieces, oz)",
						},
						"calories": map[string]interface{}{
							"type":        "number",
							"description": "Estimated calories for this food item",
						},
					},
					"required": []string{"name", "quantity", "unit", "calories"},
				},
			},
			"meal_time": map[string]interface{}{
				"type":        "string",
				"description": "Time when meal was consumed (optional, defaults to now)",
				"format":      "time",
			},
			"notes": map[string]interface{}{
				"type":        "string",
				"description": "Additional notes about the meal",
			},
		},
		"required": []string{"meal_type", "foods"},
	}
}

// LogMealInput represents the input for meal logging with maximum flexibility
type LogMealInput struct {
	MealType      string     `json:"meal_type"`
	Foods         []FoodItem `json:"foods"`
	Toast         []FoodItem `json:"toast,omitempty"`      // Sometimes LLM puts toast separately
	Snacks        []FoodItem `json:"snacks,omitempty"`     // Sometimes LLM puts snacks separately
	Items         []FoodItem `json:"items,omitempty"`      // Alternative field name
	FoodItems     []FoodItem `json:"food_items,omitempty"` // Alternative field name
	MealTime      string     `json:"meal_time,omitempty"`
	Time          string     `json:"time,omitempty"` // Alternative field name
	Notes         string     `json:"notes,omitempty"`
	Description   string     `json:"description,omitempty"`    // Alternative field name
	TotalCalories any        `json:"total_calories,omitempty"` // For validation
}

// FoodItem represents a single food item with maximum flexibility
type FoodItem struct {
	// Name variations
	Name     string `json:"name,omitempty"`
	FoodItem string `json:"food_item,omitempty"`
	Item     string `json:"item,omitempty"`
	Food     string `json:"food,omitempty"`

	// Quantity variations
	Quantity any `json:"quantity,omitempty"`
	Amount   any `json:"amount,omitempty"`
	Serving  any `json:"serving,omitempty"`
	Count    any `json:"count,omitempty"`

	// Unit variations
	Unit        string `json:"unit,omitempty"`
	Units       string `json:"units,omitempty"`
	Measurement string `json:"measurement,omitempty"`
	Size        string `json:"size,omitempty"`

	// Calories variations
	Calories any `json:"calories,omitempty"`
	Cals     any `json:"cals,omitempty"`
	Cal      any `json:"cal,omitempty"`
	Energy   any `json:"energy,omitempty"`

	// Additional fields that might be included
	Preparation   string `json:"preparation,omitempty"`
	BreadType     string `json:"bread_type,omitempty"`
	CookingMethod string `json:"cooking_method,omitempty"`
}

// ParsedFoodItem represents a parsed food item with consistent types
type ParsedFoodItem struct {
	Name     string
	Quantity float64
	Unit     string
	Calories float64
}

// Execute logs the meal to Fitbit
func (t *LogMealTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	// First, try to handle cases where input is wrapped in an extra "input" field
	var rawInput json.RawMessage = input

	// Check if input is wrapped in {"input": "..."} format
	var wrappedInput struct {
		Input string `json:"input"`
	}
	if err := json.Unmarshal(input, &wrappedInput); err == nil && wrappedInput.Input != "" {
		// Try to parse the wrapped input as JSON
		if json.Valid([]byte(wrappedInput.Input)) {
			rawInput = json.RawMessage(wrappedInput.Input)
		} else {
			// If it's not valid JSON, it might be truncated, return helpful error
			inputPreview := wrappedInput.Input
			if len(inputPreview) > 100 {
				inputPreview = inputPreview[:100] + "..."
			}
			return "", fmt.Errorf("received truncated or invalid JSON input. Please ensure the complete meal data is provided. Got: %s", inputPreview)
		}
	}

	var mealInput LogMealInput
	if err := json.Unmarshal(rawInput, &mealInput); err != nil {
		return "", fmt.Errorf("failed to parse meal input: %w. Raw input: %s", err, string(rawInput))
	}

	// Normalize meal type
	mealType := normalizeMealType(mealInput.MealType)
	if mealType == "" {
		return "", fmt.Errorf("invalid or missing meal type. Must be one of: breakfast, lunch, dinner, snack. Got: %q", mealInput.MealType)
	}

	// Collect all food items from various possible fields
	allFoods := collectAllFoods(mealInput)
	if len(allFoods) == 0 {
		return "", fmt.Errorf("no food items found. Please provide at least one food item")
	}

	// Parse foods into consistent format
	var parsedFoods []ParsedFoodItem
	for i, food := range allFoods {
		parsed, err := t.parseFoodItem(food)
		if err != nil {
			return "", fmt.Errorf("error parsing food item %d (%s): %w", i+1, getAnyFoodName(food), err)
		}
		parsedFoods = append(parsedFoods, parsed)
	}

	// Check authentication first
	if !t.isAuthenticated() {
		return `🔐 Authentication Required!

To log meals to Fitbit, you need to authenticate first. Let me help you with that.

TOOL_CALL: fitbit_login({})

After authentication, I'll log your meal automatically.`, nil
	}

	// Calculate total calories and validate
	totalCalories := 0.0
	for _, food := range parsedFoods {
		totalCalories += food.Calories
	}

	// Validate against expected total if provided
	if mealInput.TotalCalories != nil {
		expectedTotal, err := parseNumberField(mealInput.TotalCalories, "total_calories")
		if err == nil && expectedTotal > 0 {
			diff := totalCalories - expectedTotal
			if diff < -50 || diff > 50 { // Allow 50 calorie difference
				return "", fmt.Errorf("calorie mismatch: calculated %.0f calories but expected %.0f calories", totalCalories, expectedTotal)
			}
		}
	}

	// Make actual API call to Fitbit
	err := t.logMealToFitbit(ctx, mealType, parsedFoods, mealInput)
	if err != nil {
		// If unauthorized, suggest re-authentication
		if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "unauthorized") {
			return `🔐 Authentication Expired!

Your Fitbit access token has expired. Let me help you re-authenticate.

TOOL_CALL: fitbit_login({})

After re-authentication, I'll log your meal automatically.`, nil
		}
		return "", fmt.Errorf("failed to log meal to Fitbit: %w", err)
	}

	// Format success response
	var foodList []string
	for _, food := range parsedFoods {
		foodStr := fmt.Sprintf("- %s (%s %s): ~%.0f cal",
			food.Name,
			formatQuantity(food.Quantity),
			food.Unit,
			food.Calories)
		foodList = append(foodList, foodStr)
	}

	// Get meal time
	mealTime := getMealTime(mealInput)

	result := fmt.Sprintf(`✅ Successfully logged %s to Fitbit (%s):
%s

💯 Total: ~%.0f calories

🎉 Meal logged to your Fitbit account! Check your Fitbit app to see the nutrition data.`,
		mealType,
		mealTime,
		strings.Join(foodList, "\n"),
		totalCalories)

	// Add notes if provided
	notes := getNotes(mealInput)
	if notes != "" {
		result += fmt.Sprintf("\n📝 Notes: %s", notes)
	}

	return result, nil
}

// formatQuantity formats the quantity for display
func formatQuantity(quantity float64) string {
	// If it's a whole number, display without decimals
	if quantity == float64(int(quantity)) {
		return strconv.Itoa(int(quantity))
	}
	return fmt.Sprintf("%.1f", quantity)
}

// parseFoodItem converts a flexible FoodItem to a ParsedFoodItem
func (t *LogMealTool) parseFoodItem(food FoodItem) (ParsedFoodItem, error) {
	var parsed ParsedFoodItem

	// Parse name (try multiple field variations)
	parsed.Name = getAnyFoodName(food)
	if parsed.Name == "" {
		return parsed, fmt.Errorf("food item must have a name")
	}

	// Parse quantity (try multiple field variations)
	quantity, err := getAnyQuantity(food)
	if err != nil {
		return parsed, err
	}
	parsed.Quantity = quantity

	// Parse calories (try multiple field variations)
	calories, err := getAnyCalories(food)
	if err != nil {
		return parsed, err
	}
	parsed.Calories = calories

	// Parse unit (try multiple field variations, with smart defaults)
	parsed.Unit = getAnyUnit(food, parsed.Name)

	return parsed, nil
}

// normalizeMealType standardizes meal type variations
func normalizeMealType(mealType string) string {
	normalized := strings.ToLower(strings.TrimSpace(mealType))

	// Direct matches
	switch normalized {
	case "breakfast", "lunch", "dinner", "snack":
		return normalized
	}

	// Common variations
	switch normalized {
	case "morning", "am":
		return "breakfast"
	case "noon", "midday", "afternoon", "pm":
		return "lunch"
	case "evening", "night", "supper":
		return "dinner"
	case "snacking", "treat", "dessert":
		return "snack"
	}

	return ""
}

// collectAllFoods gathers food items from all possible fields
func collectAllFoods(input LogMealInput) []FoodItem {
	var allFoods []FoodItem

	// Add from main foods field
	allFoods = append(allFoods, input.Foods...)

	// Add from alternative fields
	allFoods = append(allFoods, input.Toast...)
	allFoods = append(allFoods, input.Snacks...)
	allFoods = append(allFoods, input.Items...)
	allFoods = append(allFoods, input.FoodItems...)

	return allFoods
}

// getAnyFoodName extracts food name from any available field
func getAnyFoodName(food FoodItem) string {
	candidates := []string{
		food.Name,
		food.FoodItem,
		food.Item,
		food.Food,
	}

	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) != "" {
			return strings.TrimSpace(candidate)
		}
	}

	return ""
}

// getAnyQuantity extracts quantity from any available field
func getAnyQuantity(food FoodItem) (float64, error) {
	candidates := []any{
		food.Quantity,
		food.Amount,
		food.Serving,
		food.Count,
	}

	for _, candidate := range candidates {
		if candidate != nil {
			if qty, err := parseNumberField(candidate, "quantity"); err == nil && qty > 0 {
				return qty, nil
			}
		}
	}

	// Default to 1 if no quantity found
	return 1.0, nil
}

// getAnyCalories extracts calories from any available field
func getAnyCalories(food FoodItem) (float64, error) {
	candidates := []any{
		food.Calories,
		food.Cals,
		food.Cal,
		food.Energy,
	}

	for _, candidate := range candidates {
		if candidate != nil {
			if cal, err := parseNumberField(candidate, "calories"); err == nil && cal >= 0 {
				return cal, nil
			}
		}
	}

	return 0, fmt.Errorf("calories must be specified")
}

// getAnyUnit extracts unit from any available field with smart defaults
func getAnyUnit(food FoodItem, foodName string) string {
	candidates := []string{
		food.Unit,
		food.Units,
		food.Measurement,
		food.Size,
	}

	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) != "" {
			return normalizeUnit(strings.TrimSpace(candidate))
		}
	}

	// Smart defaults based on food name
	return getDefaultUnit(foodName)
}

// normalizeUnit standardizes unit variations
func normalizeUnit(unit string) string {
	normalized := strings.ToLower(strings.TrimSpace(unit))

	// Common unit normalizations
	switch normalized {
	case "slice", "slices", "piece", "pieces":
		return "slices"
	case "large", "medium", "small", "whole", "egg", "eggs":
		return "large"
	case "cup", "cups", "c":
		return "cups"
	case "tbsp", "tablespoon", "tablespoons":
		return "tbsp"
	case "tsp", "teaspoon", "teaspoons":
		return "tsp"
	case "oz", "ounce", "ounces":
		return "oz"
	case "lb", "pound", "pounds":
		return "lbs"
	case "g", "gram", "grams":
		return "g"
	case "serving", "servings", "portion", "portions":
		return "servings"
	}

	return normalized
}

// getDefaultUnit provides sensible unit defaults based on food name
func getDefaultUnit(foodName string) string {
	name := strings.ToLower(foodName)

	if strings.Contains(name, "toast") || strings.Contains(name, "bread") || strings.Contains(name, "slice") {
		return "slices"
	}
	if strings.Contains(name, "egg") {
		return "large"
	}
	if strings.Contains(name, "cup") || strings.Contains(name, "milk") || strings.Contains(name, "juice") {
		return "cups"
	}

	return "servings"
}

// getMealTime extracts meal time from any available field
func getMealTime(input LogMealInput) string {
	candidates := []string{
		input.MealTime,
		input.Time,
	}

	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) != "" {
			return strings.TrimSpace(candidate)
		}
	}

	return "now"
}

// getNotes extracts notes from any available field
func getNotes(input LogMealInput) string {
	candidates := []string{
		input.Notes,
		input.Description,
	}

	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) != "" {
			return strings.TrimSpace(candidate)
		}
	}

	return ""
}

// parseNumberField converts a flexible number field (string or number) to float64
func parseNumberField(value any, fieldName string) (float64, error) {
	if value == nil {
		return 0, fmt.Errorf("%s is required", fieldName)
	}

	switch v := value.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		// Handle empty strings
		v = strings.TrimSpace(v)
		if v == "" {
			return 0, fmt.Errorf("%s cannot be empty", fieldName)
		}

		// Try direct parsing first
		if num, err := strconv.ParseFloat(v, 64); err == nil {
			return num, nil
		}

		// Handle common text patterns
		v = strings.ToLower(v)

		// Extract number from patterns like "2 large", "1.5 cups", "half", "quarter"
		if num := extractNumberFromText(v); num > 0 {
			return num, nil
		}

		return 0, fmt.Errorf("%s must be a number, got: %s", fieldName, v)
	default:
		return 0, fmt.Errorf("%s must be a number, got type: %T", fieldName, v)
	}
}

// extractNumberFromText extracts numbers from various text patterns
func extractNumberFromText(text string) float64 {
	text = strings.TrimSpace(strings.ToLower(text))

	// Handle word numbers
	wordNumbers := map[string]float64{
		"zero": 0, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
		"six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
		"half": 0.5, "quarter": 0.25, "third": 0.33, "couple": 2, "few": 3,
		"dozen": 12, "pair": 2, "single": 1,
	}

	if num, exists := wordNumbers[text]; exists {
		return num
	}

	// Handle fractions like "1/2", "3/4" BEFORE trying decimal extraction
	fractionRe := regexp.MustCompile(`^(\d+)/(\d+)`)
	if matches := fractionRe.FindStringSubmatch(text); len(matches) > 2 {
		if num, err := strconv.ParseFloat(matches[1], 64); err == nil {
			if denom, err := strconv.ParseFloat(matches[2], 64); err == nil && denom != 0 {
				return num / denom
			}
		}
	}

	// Extract number from beginning of string
	re := regexp.MustCompile(`^(\d+\.?\d*|\d*\.\d+)`)
	if matches := re.FindStringSubmatch(text); len(matches) > 1 {
		if num, err := strconv.ParseFloat(matches[1], 64); err == nil {
			return num
		}
	}

	return 0
}

// isAuthenticated checks if the user has a valid Fitbit access token
func (t *LogMealTool) isAuthenticated() bool {
	// Load config to ensure .env file is processed
	config.LoadConfig()
	token := os.Getenv("FITBIT_ACCESS_TOKEN")
	return token != ""
}

// logMealToFitbit makes the actual API call to Fitbit to log the meal
func (t *LogMealTool) logMealToFitbit(ctx context.Context, mealType string, foods []ParsedFoodItem, input LogMealInput) error {
	config.LoadConfig()
	accessToken := os.Getenv("FITBIT_ACCESS_TOKEN")
	userID := os.Getenv("FITBIT_USER_ID")

	if accessToken == "" {
		return fmt.Errorf("missing FITBIT_ACCESS_TOKEN")
	}
	if userID == "" {
		return fmt.Errorf("missing FITBIT_USER_ID")
	}

	// Get the date for the meal (default to today)
	date := time.Now().Format("2006-01-02")
	if mealTime := getMealTime(input); mealTime != "now" {
		// If a specific time was provided, try to parse it
		// For now, we'll use today's date but this could be enhanced
		date = time.Now().Format("2006-01-02")
	}

	// Log each food item individually to Fitbit
	client := &http.Client{Timeout: 30 * time.Second}

	for _, food := range foods {
		// Convert meal type to Fitbit meal ID
		mealID := getMealID(mealType)

		// Prepare the food data for Fitbit API
		formData := url.Values{}
		formData.Set("foodName", food.Name)
		formData.Set("mealTypeId", mealID)
		formData.Set("unitId", "147") // Generic "serving" unit ID
		formData.Set("amount", fmt.Sprintf("%.2f", food.Quantity))
		formData.Set("date", date)
		formData.Set("calories", fmt.Sprintf("%.0f", food.Calories))

		// Create the request
		apiURL := fmt.Sprintf("https://api.fitbit.com/1/user/%s/foods/log.json", userID)
		req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBufferString(formData.Encode()))
		if err != nil {
			return fmt.Errorf("failed to create request for %s: %w", food.Name, err)
		}

		// Set headers
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		// Make the request
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to log %s to Fitbit: %w", food.Name, err)
		}
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode == 401 {
			return fmt.Errorf("unauthorized: access token may be expired (401)")
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("failed to log %s: HTTP %d", food.Name, resp.StatusCode)
		}

		// Read response body for debugging
		var responseBody bytes.Buffer
		responseBody.ReadFrom(resp.Body)

		// For successful requests, we could parse the response to get the food log ID
		// but for now we'll just check the status code
	}

	return nil
}

// getMealID converts meal type to Fitbit meal type ID
func getMealID(mealType string) string {
	switch strings.ToLower(mealType) {
	case "breakfast":
		return "1"
	case "lunch":
		return "3"
	case "dinner":
		return "4"
	case "snack":
		return "7"
	default:
		return "7" // Default to snack
	}
}
