package categorization

import (
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/lithammer/fuzzysearch/fuzzy"
)

// FuzzyMatchResult represents a fuzzy match with its similarity score
type FuzzyMatchResult struct {
	Pattern    string     // The pattern that matched
	CleanName  string     // The clean merchant name
	CategoryID *uuid.UUID // The category ID to assign
	Score      int        // Similarity score (higher = better match, max ~100)
	Distance   int        // Levenshtein distance (lower = closer match)
	IsRule     bool       // True if from a rule, false if from merchant
	RuleID     *uuid.UUID
	MerchantID *uuid.UUID
}

// FuzzyMatcher provides fuzzy string matching using Levenshtein distance.
// It's ideal for catching merchant variations like "Starbucks 001" vs "Starbucks 002"
// and grouping them under a single entity.
type FuzzyMatcher struct {
	patterns []fuzzyPattern
	mu       sync.RWMutex
}

type fuzzyPattern struct {
	normalized string     // Uppercase, trimmed pattern for matching
	cleanName  string     // Display name
	categoryID *uuid.UUID
	ruleID     *uuid.UUID
	merchantID *uuid.UUID
	isRule     bool
	priority   int
}

// NewFuzzyMatcher creates a new fuzzy matcher from rules and merchants
func NewFuzzyMatcher(rules []CategoryRule, merchants []Merchant) *FuzzyMatcher {
	fm := &FuzzyMatcher{}
	fm.Build(rules, merchants)
	return fm
}

// Build constructs the fuzzy matcher from rules and merchants
func (fm *FuzzyMatcher) Build(rules []CategoryRule, merchants []Merchant) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	totalPatterns := len(rules) + len(merchants)
	fm.patterns = make([]fuzzyPattern, 0, totalPatterns)

	// Add rules (higher priority)
	for _, rule := range rules {
		cleanPattern := strings.ToUpper(strings.Trim(rule.MatchPattern, "%"))
		if cleanPattern == "" {
			continue
		}

		cleanName := ""
		if rule.CleanName != nil {
			cleanName = *rule.CleanName
		}

		ruleID := rule.ID
		fm.patterns = append(fm.patterns, fuzzyPattern{
			normalized: cleanPattern,
			cleanName:  cleanName,
			categoryID: rule.AssignedCategoryID,
			ruleID:     &ruleID,
			isRule:     true,
			priority:   rule.Priority + 1000,
		})
	}

	// Add merchants (lower priority)
	for _, merchant := range merchants {
		cleanPattern := strings.ToUpper(strings.Trim(merchant.RawPattern, "%"))
		if cleanPattern == "" {
			continue
		}

		priority := 0
		if merchant.UserID != nil {
			priority = 100
		}

		merchantID := merchant.ID
		fm.patterns = append(fm.patterns, fuzzyPattern{
			normalized: cleanPattern,
			cleanName:  merchant.CleanName,
			categoryID: merchant.DefaultCategoryID,
			merchantID: &merchantID,
			isRule:     false,
			priority:   priority,
		})
	}
}

// Match finds the best fuzzy match for the given description.
// Returns nil if no match meets the minimum threshold.
// The threshold is a similarity score (0-100), where 100 is a perfect match.
func (fm *FuzzyMatcher) Match(description string, threshold int) *FuzzyMatchResult {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	if len(fm.patterns) == 0 {
		return nil
	}

	normalized := strings.ToUpper(description)

	var bestMatch *FuzzyMatchResult
	bestScore := threshold - 1 // Only consider matches >= threshold

	for _, p := range fm.patterns {
		// Calculate fuzzy match score
		score := fuzzyScore(normalized, p.normalized)

		if score > bestScore || (score == bestScore && bestMatch != nil && p.priority > bestMatch.Score) {
			bestScore = score
			bestMatch = &FuzzyMatchResult{
				Pattern:    p.normalized,
				CleanName:  p.cleanName,
				CategoryID: p.categoryID,
				Score:      score,
				Distance:   levenshteinDistance(normalized, p.normalized),
				IsRule:     p.isRule,
				RuleID:     p.ruleID,
				MerchantID: p.merchantID,
			}
		}
	}

	return bestMatch
}

// MatchAll finds all fuzzy matches above the threshold, sorted by score (highest first)
func (fm *FuzzyMatcher) MatchAll(description string, threshold int) []FuzzyMatchResult {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	if len(fm.patterns) == 0 {
		return nil
	}

	normalized := strings.ToUpper(description)
	var results []FuzzyMatchResult

	for _, p := range fm.patterns {
		score := fuzzyScore(normalized, p.normalized)
		if score >= threshold {
			results = append(results, FuzzyMatchResult{
				Pattern:    p.normalized,
				CleanName:  p.cleanName,
				CategoryID: p.categoryID,
				Score:      score,
				Distance:   levenshteinDistance(normalized, p.normalized),
				IsRule:     p.isRule,
				RuleID:     p.ruleID,
				MerchantID: p.merchantID,
			})
		}
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// FindSimilarMerchants groups similar merchant strings together.
// This is useful for consolidating variations like "STARBUCKS 001", "STARBUCKS 002", etc.
// Returns groups of similar descriptions that should map to the same merchant.
func (fm *FuzzyMatcher) FindSimilarMerchants(descriptions []string, threshold int) map[string][]string {
	groups := make(map[string][]string)
	assigned := make(map[int]bool)

	for i, desc := range descriptions {
		if assigned[i] {
			continue
		}

		// This description becomes the canonical form for its group
		canonical := desc
		group := []string{desc}
		assigned[i] = true

		// Find all similar descriptions
		for j := i + 1; j < len(descriptions); j++ {
			if assigned[j] {
				continue
			}

			score := fuzzyScore(strings.ToUpper(desc), strings.ToUpper(descriptions[j]))
			if score >= threshold {
				group = append(group, descriptions[j])
				assigned[j] = true
			}
		}

		groups[canonical] = group
	}

	return groups
}

// RankMatches returns the source patterns ranked by similarity to the input
func (fm *FuzzyMatcher) RankMatches(description string, limit int) []FuzzyMatchResult {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	if len(fm.patterns) == 0 {
		return nil
	}

	normalized := strings.ToUpper(description)
	results := make([]FuzzyMatchResult, 0, len(fm.patterns))

	for _, p := range fm.patterns {
		score := fuzzyScore(normalized, p.normalized)
		results = append(results, FuzzyMatchResult{
			Pattern:    p.normalized,
			CleanName:  p.cleanName,
			CategoryID: p.categoryID,
			Score:      score,
			Distance:   levenshteinDistance(normalized, p.normalized),
			IsRule:     p.isRule,
			RuleID:     p.ruleID,
			MerchantID: p.merchantID,
		})
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if limit > 0 && limit < len(results) {
		results = results[:limit]
	}

	return results
}

// PatternCount returns the number of patterns in the matcher
func (fm *FuzzyMatcher) PatternCount() int {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	return len(fm.patterns)
}

// fuzzyScore calculates a similarity score between two strings (0-100)
// Uses a combination of containment checks, Levenshtein distance, and fuzzy ranking
func fuzzyScore(s1, s2 string) int {
	// Exact match
	if s1 == s2 {
		return 100
	}

	// Check if one contains the other (common for merchant variations)
	if strings.Contains(s1, s2) {
		// s2 is contained in s1 - great match
		// Score based on length ratio
		return 75 + (25 * len(s2) / len(s1))
	}
	if strings.Contains(s2, s1) {
		// s1 is contained in s2
		return 75 + (25 * len(s1) / len(s2))
	}

	// Calculate Levenshtein distance-based score
	distance := levenshteinDistance(s1, s2)
	maxLen := len(s1)
	if len(s2) > maxLen {
		maxLen = len(s2)
	}

	if maxLen == 0 {
		return 0
	}

	// Convert distance to a percentage score
	// Score = 100 * (1 - distance/maxLen)
	levenshteinScore := 100 * (maxLen - distance) / maxLen

	// Use fuzzy library's rank function for subsequence matching
	rank := fuzzy.RankMatch(s2, s1)
	fuzzyLibScore := 0
	if rank >= 0 {
		// Rank is the position in s1 where s2 starts matching
		// Lower rank = better (matches earlier in string)
		if rank < len(s1) {
			fuzzyLibScore = 60 - (rank * 40 / len(s1))
		}
	}

	// Return the best score from either method
	if levenshteinScore > fuzzyLibScore {
		return levenshteinScore
	}
	return fuzzyLibScore
}

// levenshteinDistance calculates the edit distance between two strings
func levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	// Create matrix
	r1 := []rune(s1)
	r2 := []rune(s2)
	lenR1 := len(r1)
	lenR2 := len(r2)

	// Use two rows instead of full matrix for memory efficiency
	prev := make([]int, lenR2+1)
	curr := make([]int, lenR2+1)

	// Initialize first row
	for j := 0; j <= lenR2; j++ {
		prev[j] = j
	}

	// Fill the matrix
	for i := 1; i <= lenR1; i++ {
		curr[0] = i
		for j := 1; j <= lenR2; j++ {
			cost := 1
			if r1[i-1] == r2[j-1] {
				cost = 0
			}
			curr[j] = min(
				prev[j]+1,      // deletion
				curr[j-1]+1,    // insertion
				prev[j-1]+cost, // substitution
			)
		}
		prev, curr = curr, prev
	}

	return prev[lenR2]
}

func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
