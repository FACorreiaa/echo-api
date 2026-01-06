package api

import (
	"context"

	"github.com/google/uuid"

	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/categorization"
	importservice "github.com/FACorreiaa/smart-finance-tracker/internal/domain/import/service"
)

// categorizationAdapter adapts categorization.Service to import's CategorizationService interface
type categorizationAdapter struct {
	svc *categorization.Service
}

// newCategorizationAdapter creates a new adapter
func newCategorizationAdapter(svc *categorization.Service) importservice.CategorizationService {
	return &categorizationAdapter{svc: svc}
}

// CategorizeBatch implements importservice.CategorizationService
func (a *categorizationAdapter) CategorizeBatch(ctx context.Context, userID uuid.UUID, descriptions []string) ([]*importservice.CategorizationResult, error) {
	results, err := a.svc.CategorizeBatch(ctx, userID, descriptions)
	if err != nil {
		return nil, err
	}

	// Convert categorization results to import's CategorizationResult
	importResults := make([]*importservice.CategorizationResult, len(results))
	for i, r := range results {
		importResults[i] = &importservice.CategorizationResult{
			CleanMerchantName: r.CleanMerchantName,
			CategoryID:        r.CategoryID,
			IsRecurring:       r.IsRecurring,
		}
	}

	return importResults, nil
}

// CategorizeBatchFast implements importservice.CategorizationService using high-performance Aho-Corasick
func (a *categorizationAdapter) CategorizeBatchFast(ctx context.Context, userID uuid.UUID, descriptions []string) ([]*importservice.CategorizationResult, error) {
	results, err := a.svc.CategorizeBatchFast(ctx, userID, descriptions)
	if err != nil {
		return nil, err
	}

	// Convert categorization results to import's CategorizationResult
	importResults := make([]*importservice.CategorizationResult, len(results))
	for i, r := range results {
		importResults[i] = &importservice.CategorizationResult{
			CleanMerchantName: r.CleanMerchantName,
			CategoryID:        r.CategoryID,
			IsRecurring:       r.IsRecurring,
		}
	}

	return importResults, nil
}
