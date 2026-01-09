// Package service provides business logic for budget periods
package service

import (
	"context"

	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/plan/repository"
	"github.com/google/uuid"
)

// BudgetPeriodService handles budget period business logic
type BudgetPeriodService struct {
	repo repository.BudgetPeriodRepository
}

// NewBudgetPeriodService creates a new budget period service
func NewBudgetPeriodService(repo repository.BudgetPeriodRepository) *BudgetPeriodService {
	return &BudgetPeriodService{repo: repo}
}

// GetOrCreatePeriod gets or creates a budget period for a specific month
func (s *BudgetPeriodService) GetOrCreatePeriod(ctx context.Context, planID uuid.UUID, year, month int) (*repository.BudgetPeriodWithItems, bool, error) {
	return s.repo.GetOrCreatePeriod(ctx, planID, year, month)
}

// ListPeriods lists all periods for a plan
func (s *BudgetPeriodService) ListPeriods(ctx context.Context, planID uuid.UUID) ([]*repository.BudgetPeriod, error) {
	return s.repo.ListPeriods(ctx, planID)
}

// GetPeriodByID gets a period by ID
func (s *BudgetPeriodService) GetPeriodByID(ctx context.Context, periodID uuid.UUID) (*repository.BudgetPeriodWithItems, error) {
	return s.repo.GetPeriodByID(ctx, periodID)
}

// UpdatePeriodItem updates an item's values for a period
func (s *BudgetPeriodService) UpdatePeriodItem(ctx context.Context, periodItemID uuid.UUID, budgeted, actual *int64, notes *string) (*repository.BudgetPeriodItem, error) {
	return s.repo.UpdatePeriodItem(ctx, periodItemID, budgeted, actual, notes)
}

// CopyPeriodItems copies values from one period to a new target period
func (s *BudgetPeriodService) CopyPeriodItems(ctx context.Context, sourcePeriodID uuid.UUID, targetPlanID uuid.UUID, targetYear, targetMonth int) (*repository.BudgetPeriodWithItems, error) {
	return s.repo.CopyPeriodItems(ctx, sourcePeriodID, targetPlanID, targetYear, targetMonth)
}
