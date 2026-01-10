package categorization

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/keyword"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/simple"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/google/uuid"
)

// SearchDocument represents a searchable merchant/rule document
type SearchDocument struct {
	ID          string  `json:"id"`
	Pattern     string  `json:"pattern"`     // Original pattern (for exact matching)
	CleanName   string  `json:"clean_name"`  // Clean display name
	Description string  `json:"description"` // Full text description for search
	CategoryID  string  `json:"category_id"` // Category UUID as string
	Type        string  `json:"type"`        // "rule" or "merchant"
	Priority    float64 `json:"priority"`    // For boosting results
	UserID      string  `json:"user_id"`     // Owner user ID (empty for system)
}

// SearchResult represents a search hit with relevance score
type SearchResult struct {
	Document   SearchDocument
	Score      float64 // Relevance score from Bleve
	CategoryID *uuid.UUID
	IsRule     bool
}

// SearchIndex provides full-text search capabilities using Bleve.
// It supports complex queries, fuzzy matching, and relevance scoring.
type SearchIndex struct {
	index   bleve.Index
	indexMu sync.RWMutex
	path    string // Path to index storage (empty for in-memory)
}

// NewSearchIndex creates a new search index.
// If path is empty, creates an in-memory index.
// If path is provided, creates/opens a persistent index.
func NewSearchIndex(path string) (*SearchIndex, error) {
	si := &SearchIndex{path: path}

	var index bleve.Index
	var err error

	indexMapping := buildIndexMapping()

	if path == "" {
		// In-memory index
		index, err = bleve.NewMemOnly(indexMapping)
	} else {
		// Check if index exists
		if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
			// Create new index
			if mkdirErr := os.MkdirAll(filepath.Dir(path), 0o755); mkdirErr != nil {
				return nil, fmt.Errorf("failed to create index directory: %w", mkdirErr)
			}
			index, err = bleve.New(path, indexMapping)
		} else {
			// Open existing index
			index, err = bleve.Open(path)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create/open index: %w", err)
	}

	si.index = index
	return si, nil
}

// buildIndexMapping creates the Bleve index mapping for merchant documents
func buildIndexMapping() mapping.IndexMapping {
	// Create a text field mapping for full-text search
	textFieldMapping := bleve.NewTextFieldMapping()
	textFieldMapping.Analyzer = simple.Name

	// Create a keyword field mapping for exact matches
	keywordFieldMapping := bleve.NewTextFieldMapping()
	keywordFieldMapping.Analyzer = keyword.Name

	// Create a numeric field mapping for priority boosting
	numericFieldMapping := bleve.NewNumericFieldMapping()

	// Create the document mapping
	docMapping := bleve.NewDocumentMapping()
	docMapping.AddFieldMappingsAt("pattern", keywordFieldMapping)
	docMapping.AddFieldMappingsAt("clean_name", textFieldMapping)
	docMapping.AddFieldMappingsAt("description", textFieldMapping)
	docMapping.AddFieldMappingsAt("category_id", keywordFieldMapping)
	docMapping.AddFieldMappingsAt("type", keywordFieldMapping)
	docMapping.AddFieldMappingsAt("priority", numericFieldMapping)
	docMapping.AddFieldMappingsAt("user_id", keywordFieldMapping)

	// Create the index mapping
	indexMapping := bleve.NewIndexMapping()
	indexMapping.DefaultMapping = docMapping
	indexMapping.DefaultAnalyzer = simple.Name

	return indexMapping
}

// IndexRulesAndMerchants indexes all rules and merchants for search
func (si *SearchIndex) IndexRulesAndMerchants(rules []CategoryRule, merchants []Merchant) error {
	si.indexMu.Lock()
	defer si.indexMu.Unlock()

	batch := si.index.NewBatch()

	// Index rules
	for _, rule := range rules {
		cleanName := ""
		if rule.CleanName != nil {
			cleanName = *rule.CleanName
		}

		categoryID := ""
		if rule.AssignedCategoryID != nil {
			categoryID = rule.AssignedCategoryID.String()
		}

		doc := SearchDocument{
			ID:          fmt.Sprintf("rule_%s", rule.ID.String()),
			Pattern:     rule.MatchPattern,
			CleanName:   cleanName,
			Description: fmt.Sprintf("%s %s", rule.MatchPattern, cleanName),
			CategoryID:  categoryID,
			Type:        "rule",
			Priority:    float64(rule.Priority + 1000),
			UserID:      rule.UserID.String(),
		}

		if err := batch.Index(doc.ID, doc); err != nil {
			return fmt.Errorf("failed to index rule %s: %w", rule.ID, err)
		}
	}

	// Index merchants
	for _, merchant := range merchants {
		categoryID := ""
		if merchant.DefaultCategoryID != nil {
			categoryID = merchant.DefaultCategoryID.String()
		}

		userID := ""
		if merchant.UserID != nil {
			userID = merchant.UserID.String()
		}

		priority := 0.0
		if merchant.UserID != nil {
			priority = 100.0
		}

		doc := SearchDocument{
			ID:          fmt.Sprintf("merchant_%s", merchant.ID.String()),
			Pattern:     merchant.RawPattern,
			CleanName:   merchant.CleanName,
			Description: fmt.Sprintf("%s %s", merchant.RawPattern, merchant.CleanName),
			CategoryID:  categoryID,
			Type:        "merchant",
			Priority:    priority,
			UserID:      userID,
		}

		if err := batch.Index(doc.ID, doc); err != nil {
			return fmt.Errorf("failed to index merchant %s: %w", merchant.ID, err)
		}
	}

	if err := si.index.Batch(batch); err != nil {
		return fmt.Errorf("failed to execute batch index: %w", err)
	}

	return nil
}

// Search performs a full-text search and returns matching documents
func (si *SearchIndex) Search(query string, limit int) ([]SearchResult, error) {
	si.indexMu.RLock()
	defer si.indexMu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	// Create a match query (handles tokenization and fuzzy matching)
	matchQuery := bleve.NewMatchQuery(query)
	matchQuery.SetFuzziness(1) // Allow 1 edit distance for typo tolerance

	searchRequest := bleve.NewSearchRequest(matchQuery)
	searchRequest.Size = limit
	searchRequest.Fields = []string{"*"} // Return all fields

	searchResults, err := si.index.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return si.convertResults(searchResults)
}

// SearchWithPrefix performs a prefix search (autocomplete style)
func (si *SearchIndex) SearchWithPrefix(prefix string, limit int) ([]SearchResult, error) {
	si.indexMu.RLock()
	defer si.indexMu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	// Create a prefix query
	prefixQuery := bleve.NewPrefixQuery(prefix)

	searchRequest := bleve.NewSearchRequest(prefixQuery)
	searchRequest.Size = limit
	searchRequest.Fields = []string{"*"}

	searchResults, err := si.index.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("prefix search failed: %w", err)
	}

	return si.convertResults(searchResults)
}

// SearchFuzzy performs a fuzzy search with configurable edit distance
func (si *SearchIndex) SearchFuzzy(query string, fuzziness int, limit int) ([]SearchResult, error) {
	si.indexMu.RLock()
	defer si.indexMu.RUnlock()

	if limit <= 0 {
		limit = 10
	}
	if fuzziness < 0 {
		fuzziness = 0
	}
	if fuzziness > 2 {
		fuzziness = 2 // Bleve max is 2
	}

	// Create a fuzzy query
	fuzzyQuery := bleve.NewFuzzyQuery(query)
	fuzzyQuery.SetFuzziness(fuzziness)

	searchRequest := bleve.NewSearchRequest(fuzzyQuery)
	searchRequest.Size = limit
	searchRequest.Fields = []string{"*"}

	searchResults, err := si.index.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("fuzzy search failed: %w", err)
	}

	return si.convertResults(searchResults)
}

// SearchAdvanced performs a complex query with boolean logic
// Example: "+starbucks -airport" (must have starbucks, must not have airport)
func (si *SearchIndex) SearchAdvanced(queryString string, limit int) ([]SearchResult, error) {
	si.indexMu.RLock()
	defer si.indexMu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	// Parse the query string
	query := bleve.NewQueryStringQuery(queryString)

	searchRequest := bleve.NewSearchRequest(query)
	searchRequest.Size = limit
	searchRequest.Fields = []string{"*"}

	searchResults, err := si.index.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("advanced search failed: %w", err)
	}

	return si.convertResults(searchResults)
}

// SearchByCategory finds all merchants/rules in a specific category
func (si *SearchIndex) SearchByCategory(categoryID uuid.UUID, limit int) ([]SearchResult, error) {
	si.indexMu.RLock()
	defer si.indexMu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	// Create a term query for exact category match
	termQuery := bleve.NewTermQuery(categoryID.String())
	termQuery.SetField("category_id")

	searchRequest := bleve.NewSearchRequest(termQuery)
	searchRequest.Size = limit
	searchRequest.Fields = []string{"*"}

	searchResults, err := si.index.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("category search failed: %w", err)
	}

	return si.convertResults(searchResults)
}

// convertResults converts Bleve search results to our SearchResult type
func (si *SearchIndex) convertResults(searchResults *bleve.SearchResult) ([]SearchResult, error) {
	results := make([]SearchResult, 0, len(searchResults.Hits))

	for _, hit := range searchResults.Hits {
		doc := SearchDocument{
			ID: hit.ID,
		}

		// Extract fields from the hit
		if pattern, ok := hit.Fields["pattern"].(string); ok {
			doc.Pattern = pattern
		}
		if cleanName, ok := hit.Fields["clean_name"].(string); ok {
			doc.CleanName = cleanName
		}
		if description, ok := hit.Fields["description"].(string); ok {
			doc.Description = description
		}
		if categoryID, ok := hit.Fields["category_id"].(string); ok {
			doc.CategoryID = categoryID
		}
		if docType, ok := hit.Fields["type"].(string); ok {
			doc.Type = docType
		}
		if priority, ok := hit.Fields["priority"].(float64); ok {
			doc.Priority = priority
		}
		if userID, ok := hit.Fields["user_id"].(string); ok {
			doc.UserID = userID
		}

		result := SearchResult{
			Document: doc,
			Score:    hit.Score,
			IsRule:   doc.Type == "rule",
		}

		// Parse category ID
		if doc.CategoryID != "" {
			if catID, err := uuid.Parse(doc.CategoryID); err == nil {
				result.CategoryID = &catID
			}
		}

		results = append(results, result)
	}

	return results, nil
}

// Clear removes all documents from the index
func (si *SearchIndex) Clear() error {
	si.indexMu.Lock()
	defer si.indexMu.Unlock()

	// Get all document IDs
	query := bleve.NewMatchAllQuery()
	searchRequest := bleve.NewSearchRequest(query)
	searchRequest.Size = 10000 // Reasonable batch size

	searchResults, err := si.index.Search(searchRequest)
	if err != nil {
		return fmt.Errorf("failed to list documents: %w", err)
	}

	batch := si.index.NewBatch()
	for _, hit := range searchResults.Hits {
		batch.Delete(hit.ID)
	}

	if err := si.index.Batch(batch); err != nil {
		return fmt.Errorf("failed to delete documents: %w", err)
	}

	return nil
}

// Close closes the index
func (si *SearchIndex) Close() error {
	si.indexMu.Lock()
	defer si.indexMu.Unlock()

	if si.index != nil {
		return si.index.Close()
	}
	return nil
}

// DocumentCount returns the number of documents in the index
func (si *SearchIndex) DocumentCount() (uint64, error) {
	si.indexMu.RLock()
	defer si.indexMu.RUnlock()

	return si.index.DocCount()
}

// IndexDocument adds or updates a single document
func (si *SearchIndex) IndexDocument(doc SearchDocument) error {
	si.indexMu.Lock()
	defer si.indexMu.Unlock()

	return si.index.Index(doc.ID, doc)
}

// DeleteDocument removes a document by ID
func (si *SearchIndex) DeleteDocument(id string) error {
	si.indexMu.Lock()
	defer si.indexMu.Unlock()

	return si.index.Delete(id)
}
