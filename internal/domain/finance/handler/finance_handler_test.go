package handler

import (
	"testing"
)

func TestParseNaturalLanguage_ExpensesByDefault(t *testing.T) {
	tests := []struct {
		input      string
		wantDesc   string
		wantAmount int64 // Negative for expenses, positive for income
		wantIsNeg  bool
	}{
		// Expenses (default - should be negative)
		{"Coffee 10$", "Coffee", -1000, true},
		{"Rent €500", "Rent", -50000, true},
		{"Lunch 15.50$", "Lunch", -1550, true},
		{"€3.50 coffee", "Coffee", -350, true},
		{"Gas station 45$", "Gas station", -4500, true},

		// Income (with "+" prefix - should be positive)
		{"+Salary 2000$", "Salary", 200000, false},
		{"+200€ refund", "Refund", 20000, false},
		{"+ Bonus 500$", "Bonus", 50000, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseNaturalLanguage(tt.input)

			if result.AmountMinor != tt.wantAmount {
				t.Errorf("parseNaturalLanguage(%q).AmountMinor = %d, want %d",
					tt.input, result.AmountMinor, tt.wantAmount)
			}

			if (result.AmountMinor < 0) != tt.wantIsNeg {
				t.Errorf("parseNaturalLanguage(%q) sign incorrect: got %d (negative=%v), want negative=%v",
					tt.input, result.AmountMinor, result.AmountMinor < 0, tt.wantIsNeg)
			}
		})
	}
}

func TestParseNaturalLanguage_EdgeCases(t *testing.T) {
	// Empty input
	result := parseNaturalLanguage("")
	if result.AmountMinor != 0 {
		t.Errorf("empty input should have 0 amount, got %d", result.AmountMinor)
	}

	// No amount - just description
	result = parseNaturalLanguage("Just a note")
	if result.AmountMinor != 0 {
		t.Errorf("no amount should have 0 amount, got %d", result.AmountMinor)
	}
	if result.Description != "Just a note" {
		t.Errorf("description mismatch: got %q, want %q", result.Description, "Just a note")
	}
}
