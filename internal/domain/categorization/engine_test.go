package categorization

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test basic matching functionality
func TestEngine_Match(t *testing.T) {
	categoryID := uuid.New()
	rules := []CategoryRule{
		{
			ID:                 uuid.New(),
			UserID:             uuid.New(),
			MatchPattern:       "%REVOLUT%",
			CleanName:          strPtr("Revolut"),
			AssignedCategoryID: &categoryID,
			IsRecurring:        true,
			Priority:           10,
		},
	}

	merchants := []Merchant{
		{
			ID:                uuid.New(),
			RawPattern:        "%STARBUCKS%",
			CleanName:         "Starbucks",
			DefaultCategoryID: &categoryID,
			IsSystem:          true,
		},
	}

	engine := NewEngine(rules, merchants)

	t.Run("matches rule pattern", func(t *testing.T) {
		result := engine.Match("CARD PURCHASE 27/12/2025 CAR WAL CRT DEB REVOLUT LONDON GB")
		require.NotNil(t, result)
		assert.Equal(t, "Revolut", result.CleanName)
		assert.True(t, result.IsRecurring)
		assert.True(t, result.IsRule)
	})

	t.Run("matches merchant pattern", func(t *testing.T) {
		result := engine.Match("POS STARBUCKS COFFEE #1234")
		require.NotNil(t, result)
		assert.Equal(t, "Starbucks", result.CleanName)
		assert.False(t, result.IsRule)
	})

	t.Run("returns nil for no match", func(t *testing.T) {
		result := engine.Match("RANDOM TRANSACTION WITH NO MATCH")
		assert.Nil(t, result)
	})

	t.Run("case insensitive matching", func(t *testing.T) {
		result := engine.Match("payment to revolut for subscription")
		require.NotNil(t, result)
		assert.Equal(t, "Revolut", result.CleanName)
	})
}

// Test priority handling
func TestEngine_Priority(t *testing.T) {
	categoryID1 := uuid.New()
	categoryID2 := uuid.New()

	// Rule should take priority over merchant for the same pattern
	rules := []CategoryRule{
		{
			ID:                 uuid.New(),
			UserID:             uuid.New(),
			MatchPattern:       "%NETFLIX%",
			CleanName:          strPtr("Netflix (Rule)"),
			AssignedCategoryID: &categoryID1,
			Priority:           5,
		},
	}

	merchants := []Merchant{
		{
			ID:                uuid.New(),
			RawPattern:        "%NETFLIX%",
			CleanName:         "Netflix (Merchant)",
			DefaultCategoryID: &categoryID2,
			IsSystem:          true,
		},
	}

	engine := NewEngine(rules, merchants)
	result := engine.Match("NETFLIX.COM SUBSCRIPTION")

	require.NotNil(t, result)
	assert.Equal(t, "Netflix (Rule)", result.CleanName)
	assert.True(t, result.IsRule)
	assert.Equal(t, &categoryID1, result.CategoryID)
}

// Test batch matching
func TestEngine_MatchBatch(t *testing.T) {
	rules := []CategoryRule{
		{
			ID:           uuid.New(),
			UserID:       uuid.New(),
			MatchPattern: "%UBER%",
			CleanName:    strPtr("Uber"),
		},
		{
			ID:           uuid.New(),
			UserID:       uuid.New(),
			MatchPattern: "%AMAZON%",
			CleanName:    strPtr("Amazon"),
		},
	}

	engine := NewEngine(rules, nil)

	descriptions := []string{
		"UBER TRIP 123",
		"RANDOM SHOP",
		"AMAZON PURCHASE",
		"ANOTHER RANDOM",
		"UBER EATS ORDER",
	}

	results := engine.MatchBatch(descriptions)

	assert.Len(t, results, 5)
	assert.NotNil(t, results[0]) // UBER
	assert.Equal(t, "Uber", results[0].CleanName)
	assert.Nil(t, results[1]) // RANDOM SHOP
	assert.NotNil(t, results[2]) // AMAZON
	assert.Equal(t, "Amazon", results[2].CleanName)
	assert.Nil(t, results[3]) // ANOTHER RANDOM
	assert.NotNil(t, results[4]) // UBER EATS
	assert.Equal(t, "Uber", results[4].CleanName)
}

// Test empty engine
func TestEngine_Empty(t *testing.T) {
	engine := NewEngine(nil, nil)

	assert.True(t, engine.IsEmpty())
	assert.Equal(t, 0, engine.PatternCount())
	assert.Nil(t, engine.Match("ANY TEXT"))
}

// Test rebuild functionality
func TestEngine_Rebuild(t *testing.T) {
	engine := NewEngine(nil, nil)
	assert.True(t, engine.IsEmpty())

	// Rebuild with patterns
	rules := []CategoryRule{
		{
			ID:           uuid.New(),
			MatchPattern: "%TEST%",
			CleanName:    strPtr("Test"),
		},
	}
	engine.Build(rules, nil)

	assert.False(t, engine.IsEmpty())
	assert.Equal(t, 1, engine.PatternCount())
	result := engine.Match("THIS IS A TEST")
	require.NotNil(t, result)
	assert.Equal(t, "Test", result.CleanName)
}

// Benchmark: Compare Aho-Corasick vs naive string matching
func BenchmarkCategorization(b *testing.B) {
	// Simulate a large rule-set (1,000 different merchants)
	merchants := make([]Merchant, 1000)
	for i := 0; i < 1000; i++ {
		merchants[i] = Merchant{
			ID:         uuid.New(),
			RawPattern: fmt.Sprintf("MERCHANT_%d", i),
			CleanName:  fmt.Sprintf("Merchant %d", i),
			IsSystem:   true,
		}
	}
	// Add a real one to find at position 500
	merchants[500] = Merchant{
		ID:         uuid.New(),
		RawPattern: "REVOLUT",
		CleanName:  "Revolut",
		IsSystem:   true,
	}

	engine := NewEngine(nil, merchants)

	// A typical messy bank string
	input := "CARD PURCHASE 27/12/2025 CAR WAL CRT DEB REVOLUT LONDON GB"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = engine.Match(input)
	}
}

// Benchmark: Naive approach for comparison
func BenchmarkNaiveCategorization(b *testing.B) {
	// Same 1,000 patterns
	patterns := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		patterns[i] = fmt.Sprintf("MERCHANT_%d", i)
	}
	patterns[500] = "REVOLUT"

	input := "CARD PURCHASE 27/12/2025 CAR WAL CRT DEB REVOLUT LONDON GB"
	upperInput := strings.ToUpper(input)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, pattern := range patterns {
			if strings.Contains(upperInput, pattern) {
				break
			}
		}
	}
}

// Benchmark: Batch processing with many transactions
func BenchmarkBatchCategorization(b *testing.B) {
	// 1,000 patterns
	merchants := make([]Merchant, 1000)
	for i := 0; i < 1000; i++ {
		merchants[i] = Merchant{
			ID:         uuid.New(),
			RawPattern: fmt.Sprintf("MERCHANT_%d", i),
			CleanName:  fmt.Sprintf("Merchant %d", i),
		}
	}
	// Add some real patterns
	merchants[100] = Merchant{ID: uuid.New(), RawPattern: "REVOLUT", CleanName: "Revolut"}
	merchants[200] = Merchant{ID: uuid.New(), RawPattern: "AMAZON", CleanName: "Amazon"}
	merchants[300] = Merchant{ID: uuid.New(), RawPattern: "NETFLIX", CleanName: "Netflix"}
	merchants[400] = Merchant{ID: uuid.New(), RawPattern: "SPOTIFY", CleanName: "Spotify"}

	engine := NewEngine(nil, merchants)

	// Simulate a batch import of 100 transactions
	descriptions := make([]string, 100)
	for i := 0; i < 100; i++ {
		switch i % 5 {
		case 0:
			descriptions[i] = "CARD PURCHASE REVOLUT LONDON GB"
		case 1:
			descriptions[i] = "AMAZON.COM ORDER #1234"
		case 2:
			descriptions[i] = "NETFLIX.COM SUBSCRIPTION"
		case 3:
			descriptions[i] = "SPOTIFY PREMIUM"
		default:
			descriptions[i] = fmt.Sprintf("RANDOM PURCHASE %d", i)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = engine.MatchBatch(descriptions)
	}
}

// Benchmark: Scaling with pattern count
func BenchmarkScaling(b *testing.B) {
	patternCounts := []int{100, 500, 1000, 5000, 10000}

	for _, count := range patternCounts {
		b.Run(fmt.Sprintf("patterns_%d", count), func(b *testing.B) {
			merchants := make([]Merchant, count)
			for i := 0; i < count; i++ {
				merchants[i] = Merchant{
					ID:         uuid.New(),
					RawPattern: fmt.Sprintf("MERCHANT_%d", i),
					CleanName:  fmt.Sprintf("Merchant %d", i),
				}
			}
			// Pattern to match is at the end
			merchants[count-1] = Merchant{
				ID:         uuid.New(),
				RawPattern: "REVOLUT",
				CleanName:  "Revolut",
			}

			engine := NewEngine(nil, merchants)
			input := "CARD PURCHASE 27/12/2025 CAR WAL CRT DEB REVOLUT LONDON GB"

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = engine.Match(input)
			}
		})
	}
}

// Helper function
func strPtr(s string) *string {
	return &s
}
