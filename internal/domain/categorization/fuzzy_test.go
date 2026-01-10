package categorization

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFuzzyMatcher_Match(t *testing.T) {
	categoryID := uuid.New()
	merchants := []Merchant{
		{
			ID:                uuid.New(),
			RawPattern:        "STARBUCKS",
			CleanName:         "Starbucks",
			DefaultCategoryID: &categoryID,
		},
		{
			ID:                uuid.New(),
			RawPattern:        "AMAZON",
			CleanName:         "Amazon",
			DefaultCategoryID: &categoryID,
		},
	}

	matcher := NewFuzzyMatcher(nil, merchants)

	t.Run("exact match", func(t *testing.T) {
		result := matcher.Match("STARBUCKS", 70)
		require.NotNil(t, result)
		assert.Equal(t, "Starbucks", result.CleanName)
		assert.Equal(t, 100, result.Score) // Perfect match
	})

	t.Run("contains match - variation with store number", func(t *testing.T) {
		result := matcher.Match("STARBUCKS 001 LONDON", 70)
		require.NotNil(t, result)
		assert.Equal(t, "Starbucks", result.CleanName)
		assert.GreaterOrEqual(t, result.Score, 70)
	})

	t.Run("fuzzy match with typo", func(t *testing.T) {
		result := matcher.Match("STARBACKS", 30) // One letter off - lower threshold for typos
		require.NotNil(t, result)
		assert.Equal(t, "Starbucks", result.CleanName)
	})

	t.Run("no match below threshold", func(t *testing.T) {
		result := matcher.Match("COMPLETELY DIFFERENT", 70)
		assert.Nil(t, result)
	})

	t.Run("case insensitive", func(t *testing.T) {
		result := matcher.Match("starbucks coffee", 70)
		require.NotNil(t, result)
		assert.Equal(t, "Starbucks", result.CleanName)
	})
}

func TestFuzzyMatcher_MatchAll(t *testing.T) {
	merchants := []Merchant{
		{ID: uuid.New(), RawPattern: "STAR", CleanName: "Star"},
		{ID: uuid.New(), RawPattern: "STARBUCKS", CleanName: "Starbucks"},
		{ID: uuid.New(), RawPattern: "AMAZON", CleanName: "Amazon"},
	}

	matcher := NewFuzzyMatcher(nil, merchants)
	results := matcher.MatchAll("STARBUCKS COFFEE", 50)

	// Should match both STAR and STARBUCKS
	assert.GreaterOrEqual(t, len(results), 1)

	// Results should be sorted by score
	for i := 1; i < len(results); i++ {
		assert.GreaterOrEqual(t, results[i-1].Score, results[i].Score)
	}
}

func TestFuzzyMatcher_FindSimilarMerchants(t *testing.T) {
	matcher := NewFuzzyMatcher(nil, nil)

	descriptions := []string{
		"STARBUCKS 001",
		"STARBUCKS 002",
		"STARBUCKS 003",
		"AMAZON PRIME",
		"AMAZON MARKETPLACE",
		"NETFLIX",
	}

	// Lower threshold since variations have numbers
	groups := matcher.FindSimilarMerchants(descriptions, 60)

	// Log all groups for debugging
	t.Logf("Found %d groups:", len(groups))
	for canonical, group := range groups {
		t.Logf("  %q: %v (count: %d)", canonical, group, len(group))
	}

	// Verify we got reasonable groupings
	assert.GreaterOrEqual(t, len(groups), 1, "Should have at least one group")

	// Check that similar descriptions are in the same group
	foundStarbucksGroup := false
	for _, group := range groups {
		starbucksCount := 0
		for _, desc := range group {
			if len(desc) >= 9 && desc[:9] == "STARBUCKS" {
				starbucksCount++
			}
		}
		if starbucksCount >= 2 {
			foundStarbucksGroup = true
			break
		}
	}
	assert.True(t, foundStarbucksGroup, "Starbucks variations should be grouped together")
}

func TestFuzzyMatcher_RankMatches(t *testing.T) {
	merchants := []Merchant{
		{ID: uuid.New(), RawPattern: "STARBUCKS", CleanName: "Starbucks"},
		{ID: uuid.New(), RawPattern: "SUBWAY", CleanName: "Subway"},
		{ID: uuid.New(), RawPattern: "SPOTIFY", CleanName: "Spotify"},
	}

	matcher := NewFuzzyMatcher(nil, merchants)
	results := matcher.RankMatches("STARBUCKS", 10)

	require.GreaterOrEqual(t, len(results), 1)
	assert.Equal(t, "Starbucks", results[0].CleanName)
	assert.Equal(t, 100, results[0].Score)
}

func TestFuzzyMatcher_Priority(t *testing.T) {
	categoryID := uuid.New()

	// Rule should have higher priority than merchant
	rules := []CategoryRule{
		{
			ID:                 uuid.New(),
			UserID:             uuid.New(),
			MatchPattern:       "UBER",
			CleanName:          strPtr("Uber (Rule)"),
			AssignedCategoryID: &categoryID,
			Priority:           10,
		},
	}

	merchants := []Merchant{
		{
			ID:                uuid.New(),
			RawPattern:        "UBER",
			CleanName:         "Uber (Merchant)",
			DefaultCategoryID: &categoryID,
		},
	}

	matcher := NewFuzzyMatcher(rules, merchants)
	results := matcher.MatchAll("UBER TRIP", 50)

	require.GreaterOrEqual(t, len(results), 1)
	// Rule should come first due to higher priority
	assert.Equal(t, "Uber (Rule)", results[0].CleanName)
}

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		s1       string
		s2       string
		expected int
	}{
		{"", "", 0},
		{"abc", "", 3},
		{"", "abc", 3},
		{"abc", "abc", 0},
		{"abc", "abd", 1},        // substitution
		{"abc", "abcd", 1},       // insertion
		{"abcd", "abc", 1},       // deletion
		{"kitten", "sitting", 3}, // classic example
		{"STARBUCKS", "STARBACKS", 1},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s->%s", tc.s1, tc.s2), func(t *testing.T) {
			distance := levenshteinDistance(tc.s1, tc.s2)
			assert.Equal(t, tc.expected, distance)
		})
	}
}

// Benchmark fuzzy matching
func BenchmarkFuzzyMatch(b *testing.B) {
	// Create 1000 merchants
	merchants := make([]Merchant, 1000)
	for i := 0; i < 1000; i++ {
		merchants[i] = Merchant{
			ID:         uuid.New(),
			RawPattern: fmt.Sprintf("MERCHANT_%d", i),
			CleanName:  fmt.Sprintf("Merchant %d", i),
		}
	}
	merchants[500] = Merchant{
		ID:         uuid.New(),
		RawPattern: "STARBUCKS",
		CleanName:  "Starbucks",
	}

	matcher := NewFuzzyMatcher(nil, merchants)
	input := "STARBUCKS COFFEE SHOP"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = matcher.Match(input, 70)
	}
}

func BenchmarkFuzzyMatchAll(b *testing.B) {
	merchants := make([]Merchant, 1000)
	for i := 0; i < 1000; i++ {
		merchants[i] = Merchant{
			ID:         uuid.New(),
			RawPattern: fmt.Sprintf("MERCHANT_%d", i),
			CleanName:  fmt.Sprintf("Merchant %d", i),
		}
	}

	matcher := NewFuzzyMatcher(nil, merchants)
	input := "MERCHANT_500 TRANSACTION"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = matcher.MatchAll(input, 50)
	}
}

func BenchmarkLevenshteinDistance(b *testing.B) {
	s1 := "STARBUCKS COFFEE LONDON GB"
	s2 := "STARBUCKS"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = levenshteinDistance(s1, s2)
	}
}

func BenchmarkFindSimilarMerchants(b *testing.B) {
	// Simulate 100 transaction descriptions
	descriptions := make([]string, 100)
	for i := 0; i < 100; i++ {
		switch i % 5 {
		case 0:
			descriptions[i] = fmt.Sprintf("STARBUCKS %03d", i)
		case 1:
			descriptions[i] = fmt.Sprintf("AMAZON %03d", i)
		case 2:
			descriptions[i] = fmt.Sprintf("NETFLIX %03d", i)
		case 3:
			descriptions[i] = fmt.Sprintf("UBER %03d", i)
		default:
			descriptions[i] = fmt.Sprintf("RANDOM_%d", i)
		}
	}

	matcher := NewFuzzyMatcher(nil, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = matcher.FindSimilarMerchants(descriptions, 70)
	}
}

// Comparison: Fuzzy vs Aho-Corasick
func BenchmarkCompare_AhoCorasick_vs_Fuzzy(b *testing.B) {
	merchants := make([]Merchant, 1000)
	for i := 0; i < 1000; i++ {
		merchants[i] = Merchant{
			ID:         uuid.New(),
			RawPattern: fmt.Sprintf("MERCHANT_%d", i),
			CleanName:  fmt.Sprintf("Merchant %d", i),
		}
	}
	merchants[500] = Merchant{
		ID:         uuid.New(),
		RawPattern: "STARBUCKS",
		CleanName:  "Starbucks",
	}

	engine := NewEngine(nil, merchants)
	fuzzyMatcher := NewFuzzyMatcher(nil, merchants)

	input := "STARBUCKS COFFEE LONDON"

	b.Run("AhoCorasick_Exact", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = engine.Match(input)
		}
	})

	b.Run("Fuzzy_70_Threshold", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = fuzzyMatcher.Match(input, 70)
		}
	})
}
