package categorization

import (
	"strings"
	"sync"

	"github.com/cloudflare/ahocorasick"
	"github.com/google/uuid"
)

// MatchResult represents a single pattern match with its associated metadata
type MatchResult struct {
	Pattern    string     // The original pattern that matched
	CleanName  string     // The clean merchant name to display
	CategoryID *uuid.UUID // The category ID to assign
	IsRecurring bool      // Whether this is a recurring transaction
	RuleID     *uuid.UUID // If matched by a rule
	MerchantID *uuid.UUID // If matched by a merchant
	Priority   int        // Higher priority matches take precedence
	IsRule     bool       // True if this came from a rule, false if from merchant
}

// Engine is a high-performance pattern matching engine using the Aho-Corasick algorithm.
// It can match thousands of patterns simultaneously in a single pass through the text.
// Time complexity: O(n + m) where n = text length, m = total matches
// This is independent of the number of patterns!
type Engine struct {
	matcher  *ahocorasick.Matcher
	patterns []string         // Unique patterns in same order as matcher
	metadata [][]MatchResult  // Metadata for each pattern (may have multiple entries for same pattern)
	mu       sync.RWMutex     // Protects rebuilding the matcher
}

// NewEngine creates a new categorization engine from rules and merchants.
// The engine pre-computes a state machine (trie) for efficient multi-pattern matching.
func NewEngine(rules []CategoryRule, merchants []Merchant) *Engine {
	e := &Engine{}
	e.Build(rules, merchants)
	return e
}

// Build constructs the Aho-Corasick matcher from rules and merchants.
// This can be called to rebuild the engine when rules/merchants change.
// Handles duplicate patterns by grouping all metadata for the same pattern together.
func (e *Engine) Build(rules []CategoryRule, merchants []Merchant) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Calculate total capacity
	totalPatterns := len(rules) + len(merchants)
	if totalPatterns == 0 {
		e.matcher = nil
		e.patterns = nil
		e.metadata = nil
		return
	}

	// Use a map to group metadata by normalized pattern
	// This handles the case where rules and merchants have the same pattern
	patternToIndex := make(map[string]int)
	patterns := make([]string, 0, totalPatterns)
	metadata := make([][]MatchResult, 0, totalPatterns)

	// Helper to add a pattern with its metadata
	addPattern := func(cleanPattern string, result MatchResult) {
		if idx, exists := patternToIndex[cleanPattern]; exists {
			// Pattern already exists, add metadata to existing group
			metadata[idx] = append(metadata[idx], result)
		} else {
			// New pattern
			patternToIndex[cleanPattern] = len(patterns)
			patterns = append(patterns, cleanPattern)
			metadata = append(metadata, []MatchResult{result})
		}
	}

	// Add rules first (they have higher priority)
	for _, rule := range rules {
		// Normalize pattern: remove SQL LIKE wildcards and uppercase for matching
		cleanPattern := strings.ToUpper(strings.Trim(rule.MatchPattern, "%"))
		if cleanPattern == "" {
			continue
		}

		cleanName := ""
		if rule.CleanName != nil {
			cleanName = *rule.CleanName
		}

		ruleID := rule.ID // Create a copy for the pointer
		addPattern(cleanPattern, MatchResult{
			Pattern:     rule.MatchPattern,
			CleanName:   cleanName,
			CategoryID:  rule.AssignedCategoryID,
			IsRecurring: rule.IsRecurring,
			RuleID:      &ruleID,
			Priority:    rule.Priority + 1000, // Rules always have higher base priority than merchants
			IsRule:      true,
		})
	}

	// Add merchants (lower priority than rules)
	for _, merchant := range merchants {
		cleanPattern := strings.ToUpper(strings.Trim(merchant.RawPattern, "%"))
		if cleanPattern == "" {
			continue
		}

		priority := 0
		if merchant.UserID != nil {
			priority = 100 // User merchants have priority over system merchants
		}

		merchantID := merchant.ID // Create a copy for the pointer
		addPattern(cleanPattern, MatchResult{
			Pattern:    merchant.RawPattern,
			CleanName:  merchant.CleanName,
			CategoryID: merchant.DefaultCategoryID,
			MerchantID: &merchantID,
			Priority:   priority,
			IsRule:     false,
		})
	}

	e.patterns = patterns
	e.metadata = metadata

	if len(patterns) > 0 {
		// Convert string patterns to [][]byte for the Aho-Corasick matcher
		bytePatterns := make([][]byte, len(patterns))
		for i, p := range patterns {
			bytePatterns[i] = []byte(p)
		}
		e.matcher = ahocorasick.NewMatcher(bytePatterns)
	} else {
		e.matcher = nil
	}
}

// Match finds all matching patterns in the description and returns the best match.
// The best match is determined by priority (rules > user merchants > system merchants).
// Returns nil if no patterns match.
func (e *Engine) Match(description string) *MatchResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.matcher == nil || len(e.patterns) == 0 {
		return nil
	}

	// Normalize input for matching
	normalizedInput := strings.ToUpper(description)

	// Single pass through the text to find ALL matches
	matches := e.matcher.Match([]byte(normalizedInput))
	if len(matches) == 0 {
		return nil
	}

	// Find the highest priority match across all pattern groups
	var bestMatch *MatchResult
	for _, idx := range matches {
		if idx >= 0 && idx < len(e.metadata) {
			// Each index may have multiple metadata entries (e.g., rule + merchant with same pattern)
			for i := range e.metadata[idx] {
				match := &e.metadata[idx][i]
				if bestMatch == nil || match.Priority > bestMatch.Priority {
					// Create a copy to avoid returning a pointer to the slice element
					matchCopy := *match
					bestMatch = &matchCopy
				}
			}
		}
	}

	return bestMatch
}

// MatchAll finds all matching patterns in the description.
// Returns matches sorted by priority (highest first).
func (e *Engine) MatchAll(description string) []MatchResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.matcher == nil || len(e.patterns) == 0 {
		return nil
	}

	normalizedInput := strings.ToUpper(description)
	matches := e.matcher.Match([]byte(normalizedInput))
	if len(matches) == 0 {
		return nil
	}

	results := make([]MatchResult, 0, len(matches)*2) // Estimate capacity
	for _, idx := range matches {
		if idx >= 0 && idx < len(e.metadata) {
			// Append all metadata for this pattern
			results = append(results, e.metadata[idx]...)
		}
	}

	// Sort by priority (highest first) - simple insertion sort for small slices
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].Priority > results[j-1].Priority; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}

	return results
}

// MatchBatch categorizes multiple descriptions efficiently.
// This is optimized for bulk processing - the matcher is locked once for all descriptions.
func (e *Engine) MatchBatch(descriptions []string) []*MatchResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	results := make([]*MatchResult, len(descriptions))

	if e.matcher == nil || len(e.patterns) == 0 {
		return results
	}

	for i, desc := range descriptions {
		normalizedInput := strings.ToUpper(desc)
		matches := e.matcher.Match([]byte(normalizedInput))

		if len(matches) == 0 {
			continue
		}

		// Find the highest priority match across all pattern groups
		var bestMatch *MatchResult
		for _, idx := range matches {
			if idx >= 0 && idx < len(e.metadata) {
				for j := range e.metadata[idx] {
					match := &e.metadata[idx][j]
					if bestMatch == nil || match.Priority > bestMatch.Priority {
						matchCopy := *match
						bestMatch = &matchCopy
					}
				}
			}
		}

		results[i] = bestMatch
	}

	return results
}

// PatternCount returns the number of patterns loaded in the engine.
func (e *Engine) PatternCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.patterns)
}

// IsEmpty returns true if the engine has no patterns loaded.
func (e *Engine) IsEmpty() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.matcher == nil || len(e.patterns) == 0
}
