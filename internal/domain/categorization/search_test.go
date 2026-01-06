package categorization

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchIndex_InMemory(t *testing.T) {
	// Create in-memory index
	index, err := NewSearchIndex("")
	require.NoError(t, err)
	defer index.Close()

	categoryID := uuid.New()
	userID := uuid.New()

	rules := []CategoryRule{
		{
			ID:                 uuid.New(),
			UserID:             userID,
			MatchPattern:       "%STARBUCKS%",
			CleanName:          strPtr("Starbucks"),
			AssignedCategoryID: &categoryID,
			Priority:           10,
		},
	}

	merchants := []Merchant{
		{
			ID:                uuid.New(),
			RawPattern:        "%AMAZON%",
			CleanName:         "Amazon",
			DefaultCategoryID: &categoryID,
			IsSystem:          true,
		},
		{
			ID:                uuid.New(),
			RawPattern:        "%NETFLIX%",
			CleanName:         "Netflix",
			DefaultCategoryID: &categoryID,
			IsSystem:          true,
		},
	}

	// Index documents
	err = index.IndexRulesAndMerchants(rules, merchants)
	require.NoError(t, err)

	// Verify document count
	count, err := index.DocumentCount()
	require.NoError(t, err)
	assert.Equal(t, uint64(3), count)

	t.Run("basic search", func(t *testing.T) {
		results, err := index.Search("starbucks", 10)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, "Starbucks", results[0].Document.CleanName)
		assert.True(t, results[0].IsRule)
	})

	t.Run("fuzzy search with typo", func(t *testing.T) {
		results, err := index.SearchFuzzy("amazn", 1, 10) // Missing 'o'
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(results), 1)
		assert.Equal(t, "Amazon", results[0].Document.CleanName)
	})

	t.Run("prefix search", func(t *testing.T) {
		results, err := index.SearchWithPrefix("net", 10)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(results), 1)
		assert.Equal(t, "Netflix", results[0].Document.CleanName)
	})

	t.Run("search by category", func(t *testing.T) {
		results, err := index.SearchByCategory(categoryID, 10)
		require.NoError(t, err)
		assert.Len(t, results, 3) // All items have same category
	})

	t.Run("advanced boolean search", func(t *testing.T) {
		// Add more documents for boolean testing
		index.IndexDocument(SearchDocument{
			ID:          "test_coffee_shop",
			Pattern:     "COFFEE SHOP",
			CleanName:   "Coffee Shop",
			Description: "Coffee Shop Local",
			Type:        "merchant",
		})
		index.IndexDocument(SearchDocument{
			ID:          "test_airport_coffee",
			Pattern:     "AIRPORT COFFEE",
			CleanName:   "Airport Coffee",
			Description: "Airport Coffee Stand",
			Type:        "merchant",
		})

		// Search for coffee but not airport
		results, err := index.SearchAdvanced("coffee -airport", 10)
		require.NoError(t, err)

		// Should find Coffee Shop but not Airport Coffee
		for _, r := range results {
			assert.NotContains(t, r.Document.CleanName, "Airport")
		}
	})
}

func TestSearchIndex_IndexAndDelete(t *testing.T) {
	index, err := NewSearchIndex("")
	require.NoError(t, err)
	defer index.Close()

	doc := SearchDocument{
		ID:        "test_doc",
		Pattern:   "TEST",
		CleanName: "Test Document",
		Type:      "merchant",
	}

	// Index document
	err = index.IndexDocument(doc)
	require.NoError(t, err)

	// Verify it's indexed
	count, _ := index.DocumentCount()
	assert.Equal(t, uint64(1), count)

	// Delete document
	err = index.DeleteDocument("test_doc")
	require.NoError(t, err)

	// Verify it's deleted
	count, _ = index.DocumentCount()
	assert.Equal(t, uint64(0), count)
}

func TestSearchIndex_Clear(t *testing.T) {
	index, err := NewSearchIndex("")
	require.NoError(t, err)
	defer index.Close()

	// Index some documents
	merchants := []Merchant{
		{ID: uuid.New(), RawPattern: "A", CleanName: "A"},
		{ID: uuid.New(), RawPattern: "B", CleanName: "B"},
		{ID: uuid.New(), RawPattern: "C", CleanName: "C"},
	}
	err = index.IndexRulesAndMerchants(nil, merchants)
	require.NoError(t, err)

	count, _ := index.DocumentCount()
	assert.Equal(t, uint64(3), count)

	// Clear index
	err = index.Clear()
	require.NoError(t, err)

	count, _ = index.DocumentCount()
	assert.Equal(t, uint64(0), count)
}

// Benchmark search operations
func BenchmarkSearch(b *testing.B) {
	index, _ := NewSearchIndex("")
	defer index.Close()

	// Index 1000 merchants
	merchants := make([]Merchant, 1000)
	for i := 0; i < 1000; i++ {
		merchants[i] = Merchant{
			ID:         uuid.New(),
			RawPattern: fmt.Sprintf("MERCHANT_%d", i),
			CleanName:  fmt.Sprintf("Merchant %d", i),
		}
	}
	// Add specific merchants
	merchants[500] = Merchant{ID: uuid.New(), RawPattern: "STARBUCKS", CleanName: "Starbucks"}
	merchants[600] = Merchant{ID: uuid.New(), RawPattern: "AMAZON", CleanName: "Amazon"}

	index.IndexRulesAndMerchants(nil, merchants)

	b.ResetTimer()

	b.Run("BasicSearch", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = index.Search("starbucks", 10)
		}
	})

	b.Run("FuzzySearch", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = index.SearchFuzzy("starbuks", 1, 10)
		}
	})

	b.Run("PrefixSearch", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = index.SearchWithPrefix("star", 10)
		}
	})

	b.Run("AdvancedSearch", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = index.SearchAdvanced("+merchant -500", 10)
		}
	})
}

func BenchmarkIndexing(b *testing.B) {
	b.Run("Index1000Merchants", func(b *testing.B) {
		merchants := make([]Merchant, 1000)
		for i := 0; i < 1000; i++ {
			merchants[i] = Merchant{
				ID:         uuid.New(),
				RawPattern: fmt.Sprintf("MERCHANT_%d", i),
				CleanName:  fmt.Sprintf("Merchant %d", i),
			}
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			index, _ := NewSearchIndex("")
			_ = index.IndexRulesAndMerchants(nil, merchants)
			index.Close()
		}
	})
}

// Compare all three approaches
func BenchmarkCompare_All_Approaches(b *testing.B) {
	merchants := make([]Merchant, 1000)
	for i := 0; i < 1000; i++ {
		merchants[i] = Merchant{
			ID:         uuid.New(),
			RawPattern: fmt.Sprintf("MERCHANT_%d", i),
			CleanName:  fmt.Sprintf("Merchant %d", i),
		}
	}
	merchants[500] = Merchant{ID: uuid.New(), RawPattern: "STARBUCKS", CleanName: "Starbucks"}

	// Setup engines
	engine := NewEngine(nil, merchants)
	fuzzyMatcher := NewFuzzyMatcher(nil, merchants)
	searchIndex, _ := NewSearchIndex("")
	searchIndex.IndexRulesAndMerchants(nil, merchants)
	defer searchIndex.Close()

	input := "STARBUCKS COFFEE SHOP"

	b.Run("AhoCorasick_Exact", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = engine.Match(input)
		}
	})

	b.Run("FuzzyMatcher_70", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = fuzzyMatcher.Match(input, 70)
		}
	})

	b.Run("Bleve_Search", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = searchIndex.Search("starbucks", 1)
		}
	})

	b.Run("Bleve_FuzzySearch", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = searchIndex.SearchFuzzy("starbucks", 1, 1)
		}
	})
}
