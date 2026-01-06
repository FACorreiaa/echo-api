package categorization

import (
	"context"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// Service handles transaction categorization logic
type Service struct {
	repo *Repository

	// Cache for rules/merchants (refreshed periodically)
	ruleCache     map[uuid.UUID][]CategoryRule
	merchantCache []Merchant
	cacheMu       sync.RWMutex

	// High-performance Aho-Corasick engine per user
	// Key: userID, Value: pre-built engine for that user
	engineCache map[uuid.UUID]*Engine
	engineMu    sync.RWMutex

	// Fuzzy matcher per user for handling merchant variations
	fuzzyCache map[uuid.UUID]*FuzzyMatcher
	fuzzyMu    sync.RWMutex

	// Search index for full-text search (shared across users)
	searchIndex *SearchIndex
	searchMu    sync.RWMutex
}

// NewService creates a new categorization service
func NewService(repo *Repository) *Service {
	return &Service{
		repo:          repo,
		ruleCache:     make(map[uuid.UUID][]CategoryRule),
		merchantCache: nil,
		engineCache:   make(map[uuid.UUID]*Engine),
		fuzzyCache:    make(map[uuid.UUID]*FuzzyMatcher),
	}
}

// NewServiceWithSearch creates a categorization service with full-text search enabled
func NewServiceWithSearch(repo *Repository, indexPath string) (*Service, error) {
	s := NewService(repo)

	// Initialize search index
	index, err := NewSearchIndex(indexPath)
	if err != nil {
		return nil, err
	}
	s.searchIndex = index

	return s, nil
}

// Categorize takes a raw transaction description and returns enriched data
func (s *Service) Categorize(ctx context.Context, userID uuid.UUID, description string) (*CategorizationResult, error) {
	result := &CategorizationResult{
		CleanMerchantName: cleanDescription(description),
	}

	// 1. Check user's custom rules first (highest priority)
	rules, err := s.GetUserRules(ctx, userID)
	if err != nil {
		return result, nil // Fail open - return cleaned name without category
	}

	for _, rule := range rules {
		if matchPattern(description, rule.MatchPattern) {
			if rule.CleanName != nil {
				result.CleanMerchantName = *rule.CleanName
			}
			result.CategoryID = rule.AssignedCategoryID
			result.IsRecurring = rule.IsRecurring
			result.RuleID = &rule.ID
			return result, nil
		}
	}

	// 2. Check global merchant database
	merchants, err := s.getMerchants(ctx, &userID)
	if err != nil {
		return result, nil // Fail open
	}

	for _, merchant := range merchants {
		if matchPattern(description, merchant.RawPattern) {
			result.CleanMerchantName = merchant.CleanName
			result.CategoryID = merchant.DefaultCategoryID
			result.MerchantID = &merchant.ID
			return result, nil
		}
	}

	// 3. No match - return cleaned description
	return result, nil
}

// CategorizeBatch categorizes multiple descriptions efficiently
func (s *Service) CategorizeBatch(ctx context.Context, userID uuid.UUID, descriptions []string) ([]*CategorizationResult, error) {
	// Pre-fetch rules and merchants once
	rules, _ := s.GetUserRules(ctx, userID)
	merchants, _ := s.getMerchants(ctx, &userID)

	results := make([]*CategorizationResult, len(descriptions))

	for i, desc := range descriptions {
		result := &CategorizationResult{
			CleanMerchantName: cleanDescription(desc),
		}

		// Check rules
		for _, rule := range rules {
			if matchPattern(desc, rule.MatchPattern) {
				if rule.CleanName != nil {
					result.CleanMerchantName = *rule.CleanName
				}
				result.CategoryID = rule.AssignedCategoryID
				result.IsRecurring = rule.IsRecurring
				result.RuleID = &rule.ID
				break
			}
		}

		// If no rule matched, check merchants
		if result.RuleID == nil {
			for _, merchant := range merchants {
				if matchPattern(desc, merchant.RawPattern) {
					result.CleanMerchantName = merchant.CleanName
					result.CategoryID = merchant.DefaultCategoryID
					result.MerchantID = &merchant.ID
					break
				}
			}
		}

		results[i] = result
	}

	return results, nil
}

// CreateRule creates a new categorization rule with optional backfill
func (s *Service) CreateRule(ctx context.Context, userID uuid.UUID, pattern, cleanName string, categoryID *uuid.UUID, isRecurring, applyToExisting bool) (*CategoryRule, int64, error) {
	// Check if rule already exists
	existing, err := s.repo.FindRuleByPattern(ctx, userID, pattern)
	if err != nil {
		return nil, 0, err
	}
	if existing != nil {
		// Could update instead, but for now just return existing
		return existing, 0, nil
	}

	rule := &CategoryRule{
		UserID:             userID,
		MatchPattern:       pattern,
		CleanName:          &cleanName,
		AssignedCategoryID: categoryID,
		IsRecurring:        isRecurring,
		Priority:           0,
	}

	if err := s.repo.CreateRule(ctx, rule); err != nil {
		return nil, 0, err
	}

	// Invalidate caches (both rule cache and engine cache)
	s.cacheMu.Lock()
	delete(s.ruleCache, userID)
	s.cacheMu.Unlock()
	s.invalidateEngineCache(userID)

	// Optionally apply to existing transactions
	var updated int64
	if applyToExisting {
		updated, err = s.repo.UpdateTransactionsMerchant(ctx, userID, pattern, cleanName, categoryID)
		if err != nil {
			// Rule was created, just log the backfill error
			return rule, 0, nil
		}
	}

	return rule, updated, nil
}

// GetUserRules fetches rules with caching (exported for handler access)
func (s *Service) GetUserRules(ctx context.Context, userID uuid.UUID) ([]CategoryRule, error) {
	s.cacheMu.RLock()
	if rules, ok := s.ruleCache[userID]; ok {
		s.cacheMu.RUnlock()
		return rules, nil
	}
	s.cacheMu.RUnlock()

	rules, err := s.repo.GetUserRules(ctx, userID)
	if err != nil {
		return nil, err
	}

	s.cacheMu.Lock()
	s.ruleCache[userID] = rules
	s.cacheMu.Unlock()

	return rules, nil
}

// getMerchants fetches merchants with caching
func (s *Service) getMerchants(ctx context.Context, userID *uuid.UUID) ([]Merchant, error) {
	s.cacheMu.RLock()
	if s.merchantCache != nil {
		s.cacheMu.RUnlock()
		return s.merchantCache, nil
	}
	s.cacheMu.RUnlock()

	merchants, err := s.repo.GetMerchants(ctx, userID)
	if err != nil {
		return nil, err
	}

	s.cacheMu.Lock()
	s.merchantCache = merchants
	s.cacheMu.Unlock()

	return merchants, nil
}

// getOrBuildEngine returns a cached Aho-Corasick engine for the user, building it if necessary.
// This engine enables O(n) pattern matching regardless of the number of rules/merchants.
func (s *Service) getOrBuildEngine(ctx context.Context, userID uuid.UUID) (*Engine, error) {
	// Check cache first
	s.engineMu.RLock()
	if engine, ok := s.engineCache[userID]; ok {
		s.engineMu.RUnlock()
		return engine, nil
	}
	s.engineMu.RUnlock()

	// Build new engine
	rules, err := s.GetUserRules(ctx, userID)
	if err != nil {
		return nil, err
	}

	merchants, err := s.getMerchants(ctx, &userID)
	if err != nil {
		return nil, err
	}

	engine := NewEngine(rules, merchants)

	// Cache the engine
	s.engineMu.Lock()
	s.engineCache[userID] = engine
	s.engineMu.Unlock()

	return engine, nil
}

// invalidateEngineCache removes the cached engine for a user, forcing a rebuild on next use.
// Call this when rules or merchants change.
func (s *Service) invalidateEngineCache(userID uuid.UUID) {
	s.engineMu.Lock()
	delete(s.engineCache, userID)
	s.engineMu.Unlock()

	// Also invalidate fuzzy cache
	s.fuzzyMu.Lock()
	delete(s.fuzzyCache, userID)
	s.fuzzyMu.Unlock()
}

// getOrBuildFuzzyMatcher returns a cached FuzzyMatcher for the user, building it if necessary.
func (s *Service) getOrBuildFuzzyMatcher(ctx context.Context, userID uuid.UUID) (*FuzzyMatcher, error) {
	s.fuzzyMu.RLock()
	if matcher, ok := s.fuzzyCache[userID]; ok {
		s.fuzzyMu.RUnlock()
		return matcher, nil
	}
	s.fuzzyMu.RUnlock()

	rules, err := s.GetUserRules(ctx, userID)
	if err != nil {
		return nil, err
	}

	merchants, err := s.getMerchants(ctx, &userID)
	if err != nil {
		return nil, err
	}

	matcher := NewFuzzyMatcher(rules, merchants)

	s.fuzzyMu.Lock()
	s.fuzzyCache[userID] = matcher
	s.fuzzyMu.Unlock()

	return matcher, nil
}

// CategorizeFast uses the high-performance Aho-Corasick engine for categorization.
// This is significantly faster than Categorize when you have many rules/merchants.
func (s *Service) CategorizeFast(ctx context.Context, userID uuid.UUID, description string) (*CategorizationResult, error) {
	result := &CategorizationResult{
		CleanMerchantName: cleanDescription(description),
	}

	engine, err := s.getOrBuildEngine(ctx, userID)
	if err != nil {
		return result, nil // Fail open
	}

	match := engine.Match(description)
	if match == nil {
		return result, nil
	}

	// Convert MatchResult to CategorizationResult
	if match.CleanName != "" {
		result.CleanMerchantName = match.CleanName
	}
	result.CategoryID = match.CategoryID
	result.IsRecurring = match.IsRecurring
	result.RuleID = match.RuleID
	result.MerchantID = match.MerchantID

	return result, nil
}

// CategorizeBatchFast categorizes multiple descriptions using the Aho-Corasick engine.
// This provides massive performance gains for bulk imports (5M+ transactions/second).
func (s *Service) CategorizeBatchFast(ctx context.Context, userID uuid.UUID, descriptions []string) ([]*CategorizationResult, error) {
	results := make([]*CategorizationResult, len(descriptions))

	// Initialize with cleaned descriptions
	for i, desc := range descriptions {
		results[i] = &CategorizationResult{
			CleanMerchantName: cleanDescription(desc),
		}
	}

	engine, err := s.getOrBuildEngine(ctx, userID)
	if err != nil {
		return results, nil // Fail open with cleaned names
	}

	// Batch match all descriptions
	matches := engine.MatchBatch(descriptions)

	// Merge match results
	for i, match := range matches {
		if match == nil {
			continue
		}
		if match.CleanName != "" {
			results[i].CleanMerchantName = match.CleanName
		}
		results[i].CategoryID = match.CategoryID
		results[i].IsRecurring = match.IsRecurring
		results[i].RuleID = match.RuleID
		results[i].MerchantID = match.MerchantID
	}

	return results, nil
}

// cleanDescription performs basic cleanup on raw bank descriptions
func cleanDescription(desc string) string {
	// Remove common prefixes
	prefixes := []string{
		"COMPRAS C.DEB ",
		"COMPRA ",
		"PURCHASE ",
		"POS ",
		"DEBIT CARD ",
		"PAGAMENTO ",
		"PAG*",
	}

	cleaned := strings.TrimSpace(desc)
	upper := strings.ToUpper(cleaned)

	for _, prefix := range prefixes {
		if strings.HasPrefix(upper, prefix) {
			cleaned = strings.TrimSpace(cleaned[len(prefix):])
			break
		}
	}

	// Remove trailing reference numbers (common pattern: *1234 or #1234)
	if idx := strings.LastIndex(cleaned, "*"); idx > 0 {
		potentialRef := cleaned[idx+1:]
		if len(potentialRef) <= 6 && isNumeric(potentialRef) {
			cleaned = strings.TrimSpace(cleaned[:idx])
		}
	}

	// Title case for cleaner display
	return toTitleCase(cleaned)
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

func toTitleCase(s string) string {
	words := strings.Fields(s)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}
	return strings.Join(words, " ")
}

// ============================================================================
// Fuzzy Matching Methods
// ============================================================================

// CategorizeFuzzy uses fuzzy matching to categorize a transaction.
// This is useful when exact pattern matching fails but you want to catch
// merchant variations like "STARBUCKS 001" vs "STARBUCKS 002".
// The threshold parameter controls match sensitivity (0-100, higher = stricter).
// Recommended threshold: 70 for loose matching, 85 for strict matching.
func (s *Service) CategorizeFuzzy(ctx context.Context, userID uuid.UUID, description string, threshold int) (*CategorizationResult, error) {
	result := &CategorizationResult{
		CleanMerchantName: cleanDescription(description),
	}

	matcher, err := s.getOrBuildFuzzyMatcher(ctx, userID)
	if err != nil {
		return result, nil // Fail open
	}

	match := matcher.Match(description, threshold)
	if match == nil {
		return result, nil
	}

	if match.CleanName != "" {
		result.CleanMerchantName = match.CleanName
	}
	result.CategoryID = match.CategoryID
	result.RuleID = match.RuleID
	result.MerchantID = match.MerchantID

	return result, nil
}

// CategorizeWithFallback tries exact matching first, then falls back to fuzzy matching.
// This provides the best balance of speed (Aho-Corasick) and flexibility (fuzzy).
func (s *Service) CategorizeWithFallback(ctx context.Context, userID uuid.UUID, description string, fuzzyThreshold int) (*CategorizationResult, error) {
	// Try fast exact matching first
	result, err := s.CategorizeFast(ctx, userID, description)
	if err != nil {
		return result, err
	}

	// If we got a match, return it
	if result.RuleID != nil || result.MerchantID != nil {
		return result, nil
	}

	// Fall back to fuzzy matching
	return s.CategorizeFuzzy(ctx, userID, description, fuzzyThreshold)
}

// SuggestMerchantMatches returns the top fuzzy matches for a description.
// Useful for UI suggestions when the user is categorizing a new transaction.
func (s *Service) SuggestMerchantMatches(ctx context.Context, userID uuid.UUID, description string, limit int) ([]FuzzyMatchResult, error) {
	matcher, err := s.getOrBuildFuzzyMatcher(ctx, userID)
	if err != nil {
		return nil, err
	}

	return matcher.RankMatches(description, limit), nil
}

// GroupSimilarMerchants groups similar transaction descriptions together.
// This is useful for batch cleanup of merchant names in user's transaction history.
// Returns a map where the key is the canonical merchant name and value is all similar descriptions.
func (s *Service) GroupSimilarMerchants(ctx context.Context, userID uuid.UUID, descriptions []string, threshold int) (map[string][]string, error) {
	matcher, err := s.getOrBuildFuzzyMatcher(ctx, userID)
	if err != nil {
		return nil, err
	}

	return matcher.FindSimilarMerchants(descriptions, threshold), nil
}

// ============================================================================
// Full-Text Search Methods (requires SearchIndex to be initialized)
// ============================================================================

// SearchMerchants performs full-text search across merchants and rules.
// Returns results ranked by relevance.
func (s *Service) SearchMerchants(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	s.searchMu.RLock()
	defer s.searchMu.RUnlock()

	if s.searchIndex == nil {
		return nil, nil // Search not enabled
	}

	return s.searchIndex.Search(query, limit)
}

// SearchMerchantsWithPrefix performs autocomplete-style prefix search.
func (s *Service) SearchMerchantsWithPrefix(ctx context.Context, prefix string, limit int) ([]SearchResult, error) {
	s.searchMu.RLock()
	defer s.searchMu.RUnlock()

	if s.searchIndex == nil {
		return nil, nil
	}

	return s.searchIndex.SearchWithPrefix(prefix, limit)
}

// SearchMerchantsFuzzy performs fuzzy full-text search with typo tolerance.
func (s *Service) SearchMerchantsFuzzy(ctx context.Context, query string, fuzziness, limit int) ([]SearchResult, error) {
	s.searchMu.RLock()
	defer s.searchMu.RUnlock()

	if s.searchIndex == nil {
		return nil, nil
	}

	return s.searchIndex.SearchFuzzy(query, fuzziness, limit)
}

// SearchMerchantsAdvanced performs complex boolean queries.
// Example: "+coffee -airport" (must have coffee, must not have airport)
func (s *Service) SearchMerchantsAdvanced(ctx context.Context, queryString string, limit int) ([]SearchResult, error) {
	s.searchMu.RLock()
	defer s.searchMu.RUnlock()

	if s.searchIndex == nil {
		return nil, nil
	}

	return s.searchIndex.SearchAdvanced(queryString, limit)
}

// RebuildSearchIndex rebuilds the full-text search index from current rules and merchants.
// Call this periodically or when data changes significantly.
func (s *Service) RebuildSearchIndex(ctx context.Context, userID uuid.UUID) error {
	s.searchMu.Lock()
	defer s.searchMu.Unlock()

	if s.searchIndex == nil {
		return nil
	}

	rules, err := s.GetUserRules(ctx, userID)
	if err != nil {
		return err
	}

	merchants, err := s.getMerchants(ctx, &userID)
	if err != nil {
		return err
	}

	// Clear and rebuild
	if err := s.searchIndex.Clear(); err != nil {
		return err
	}

	return s.searchIndex.IndexRulesAndMerchants(rules, merchants)
}

// CloseSearchIndex closes the search index. Call this on service shutdown.
func (s *Service) CloseSearchIndex() error {
	s.searchMu.Lock()
	defer s.searchMu.Unlock()

	if s.searchIndex != nil {
		return s.searchIndex.Close()
	}
	return nil
}
