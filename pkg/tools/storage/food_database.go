package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// FoodDatabaseTool provides calorie estimates for common foods
type FoodDatabaseTool struct {
	foodData map[string]FoodInfo
}

// FoodInfo represents nutritional information for a food
type FoodInfo struct {
	Name        string   `json:"name"`
	CaloriesPer string   `json:"calories_per"`
	Calories    float64  `json:"calories"`
	Unit        string   `json:"unit"`
	CommonUnits []string `json:"common_units"`
}

// NewFoodDatabaseTool creates a new food database tool
func NewFoodDatabaseTool() *FoodDatabaseTool {
	tool := &FoodDatabaseTool{
		foodData: make(map[string]FoodInfo),
	}
	tool.initializeFoodData()
	return tool
}

// Name returns the tool name
func (t *FoodDatabaseTool) Name() string {
	return "lookup_food_calories"
}

// Description returns the tool description
func (t *FoodDatabaseTool) Description() string {
	return "Look up calorie estimates for common foods. Provides calories per standard serving size and common units."
}

// InputSchema returns the input schema for the tool
func (t *FoodDatabaseTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"food_name": map[string]interface{}{
				"type":        "string",
				"description": "Name of the food to look up",
			},
			"search_terms": map[string]interface{}{
				"type":        "array",
				"description": "Alternative search terms if exact match not found",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
		},
		"required": []string{"food_name"},
	}
}

// LookupInput represents the input for food lookup
type LookupInput struct {
	FoodName    string   `json:"food_name"`
	SearchTerms []string `json:"search_terms,omitempty"`
}

// Execute looks up food calorie information
func (t *FoodDatabaseTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var lookupInput LookupInput
	if err := json.Unmarshal(input, &lookupInput); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	foodName := strings.ToLower(strings.TrimSpace(lookupInput.FoodName))

	// Try exact match first
	if food, exists := t.foodData[foodName]; exists {
		return t.formatFoodInfo(food), nil
	}

	// Try partial matches
	var matches []FoodInfo
	for key, food := range t.foodData {
		if strings.Contains(key, foodName) || strings.Contains(foodName, key) {
			matches = append(matches, food)
		}
	}

	// Try search terms if provided
	for _, term := range lookupInput.SearchTerms {
		term = strings.ToLower(strings.TrimSpace(term))
		if food, exists := t.foodData[term]; exists {
			matches = append(matches, food)
		}
	}

	if len(matches) == 0 {
		return fmt.Sprintf("❌ No calorie data found for '%s'.\n💡 Try searching for:\n- Basic food names (e.g., 'chicken' instead of 'grilled chicken breast')\n- Common foods (e.g., 'egg', 'bread', 'rice')\n- Use general estimates: 100-200 cal for small items, 300-600 cal for meals", lookupInput.FoodName), nil
	}

	// Return best matches
	result := fmt.Sprintf("🔍 Found %d match(es) for '%s':\n\n", len(matches), lookupInput.FoodName)
	for i, food := range matches {
		if i >= 3 { // Limit to top 3 matches
			break
		}
		result += t.formatFoodInfo(food) + "\n"
	}

	return result, nil
}

func (t *FoodDatabaseTool) formatFoodInfo(food FoodInfo) string {
	result := fmt.Sprintf("🍽️  **%s**: %.0f cal per %s", food.Name, food.Calories, food.CaloriesPer)
	if len(food.CommonUnits) > 0 {
		result += fmt.Sprintf("\n   Common units: %s", strings.Join(food.CommonUnits, ", "))
	}
	return result
}

func (t *FoodDatabaseTool) initializeFoodData() {
	// Basic foods database - common items with calorie estimates
	foods := []FoodInfo{
		// Eggs & Dairy
		{"egg", "1 large egg", 70, "each", []string{"piece", "large", "medium"}},
		{"milk", "1 cup", 150, "cup", []string{"glass", "8oz"}},
		{"cheese", "1 oz", 110, "oz", []string{"slice", "cube"}},
		{"yogurt", "1 cup", 150, "cup", []string{"container"}},
		{"butter", "1 tbsp", 100, "tbsp", []string{"pat"}},

		// Grains & Bread
		{"bread", "1 slice", 80, "slice", []string{"piece"}},
		{"rice", "1 cup cooked", 205, "cup", []string{"serving"}},
		{"pasta", "1 cup cooked", 220, "cup", []string{"serving"}},
		{"oatmeal", "1 cup cooked", 150, "cup", []string{"bowl"}},
		{"bagel", "1 medium", 250, "each", []string{"whole"}},
		{"toast", "1 slice", 80, "slice", []string{"piece"}},

		// Proteins
		{"chicken breast", "3 oz cooked", 140, "3oz", []string{"piece", "serving"}},
		{"ground beef", "3 oz cooked", 230, "3oz", []string{"serving"}},
		{"salmon", "3 oz cooked", 175, "3oz", []string{"fillet", "serving"}},
		{"tuna", "3 oz", 100, "3oz", []string{"can", "serving"}},
		{"beans", "1/2 cup", 120, "1/2 cup", []string{"serving"}},

		// Fruits
		{"apple", "1 medium", 80, "each", []string{"whole", "medium"}},
		{"banana", "1 medium", 105, "each", []string{"whole", "medium"}},
		{"orange", "1 medium", 60, "each", []string{"whole", "medium"}},
		{"berries", "1 cup", 80, "cup", []string{"handful"}},
		{"grapes", "1 cup", 60, "cup", []string{"handful"}},

		// Vegetables
		{"broccoli", "1 cup", 25, "cup", []string{"serving"}},
		{"carrots", "1 cup", 50, "cup", []string{"serving"}},
		{"lettuce", "1 cup", 10, "cup", []string{"serving"}},
		{"potato", "1 medium", 160, "each", []string{"whole", "medium"}},
		{"tomato", "1 medium", 25, "each", []string{"whole"}},

		// Snacks & Others
		{"peanut butter", "2 tbsp", 190, "2 tbsp", []string{"serving"}},
		{"nuts", "1 oz", 170, "oz", []string{"handful", "small bag"}},
		{"chips", "1 oz", 150, "oz", []string{"small bag", "handful"}},
		{"chocolate", "1 oz", 150, "oz", []string{"square", "piece"}},
		{"ice cream", "1/2 cup", 140, "1/2 cup", []string{"scoop"}},

		// Beverages
		{"coffee", "1 cup black", 5, "cup", []string{"mug"}},
		{"orange juice", "8 oz", 110, "glass", []string{"cup", "8oz"}},
		{"soda", "12 oz", 150, "can", []string{"bottle"}},
		{"beer", "12 oz", 150, "bottle", []string{"can"}},
		{"wine", "5 oz", 125, "glass", []string{"serving"}},
	}

	for _, food := range foods {
		key := strings.ToLower(food.Name)
		t.foodData[key] = food

		// Add common variations
		if food.Name == "egg" {
			t.foodData["eggs"] = food
		}
		if food.Name == "bread" {
			t.foodData["toast"] = FoodInfo{
				Name: "toast", CaloriesPer: "1 slice", Calories: 80, Unit: "slice", CommonUnits: []string{"piece"},
			}
		}
	}
}
