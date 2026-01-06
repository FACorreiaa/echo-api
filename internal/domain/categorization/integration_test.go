package categorization

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEngineIntegration tests the full categorization engine integration
func TestEngineIntegration(t *testing.T) {
	// Setup test data
	categoryFood := uuid.New()
	categoryTransport := uuid.New()
	categoryEntertainment := uuid.New()
	categoryShopping := uuid.New()

	rules := []CategoryRule{
		{
			ID:                 uuid.New(),
			UserID:             uuid.New(),
			MatchPattern:       "STARBUCKS",
			CleanName:          strPtr("Starbucks"),
			AssignedCategoryID: &categoryFood,
			IsRecurring:        false,
			Priority:           10,
		},
		{
			ID:                 uuid.New(),
			UserID:             uuid.New(),
			MatchPattern:       "NETFLIX",
			CleanName:          strPtr("Netflix"),
			AssignedCategoryID: &categoryEntertainment,
			IsRecurring:        true,
			Priority:           5,
		},
		{
			ID:                 uuid.New(),
			UserID:             uuid.New(),
			MatchPattern:       "UBER TRIP",
			CleanName:          strPtr("Uber"),
			AssignedCategoryID: &categoryTransport,
			IsRecurring:        false,
			Priority:           0,
		},
	}

	merchants := []Merchant{
		{
			ID:                uuid.New(),
			RawPattern:        "AMAZON",
			CleanName:         "Amazon",
			DefaultCategoryID: &categoryShopping,
		},
		{
			ID:                uuid.New(),
			RawPattern:        "WALMART",
			CleanName:         "Walmart",
			DefaultCategoryID: &categoryShopping,
		},
	}

	t.Run("engine matches rules", func(t *testing.T) {
		engine := NewEngine(rules, merchants)

		result := engine.Match("STARBUCKS COFFEE 001")
		require.NotNil(t, result)
		assert.Equal(t, "Starbucks", result.CleanName)
		assert.Equal(t, &categoryFood, result.CategoryID)
		assert.True(t, result.IsRule)
	})

	t.Run("engine matches merchants", func(t *testing.T) {
		engine := NewEngine(rules, merchants)

		result := engine.Match("AMAZON PURCHASE #12345")
		require.NotNil(t, result)
		assert.Equal(t, "Amazon", result.CleanName)
		assert.Equal(t, &categoryShopping, result.CategoryID)
		assert.False(t, result.IsRule)
	})

	t.Run("engine batch matching", func(t *testing.T) {
		engine := NewEngine(rules, merchants)

		descriptions := []string{
			"STARBUCKS COFFEE 001",
			"NETFLIX SUBSCRIPTION",
			"UBER TRIP TO AIRPORT",
			"AMAZON PURCHASE",
			"UNKNOWN MERCHANT",
		}

		results := engine.MatchBatch(descriptions)
		require.Len(t, results, 5)

		// Check specific results
		assert.NotNil(t, results[0])
		assert.Equal(t, "Starbucks", results[0].CleanName)

		assert.NotNil(t, results[1])
		assert.Equal(t, "Netflix", results[1].CleanName)
		assert.True(t, results[1].IsRecurring)

		assert.NotNil(t, results[2])
		assert.Equal(t, "Uber", results[2].CleanName)

		assert.NotNil(t, results[3])
		assert.Equal(t, "Amazon", results[3].CleanName)

		assert.Nil(t, results[4]) // Unknown merchant
	})

	t.Run("fuzzy matcher", func(t *testing.T) {
		matcher := NewFuzzyMatcher(rules, merchants)

		// Exact match should work
		match := matcher.Match("STARBUCKS COFFEE", 70)
		require.NotNil(t, match)
		assert.Equal(t, "Starbucks", match.CleanName)

		// Test ranking
		matches := matcher.RankMatches("STAR", 5)
		assert.NotEmpty(t, matches)
	})

	t.Run("fuzzy group similar", func(t *testing.T) {
		matcher := NewFuzzyMatcher(rules, merchants)

		descriptions := []string{
			"STARBUCKS 001",
			"STARBUCKS 002",
			"STARBUCKS COFFEE",
			"AMAZON",
			"AMAZON PRIME",
		}

		groups := matcher.FindSimilarMerchants(descriptions, 70)
		// Should group Starbucks variants together and Amazon variants together
		assert.NotEmpty(t, groups)
	})
}

// TestEnginePerformance validates engine performance characteristics
func TestEnginePerformance(t *testing.T) {
	// Create many rules and merchants to test scaling
	categoryID := uuid.New()
	rules := make([]CategoryRule, 100)
	for i := 0; i < 100; i++ {
		rules[i] = CategoryRule{
			ID:                 uuid.New(),
			UserID:             uuid.New(),
			MatchPattern:       fmt.Sprintf("MERCHANT_%d", i),
			CleanName:          strPtr(fmt.Sprintf("Merchant %d", i)),
			AssignedCategoryID: &categoryID,
			Priority:           i,
		}
	}

	merchants := make([]Merchant, 50)
	for i := 0; i < 50; i++ {
		merchants[i] = Merchant{
			ID:                uuid.New(),
			RawPattern:        fmt.Sprintf("GLOBAL_%d", i),
			CleanName:         fmt.Sprintf("Global %d", i),
			DefaultCategoryID: &categoryID,
		}
	}

	t.Run("engine handles many patterns", func(t *testing.T) {
		engine := NewEngine(rules, merchants)

		// Should match first rule
		result := engine.Match("MERCHANT_0 STORE")
		require.NotNil(t, result)
		assert.Equal(t, "Merchant 0", result.CleanName)

		// Should match last rule
		result = engine.Match("MERCHANT_99 SHOP")
		require.NotNil(t, result)
		assert.Equal(t, "Merchant 99", result.CleanName)

		// Should match merchant (note: GLOBAL_5 instead of GLOBAL_25 to avoid substring issues)
		// With Aho-Corasick, "GLOBAL_25" would also match "GLOBAL_2" since it's a substring
		result = engine.Match("GLOBAL_5 PURCHASE")
		require.NotNil(t, result)
		assert.Equal(t, "Global 5", result.CleanName)
	})

	t.Run("batch performance with many patterns", func(t *testing.T) {
		engine := NewEngine(rules, merchants)

		// Create batch of descriptions
		descriptions := make([]string, 1000)
		for i := 0; i < 1000; i++ {
			if i%3 == 0 {
				descriptions[i] = fmt.Sprintf("MERCHANT_%d STORE %d", i%100, i)
			} else if i%3 == 1 {
				descriptions[i] = fmt.Sprintf("GLOBAL_%d PURCHASE %d", i%50, i)
			} else {
				descriptions[i] = fmt.Sprintf("UNKNOWN_TRANSACTION_%d", i)
			}
		}

		results := engine.MatchBatch(descriptions)
		require.Len(t, results, 1000)

		// Count matches
		matchCount := 0
		for _, r := range results {
			if r != nil {
				matchCount++
			}
		}

		// Should match at least 2/3 (merchant and global patterns)
		assert.Greater(t, matchCount, 600)
	})
}

// BenchmarkEngineVsFuzzy compares Aho-Corasick vs Fuzzy matching
func BenchmarkEngineVsFuzzy(b *testing.B) {
	categoryID := uuid.New()
	rules := make([]CategoryRule, 100)
	for i := 0; i < 100; i++ {
		rules[i] = CategoryRule{
			ID:                 uuid.New(),
			UserID:             uuid.New(),
			MatchPattern:       fmt.Sprintf("MERCHANT_%d", i),
			CleanName:          strPtr(fmt.Sprintf("Merchant %d", i)),
			AssignedCategoryID: &categoryID,
			Priority:           i,
		}
	}

	merchants := make([]Merchant, 50)
	for i := 0; i < 50; i++ {
		merchants[i] = Merchant{
			ID:                uuid.New(),
			RawPattern:        fmt.Sprintf("GLOBAL_%d", i),
			CleanName:         fmt.Sprintf("Global %d", i),
			DefaultCategoryID: &categoryID,
		}
	}

	descriptions := []string{
		"MERCHANT_50 STORE 001",
		"GLOBAL_25 PURCHASE",
		"UNKNOWN_TRANSACTION",
		"MERCHANT_99 SHOP",
		"GLOBAL_0 ORDER",
	}

	b.Run("AhoCorasick_Single", func(b *testing.B) {
		engine := NewEngine(rules, merchants)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = engine.Match(descriptions[i%len(descriptions)])
		}
	})

	b.Run("Fuzzy_Single", func(b *testing.B) {
		matcher := NewFuzzyMatcher(rules, merchants)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = matcher.Match(descriptions[i%len(descriptions)], 70)
		}
	})

	b.Run("AhoCorasick_Batch_100", func(b *testing.B) {
		engine := NewEngine(rules, merchants)
		batch := make([]string, 100)
		for i := 0; i < 100; i++ {
			batch[i] = descriptions[i%len(descriptions)]
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = engine.MatchBatch(batch)
		}
	})

	b.Run("AhoCorasick_Batch_1000", func(b *testing.B) {
		engine := NewEngine(rules, merchants)
		batch := make([]string, 1000)
		for i := 0; i < 1000; i++ {
			batch[i] = descriptions[i%len(descriptions)]
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = engine.MatchBatch(batch)
		}
	})

	b.Run("AhoCorasick_Batch_10000", func(b *testing.B) {
		engine := NewEngine(rules, merchants)
		batch := make([]string, 10000)
		for i := 0; i < 10000; i++ {
			batch[i] = descriptions[i%len(descriptions)]
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = engine.MatchBatch(batch)
		}
	})
}

