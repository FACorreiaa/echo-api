package service

import (
	"context"
	"fmt"
	"time"

	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/plan/repository"
	"github.com/google/uuid"
)

// ReplicatePlanInput defines the request to clone a plan
type ReplicatePlanInput struct {
	SourcePlanID uuid.UUID
	TargetPeriod time.Time // e.g., first day of next month
	NewName      string
}

// PatchPlanItemInput defines granular updates
type PatchPlanItemInput struct {
	PlanID       uuid.UUID
	ItemID       uuid.UUID
	Name         *string
	BudgetAmount *int64
	ItemType     *string // "budget", "recurring", "goal", "income"
}

// ReplicatePlan creates a deep clone of a plan with smart logic
func (s *PlanService) ReplicatePlan(ctx context.Context, userID uuid.UUID, input ReplicatePlanInput) (*PlanWithDetails, error) {
	// 1. Fetch Source Plan via Service Method (checks ownership validation logic inside if needed, but here we pass userID)
	sourcePlan, err := s.GetPlanWithDetails(ctx, userID, input.SourcePlanID)
	if err != nil {
		return nil, fmt.Errorf("failed to get source plan: %w", err)
	}
	if sourcePlan == nil {
		return nil, fmt.Errorf("source plan not found")
	}

	// 2. Prepare Target Plan
	description := fmt.Sprintf("Replicated from %s", sourcePlan.Plan.Name)
	newPlan := &repository.UserPlan{
		ID:           uuid.New(),
		UserID:       userID,
		Name:         input.NewName,
		Description:  &description,
		Status:       repository.PlanStatusDraft,
		SourceType:   repository.PlanSourceTemplate,
		CurrencyCode: sourcePlan.Plan.CurrencyCode,
		// Config:       sourcePlan.Plan.Config, // Copy Config if needed
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Store period in description or config if schema doesn't support it yet
	// For now, we omit Period field causing error

	// 3. Create Plan in DB
	if err := s.repo.CreatePlan(ctx, newPlan); err != nil {
		return nil, fmt.Errorf("failed to create target plan: %w", err)
	}

	// 4. Iterate and Smart Copy
	// Map to track new IDs if needed for referencing
	for _, group := range sourcePlan.Groups {
		newGroup := &repository.PlanCategoryGroup{
			ID:            uuid.New(),
			PlanID:        newPlan.ID,
			Name:          group.Name,
			Color:         group.Color,
			TargetPercent: group.TargetPercent,
			SortOrder:     group.SortOrder,
			Labels:        group.Labels,
		}
		if err := s.repo.CreateCategoryGroup(ctx, newGroup); err != nil {
			return nil, err
		}

		// Find categories for this group
		// Note: sourcePlan.Categories is a flat list, we need to filter or if the structure in PlanWithDetails matches
		// The PlanWithDetails struct has flat lists. We should iterate intelligently.
		// Actually, let's just use the filtered lists if available, or iterate all.
		// Optimization: Filter in memory
		for _, cat := range sourcePlan.Categories {
			if cat.GroupID == nil || *cat.GroupID != group.ID {
				continue
			}

			newCat := &repository.PlanCategory{
				ID:        uuid.New(),
				GroupID:   &newGroup.ID,
				PlanID:    newPlan.ID,
				Name:      cat.Name,
				Icon:      cat.Icon,
				SortOrder: cat.SortOrder,
				Labels:    cat.Labels,
			}
			if err := s.repo.CreateCategory(ctx, newCat); err != nil {
				return nil, err
			}

			for _, item := range sourcePlan.Items {
				if item.CategoryID == nil || *item.CategoryID != cat.ID {
					continue
				}

				// SMART LOGIC:
				// 1. One-time items -> Skip (Implementation needs a way to identify them, for now we assume all are copied unless flagged)
				newItem := &repository.PlanItem{
					ID:            uuid.New(),
					PlanID:        newPlan.ID,
					CategoryID:    &newCat.ID,
					Name:          item.Name,
					BudgetedMinor: item.BudgetedMinor, // Default to same budget
					WidgetType:    item.WidgetType,
					FieldType:     item.FieldType,
					ItemType:      item.ItemType,
					SortOrder:     item.SortOrder,
					Labels:        item.Labels,
					ConfigID:      item.ConfigID,
					// Do not copy Actuals
				}

				if err := s.repo.CreateItem(ctx, newItem); err != nil {
					return nil, err
				}
			}
		}
	}

	return s.GetPlanWithDetails(ctx, userID, newPlan.ID)
}

// PatchPlanItem allows granular updates to a plan item
func (s *PlanService) PatchPlanItem(ctx context.Context, userID uuid.UUID, input PatchPlanItemInput) error {
	// 1. Fetch Plan to verify ownership and get items
	// This is expensive but safe. Optimally we'd have GetItemByID with owner check.
	plan, err := s.GetPlanWithDetails(ctx, userID, input.PlanID)
	if err != nil {
		return err
	}
	if plan == nil {
		return fmt.Errorf("plan not found")
	}

	// 2. Find Item
	var targetItem *repository.PlanItem
	for _, item := range plan.Items {
		if item.ID == input.ItemID {
			targetItem = item
			break
		}
	}
	if targetItem == nil {
		return fmt.Errorf("item not found")
	}

	// 3. Apply Updates
	if input.Name != nil {
		targetItem.Name = *input.Name
	}
	if input.BudgetAmount != nil {
		targetItem.BudgetedMinor = *input.BudgetAmount
	}
	if input.ItemType != nil {
		targetItem.ItemType = repository.ItemType(*input.ItemType)
	}
	targetItem.UpdatedAt = time.Now()

	// 4. Save
	return s.repo.UpdateItem(ctx, targetItem)
}
