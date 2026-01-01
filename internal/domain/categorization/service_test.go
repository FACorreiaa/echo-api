package categorization

import (
	"testing"
)

func TestCleanDescription(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "removes COMPRAS prefix",
			input:    "COMPRAS C.DEB APPLE.COM",
			expected: "Apple.com",
		},
		{
			name:     "removes PURCHASE prefix",
			input:    "PURCHASE STARBUCKS COFFEE",
			expected: "Starbucks Coffee",
		},
		{
			name:     "removes trailing reference",
			input:    "NETFLIX*1234",
			expected: "Netflix",
		},
		{
			name:     "title cases result",
			input:    "UBER TRIP",
			expected: "Uber Trip",
		},
		{
			name:     "handles already clean input",
			input:    "Spotify",
			expected: "Spotify",
		},
		{
			name:     "removes POS prefix",
			input:    "POS MCDONALDS",
			expected: "Mcdonalds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanDescription(tt.input)
			if result != tt.expected {
				t.Errorf("cleanDescription(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name        string
		description string
		pattern     string
		shouldMatch bool
	}{
		{
			name:        "exact match",
			description: "NETFLIX",
			pattern:     "NETFLIX",
			shouldMatch: true,
		},
		{
			name:        "contains match with wildcards",
			description: "COMPRAS C.DEB NETFLIX.COM/BILL",
			pattern:     "%NETFLIX%",
			shouldMatch: true,
		},
		{
			name:        "suffix match",
			description: "APPLE.COM/BILL",
			pattern:     "%APPLE%",
			shouldMatch: true,
		},
		{
			name:        "no match",
			description: "STARBUCKS COFFEE",
			pattern:     "%NETFLIX%",
			shouldMatch: false,
		},
		{
			name:        "case insensitive",
			description: "netflix subscription",
			pattern:     "%NETFLIX%",
			shouldMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchPattern(tt.description, tt.pattern)
			if result != tt.shouldMatch {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.description, tt.pattern, result, tt.shouldMatch)
			}
		})
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"1234", true},
		{"0", true},
		{"123abc", false},
		{"", false},
		{"12.34", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isNumeric(tt.input)
			if result != tt.expected {
				t.Errorf("isNumeric(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestToTitleCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"HELLO WORLD", "Hello World"},
		{"hello world", "Hello World"},
		{"HeLLo WoRLD", "Hello World"},
		{"a", "A"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toTitleCase(tt.input)
			if result != tt.expected {
				t.Errorf("toTitleCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
