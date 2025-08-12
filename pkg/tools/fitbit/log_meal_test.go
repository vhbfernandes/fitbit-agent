package fitbit

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestLogMealFlexibility(t *testing.T) {
	tool := NewLogMealTool()

	testCases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "Original format that was failing",
			input:   `{"meal_type": "breakfast", "foods": [{"food_item": "scrambled eggs", "quantity": "2 large"}], "toast": [{"food_item": "toast", "quantity": "2 slices"}], "total_calories": "440"}`,
			wantErr: true, // Should fail because calories missing
		},
		{
			name:    "Standard format",
			input:   `{"meal_type": "breakfast", "foods": [{"name": "scrambled eggs", "quantity": 2, "unit": "large", "calories": 140}, {"name": "toast", "quantity": 2, "unit": "slices", "calories": 160}]}`,
			wantErr: false,
		},
		{
			name:    "Mixed field names",
			input:   `{"meal_type": "breakfast", "foods": [{"food_item": "scrambled eggs", "amount": "two", "calories": "140"}], "toast": [{"item": "toast", "count": 2, "unit": "slices", "cals": 160}]}`,
			wantErr: false,
		},
		{
			name:    "String numbers",
			input:   `{"meal_type": "breakfast", "foods": [{"name": "eggs", "quantity": "2", "calories": "140"}, {"name": "toast", "quantity": "2", "calories": "160"}]}`,
			wantErr: false,
		},
		{
			name:    "Alternative meal type",
			input:   `{"meal_type": "morning", "foods": [{"name": "cereal", "quantity": 1, "unit": "bowl", "calories": 200}]}`,
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), json.RawMessage(tc.input))

			if tc.wantErr && err == nil {
				t.Errorf("Expected error but got none. Result: %s", result)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if !tc.wantErr && err == nil {
				// Should contain authentication message since no token set
				if !strings.Contains(result, "Authentication Required") {
					t.Errorf("Expected authentication message in result: %s", result)
				}
			}
		})
	}
}

func TestNumberParsing(t *testing.T) {
	testCases := []struct {
		input    any
		expected float64
		wantErr  bool
	}{
		{2, 2.0, false},
		{2.5, 2.5, false},
		{"2", 2.0, false},
		{"2.5", 2.5, false},
		{"two", 2.0, false},
		{"half", 0.5, false},
		{"1/2", 0.5, false},
		{"2 large", 2.0, false},
		{"1.5 cups", 1.5, false},
		{"", 0.0, true},
		{nil, 0.0, true},
	}

	for _, tc := range testCases {
		result, err := parseNumberField(tc.input, "test")

		if tc.wantErr && err == nil {
			t.Errorf("Input %v: expected error but got none", tc.input)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("Input %v: expected no error but got: %v", tc.input, err)
		}
		if !tc.wantErr && result != tc.expected {
			t.Errorf("Input %v: expected %f but got %f", tc.input, tc.expected, result)
		}
	}
}
