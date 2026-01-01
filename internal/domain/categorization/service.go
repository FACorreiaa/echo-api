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
}

// NewService creates a new categorization service
func NewService(repo *Repository) *Service {
	return &Service{
		repo:          repo,
		ruleCache:     make(map[uuid.UUID][]CategoryRule),
		merchantCache: nil,
	}
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

	// Invalidate cache
	s.cacheMu.Lock()
	delete(s.ruleCache, userID)
	s.cacheMu.Unlock()

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
