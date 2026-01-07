package normalizer

import (
	"testing"
)

func TestMerchantSanitizer_Sanitize(t *testing.T) {
	sanitizer := NewMerchantSanitizer()

	tests := []struct {
		name           string
		input          string
		expectedName   string
		expectedCat    string
		expectedSubcat string
	}{
		{
			name:           "Pingo Doce with prefix",
			input:          "COMPRA PGO DOCE ALVALADE 123456",
			expectedName:   "Pingo Doce",
			expectedCat:    "Groceries",
			expectedSubcat: "Supermarket",
		},
		{
			name:           "Lidl simple",
			input:          "LIDL LISBOA",
			expectedName:   "Lidl",
			expectedCat:    "Groceries",
			expectedSubcat: "Supermarket",
		},
		{
			name:           "Netflix subscription",
			input:          "NETFLIX.COM",
			expectedName:   "Netflix",
			expectedCat:    "Entertainment",
			expectedSubcat: "Streaming",
		},
		{
			name:           "Uber ride",
			input:          "UBER *TRIP 12/01",
			expectedName:   "Uber",
			expectedCat:    "Transport",
			expectedSubcat: "Rideshare",
		},
		{
			name:           "Uber Eats delivery",
			input:          "UBER EATS",
			expectedName:   "Uber Eats",
			expectedCat:    "Food & Drink",
			expectedSubcat: "Delivery",
		},
		{
			name:           "Unknown merchant gets title case",
			input:          "SOME RANDOM STORE 456789",
			expectedName:   "Some Random Store",
			expectedCat:    "",
			expectedSubcat: "",
		},
		{
			name:           "McDonald's with prefix",
			input:          "COMPRAS MC DONALDS COLOMBO",
			expectedName:   "McDonald's",
			expectedCat:    "Food & Drink",
			expectedSubcat: "Fast Food",
		},
		{
			name:           "Revolut",
			input:          "REVOLUT",
			expectedName:   "Revolut",
			expectedCat:    "Finance",
			expectedSubcat: "Digital Bank",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizer.Sanitize(tt.input)

			if result.NormalizedName != tt.expectedName {
				t.Errorf("NormalizedName = %q, want %q", result.NormalizedName, tt.expectedName)
			}
			if result.Category != tt.expectedCat {
				t.Errorf("Category = %q, want %q", result.Category, tt.expectedCat)
			}
			if result.Subcategory != tt.expectedSubcat {
				t.Errorf("Subcategory = %q, want %q", result.Subcategory, tt.expectedSubcat)
			}
		})
	}
}

func TestCleanMerchantName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"COMPRA PINGO DOCE 123456", "PINGO DOCE"},
		{"PAG STARBUCKS 12/01", "STARBUCKS"},
		{"MB WAY RESTAURANTE XYZ", "RESTAURANTE XYZ"},
		{"  LIDL  PORTO  ", "LIDL PORTO"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := cleanMerchantName(tt.input)
			if result != tt.expected {
				t.Errorf("cleanMerchantName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
