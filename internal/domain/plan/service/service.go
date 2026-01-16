// Package service provides business logic for user plans.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	importrepo "github.com/FACorreiaa/smart-finance-tracker/internal/domain/import/repository"
	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/plan/repository"
	"github.com/google/uuid"
)

// PlanService handles plan business logic
type PlanService struct {
	repo       repository.PlanRepository
	importRepo importrepo.ImportRepository
	logger     *slog.Logger
}

// NewPlanService creates a new plan service
func NewPlanService(repo repository.PlanRepository, importRepo importrepo.ImportRepository, logger *slog.Logger) *PlanService {
	return &PlanService{
		repo:       repo,
		importRepo: importRepo,
		logger:     logger,
	}
}

// CreatePlan creates a new financial plan
func (s *PlanService) CreatePlan(ctx context.Context, userID uuid.UUID, input *CreatePlanInput) (*PlanWithDetails, error) {
	// Debug: log the user ID to help diagnose foreign key violations
	s.logger.Info("creating plan", slog.String("user_id", userID.String()), slog.String("plan_name", input.Name))

	config, _ := json.Marshal(map[string]any{
		"chart_type":       "horizontal_bar",
		"show_percentages": true,
	})

	plan := &repository.UserPlan{
		UserID:       userID,
		Name:         input.Name,
		Description:  input.Description,
		Status:       repository.PlanStatusDraft,
		SourceType:   repository.PlanSourceManual,
		CurrencyCode: input.CurrencyCode,
		Config:       config,
	}
	if plan.CurrencyCode == "" {
		plan.CurrencyCode = "EUR"
	}

	// Build nested structure
	var groups []*repository.PlanCategoryGroup
	var categories []*repository.PlanCategory
	var items []*repository.PlanItem

	sortOrder := 0
	for _, groupInput := range input.CategoryGroups {
		group := &repository.PlanCategoryGroup{
			ID:            uuid.New(),
			Name:          groupInput.Name,
			Color:         groupInput.Color,
			TargetPercent: groupInput.TargetPercent,
			SortOrder:     sortOrder,
			Labels:        marshalLabels(groupInput.Labels),
		}
		groups = append(groups, group)
		sortOrder++

		catSortOrder := 0
		for _, catInput := range groupInput.Categories {
			category := &repository.PlanCategory{
				ID:        uuid.New(),
				GroupID:   &group.ID,
				Name:      catInput.Name,
				Icon:      catInput.Icon,
				SortOrder: catSortOrder,
				Labels:    marshalLabels(catInput.Labels),
			}
			categories = append(categories, category)
			catSortOrder++

			itemSortOrder := 0
			for _, itemInput := range catInput.Items {
				item := &repository.PlanItem{
					ID:            uuid.New(),
					CategoryID:    &category.ID,
					Name:          itemInput.Name,
					BudgetedMinor: itemInput.BudgetedMinor,
					WidgetType:    itemInput.WidgetType,
					FieldType:     itemInput.FieldType,
					SortOrder:     itemSortOrder,
					Labels:        marshalLabels(itemInput.Labels),
				}
				if item.WidgetType == "" {
					item.WidgetType = repository.WidgetTypeInput
				}
				if item.FieldType == "" {
					item.FieldType = repository.FieldTypeCurrency
				}
				if itemInput.ConfigID != nil {
					id, err := uuid.Parse(*itemInput.ConfigID)
					if err == nil {
						item.ConfigID = &id
					}
				}
				item.ItemType = itemInput.ItemType
				if itemInput.InitialActualMinor != nil {
					item.ActualMinor = *itemInput.InitialActualMinor
				}

				items = append(items, item)
				itemSortOrder++
			}
		}
	}

	if err := s.repo.CreatePlanWithStructure(ctx, plan, groups, categories, items); err != nil {
		return nil, err
	}

	return s.GetPlanWithDetails(ctx, userID, plan.ID)
}

// GetPlan retrieves a plan by ID with ownership check
func (s *PlanService) GetPlan(ctx context.Context, userID, planID uuid.UUID) (*repository.UserPlan, error) {
	plan, err := s.repo.GetPlanByID(ctx, planID)
	if err != nil {
		return nil, err
	}
	if plan == nil || plan.UserID != userID {
		return nil, nil
	}
	return plan, nil
}

// ListPlans lists all plans for a user
func (s *PlanService) ListPlans(ctx context.Context, userID uuid.UUID, status *repository.PlanStatus, limit, offset int) ([]*repository.UserPlan, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.repo.ListPlansByUser(ctx, userID, status, limit, offset)
}

// UpdatePlan updates a plan
func (s *PlanService) UpdatePlan(ctx context.Context, userID, planID uuid.UUID, name, description *string) (*repository.UserPlan, error) {
	plan, err := s.GetPlan(ctx, userID, planID)
	if err != nil || plan == nil {
		return nil, err
	}

	if name != nil {
		plan.Name = *name
	}
	if description != nil {
		plan.Description = description
	}

	if err := s.repo.UpdatePlan(ctx, plan); err != nil {
		return nil, err
	}

	return s.repo.GetPlanByID(ctx, planID)
}

// UpdatePlanItem updates a specific item's budgeted amount
func (s *PlanService) UpdatePlanItem(ctx context.Context, userID, planID, itemID uuid.UUID, budgetedMinor int64) error {
	// Verify ownership
	plan, err := s.GetPlan(ctx, userID, planID)
	if err != nil || plan == nil {
		return err
	}

	return s.repo.UpdateItemBudget(ctx, itemID, budgetedMinor)
}

// DeletePlan soft-deletes a plan
func (s *PlanService) DeletePlan(ctx context.Context, userID, planID uuid.UUID) error {
	plan, err := s.GetPlan(ctx, userID, planID)
	if err != nil || plan == nil {
		return err
	}
	return s.repo.DeletePlan(ctx, planID)
}

// SetActivePlan marks a plan as active
func (s *PlanService) SetActivePlan(ctx context.Context, userID, planID uuid.UUID) (*repository.UserPlan, error) {
	if err := s.repo.SetActivePlan(ctx, userID, planID); err != nil {
		return nil, err
	}
	return s.repo.GetPlanByID(ctx, planID)
}

// DuplicatePlan creates a copy of a plan
func (s *PlanService) DuplicatePlan(ctx context.Context, userID, planID uuid.UUID, newName string) (*repository.UserPlan, error) {
	// Verify ownership
	plan, err := s.GetPlan(ctx, userID, planID)
	if err != nil || plan == nil {
		return nil, err
	}

	return s.repo.DuplicatePlan(ctx, planID, newName, userID)
}

// GetPlanWithDetails retrieves a plan with all its structure (groups, categories, items)
func (s *PlanService) GetPlanWithDetails(ctx context.Context, userID, planID uuid.UUID) (*PlanWithDetails, error) {
	plan, err := s.GetPlan(ctx, userID, planID)
	if err != nil || plan == nil {
		return nil, err
	}

	groups, err := s.repo.GetCategoryGroupsByPlan(ctx, planID)
	if err != nil {
		return nil, err
	}

	categories, err := s.repo.GetCategoriesByPlan(ctx, planID)
	if err != nil {
		return nil, err
	}

	items, err := s.repo.GetItemsByPlan(ctx, planID)
	if err != nil {
		return nil, err
	}

	return &PlanWithDetails{
		Plan:       plan,
		Groups:     groups,
		Categories: categories,
		Items:      items,
	}, nil
}

// UpdatePlanStructure updates the entire structure of a plan
func (s *PlanService) UpdatePlanStructure(ctx context.Context, userID, planID uuid.UUID, allowedGroups []CreateCategoryGroupInput) (*repository.UserPlan, error) {
	// 1. Verify Plan Ownership
	plan, err := s.GetPlan(ctx, userID, planID)
	if err != nil || plan == nil {
		return nil, err
	}

	// 2. Fetch existing items to preserve actuals (if ID provided)
	existingItems, err := s.repo.GetItemsByPlan(ctx, planID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch existing items: %w", err)
	}
	existingItemsMap := make(map[uuid.UUID]*repository.PlanItem)
	for _, i := range existingItems {
		existingItemsMap[i.ID] = i
	}

	// 3. Flatten structure
	var groups []*repository.PlanCategoryGroup
	var categories []*repository.PlanCategory
	var items []*repository.PlanItem

	sortOrder := 0
	for _, groupInput := range allowedGroups {
		groupID := uuid.New()
		if groupInput.ID != nil {
			groupID = *groupInput.ID
		}

		group := &repository.PlanCategoryGroup{
			ID:            groupID,
			PlanID:        planID,
			Name:          groupInput.Name,
			Color:         groupInput.Color,
			TargetPercent: groupInput.TargetPercent,
			SortOrder:     sortOrder,
			Labels:        marshalLabels(groupInput.Labels),
		}
		groups = append(groups, group)
		sortOrder++

		catSortOrder := 0
		for _, catInput := range groupInput.Categories {
			catID := uuid.New()
			if catInput.ID != nil {
				catID = *catInput.ID
			}

			category := &repository.PlanCategory{
				ID:        catID,
				PlanID:    planID,
				GroupID:   &group.ID,
				Name:      catInput.Name,
				Icon:      catInput.Icon,
				SortOrder: catSortOrder,
				Labels:    marshalLabels(catInput.Labels),
			}
			categories = append(categories, category)
			catSortOrder++

			itemSortOrder := 0
			for _, itemInput := range catInput.Items {
				itemID := uuid.New()
				if itemInput.ID != nil {
					itemID = *itemInput.ID
				}

				// Resolve ActualMinor preservation logic
				actualMinor := int64(0)
				if itemInput.InitialActualMinor != nil {
					// Explicit override/init
					actualMinor = *itemInput.InitialActualMinor
				} else if existing, ok := existingItemsMap[itemID]; ok {
					// Preserve existing
					actualMinor = existing.ActualMinor
				}

				// Resolve ConfigID
				var configID *uuid.UUID
				if itemInput.ConfigID != nil {
					if id, err := uuid.Parse(*itemInput.ConfigID); err == nil {
						configID = &id
					}
				}

				item := &repository.PlanItem{
					ID:            itemID,
					PlanID:        planID,
					CategoryID:    &category.ID,
					Name:          itemInput.Name,
					BudgetedMinor: itemInput.BudgetedMinor,
					ActualMinor:   actualMinor,
					WidgetType:    itemInput.WidgetType,
					FieldType:     itemInput.FieldType,
					SortOrder:     itemSortOrder,
					Labels:        marshalLabels(itemInput.Labels),
					ItemType:      itemInput.ItemType,
					ConfigID:      configID,
				}
				if item.WidgetType == "" {
					item.WidgetType = repository.WidgetTypeInput
				}
				if item.FieldType == "" {
					item.FieldType = repository.FieldTypeCurrency
				}

				items = append(items, item)
				itemSortOrder++
			}
		}
	}

	// 4. Update via Repo
	if err := s.repo.UpdatePlanStructure(ctx, planID, groups, categories, items); err != nil {
		return nil, fmt.Errorf("failed to update plan structure: %w", err)
	}

	return s.repo.GetPlanByID(ctx, planID)
}

// ComputePlanActualsInput contains the input for computing plan actuals
type ComputePlanActualsInput struct {
	StartDate time.Time
	EndDate   time.Time
	Persist   bool // If true, update the plan items in the database
}

// ComputePlanActualsResult contains the result of computing plan actuals
type ComputePlanActualsResult struct {
	Plan                *PlanWithDetails
	ItemsUpdated        int
	TransactionsMatched int
	UnmatchedItems      []UnmatchedItem
}

// UnmatchedItem represents a plan item that couldn't be matched to transactions
type UnmatchedItem struct {
	ItemID   uuid.UUID
	ItemName string
	Reason   string // "no_category_mapping" or "no_transactions"
}

// ComputePlanActuals syncs actual spending from transactions to plan items
// This method queries transactions for the given period and aggregates spending
// by category, then maps them to plan items via category name matching.
func (s *PlanService) ComputePlanActuals(ctx context.Context, userID, planID uuid.UUID, input *ComputePlanActualsInput) (*ComputePlanActualsResult, error) {
	// Get plan with all its details
	planDetails, err := s.GetPlanWithDetails(ctx, userID, planID)
	if err != nil {
		return nil, err
	}

	// Query transaction totals by category for the period
	categoryTotals, err := s.importRepo.GetCategoryTotals(ctx, userID, input.StartDate, input.EndDate)
	if err != nil {
		s.logger.Error("failed to get category totals", slog.Any("error", err))
		return nil, err
	}

	// Build a map from category name (lowercased) to total spending
	categoryMap := make(map[string]int64)
	totalTransactions := 0
	for _, ct := range categoryTotals {
		categoryMap[strings.ToLower(ct.CategoryName)] = ct.TotalMinor
		totalTransactions += ct.Count
	}

	// Match plan items to category totals
	result := &ComputePlanActualsResult{
		Plan:                planDetails,
		ItemsUpdated:        0,
		TransactionsMatched: totalTransactions,
		UnmatchedItems:      make([]UnmatchedItem, 0),
	}

	for _, item := range planDetails.Items {
		itemNameLower := strings.ToLower(item.Name)

		// Try to find a matching category
		if total, found := categoryMap[itemNameLower]; found {
			// Update the item's actual amount
			if input.Persist {
				err := s.repo.UpdatePlanItemActual(ctx, item.ID, total)
				if err != nil {
					s.logger.Warn("failed to update plan item actual",
						slog.String("item_id", item.ID.String()),
						slog.Any("error", err),
					)
				} else {
					result.ItemsUpdated++
				}
			} else {
				result.ItemsUpdated++
			}
		} else {
			// No matching category found
			result.UnmatchedItems = append(result.UnmatchedItems, UnmatchedItem{
				ItemID:   item.ID,
				ItemName: item.Name,
				Reason:   "no_category_mapping",
			})
		}
	}

	s.logger.Info("computed plan actuals",
		slog.String("plan_id", planID.String()),
		slog.Int("items_updated", result.ItemsUpdated),
		slog.Int("transactions_matched", result.TransactionsMatched),
		slog.Int("unmatched_items", len(result.UnmatchedItems)),
		slog.Int("categories_found", len(categoryTotals)),
	)

	return result, nil
}

// ProcessTransaction handles real-time dual-impact updates.
// It finds the active plan and updates the relevant budget item's actual spend.
// Matching is done by category name (case-insensitive) since transaction categories
// and plan categories are in different ID spaces.
func (s *PlanService) ProcessTransaction(ctx context.Context, userID uuid.UUID, txAmountMinor int64, txCategoryID *uuid.UUID, txCategoryName string) error {
	// 1. Get Active Plan
	activePlan, err := s.repo.GetActivePlan(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get active plan: %w", err)
	}
	if activePlan == nil {
		// No active plan, nothing to update. This is valid.
		return nil
	}

	// 2. If transaction has no category name, we can't match it to a budget item
	if txCategoryName == "" {
		return nil
	}

	// 3. Get all items for the active plan
	items, err := s.repo.GetItemsByPlan(ctx, activePlan.ID)
	if err != nil {
		return fmt.Errorf("failed to fetch items for plan %s: %w", activePlan.ID, err)
	}

	// 4. Find matching item by name (case-insensitive)
	txCategoryLower := strings.ToLower(txCategoryName)
	var matchedItem *repository.PlanItem
	for _, item := range items {
		// Match by item name OR category name
		if strings.ToLower(item.Name) == txCategoryLower {
			// Only match budget/recurring items (not goals/income)
			if item.ItemType == repository.ItemTypeBudget || item.ItemType == repository.ItemTypeRecurring {
				matchedItem = item
				break
			}
		}
	}

	if matchedItem != nil {
		// 5. Update Actual (use absolute value since expenses are negative)
		amountToAdd := txAmountMinor
		if amountToAdd < 0 {
			amountToAdd = -amountToAdd // Make positive for budget tracking
		}

		if err := s.repo.IncrementPlanItemActual(ctx, matchedItem.ID, amountToAdd); err != nil {
			s.logger.Error("failed to increment plan item actual",
				slog.String("item_id", matchedItem.ID.String()),
				slog.Int64("amount", amountToAdd),
				slog.Any("error", err),
			)
			return err
		}
		s.logger.Info("dual-impact update success",
			slog.String("plan_id", activePlan.ID.String()),
			slog.String("item_id", matchedItem.ID.String()),
			slog.String("item_name", matchedItem.Name),
			slog.String("category", txCategoryName),
			slog.Int64("amount_added", amountToAdd),
		)
	} else {
		s.logger.Debug("no matching budget item for transaction category",
			slog.String("category", txCategoryName),
			slog.String("plan_id", activePlan.ID.String()),
		)
	}

	return nil
}

// ============================================================================
// Input/Output Types
// ============================================================================

// CreatePlanInput contains the data for creating a plan
type CreatePlanInput struct {
	Name           string
	Description    *string
	CurrencyCode   string
	CategoryGroups []CreateCategoryGroupInput
}

// CreateCategoryGroupInput contains the data for creating a category group
type CreateCategoryGroupInput struct {
	ID            *uuid.UUID
	Name          string
	Color         *string
	TargetPercent float64
	Labels        map[string]string
	Categories    []CreateCategoryInput
}

// CreateCategoryInput contains the data for creating a category
type CreateCategoryInput struct {
	ID     *uuid.UUID
	Name   string
	Icon   *string
	Labels map[string]string
	Items  []CreateItemInput
}

// CreateItemInput contains the data for creating an item
type CreateItemInput struct {
	ID                 *uuid.UUID
	Name               string
	BudgetedMinor      int64
	WidgetType         repository.WidgetType
	FieldType          repository.FieldType
	Labels             map[string]string
	ItemType           repository.ItemType
	ConfigID           *string
	InitialActualMinor *int64
}

// PlanWithDetails contains a plan with all its nested structure
type PlanWithDetails struct {
	Plan       *repository.UserPlan
	Groups     []*repository.PlanCategoryGroup
	Categories []*repository.PlanCategory
	Items      []*repository.PlanItem
}

// ExcelAnalysisResult contains the result of analyzing an Excel file
type ExcelAnalysisResult struct {
	Sheets         []SheetInfo `json:"sheets"`
	SuggestedSheet string      `json:"suggested_sheet"`
}

// SheetInfo contains information about a single Excel sheet
type SheetInfo struct {
	Name               string             `json:"name"`
	IsLivingPlan       bool               `json:"is_living_plan"`
	RowCount           int                `json:"row_count"`
	FormulaCount       int                `json:"formula_count"`
	DetectedCategories []string           `json:"detected_categories"`
	MonthColumns       []string           `json:"month_columns"`
	DetectedMapping    *ColumnMappingInfo `json:"detected_mapping,omitempty"` // Auto-detected column layout
	PreviewRows        [][]string         `json:"preview_rows,omitempty"`     // First 5 rows for UI preview
}

// ColumnMappingInfo contains auto-detected column positions for import
type ColumnMappingInfo struct {
	CategoryColumn   string  `json:"category_column"`
	ValueColumn      string  `json:"value_column"`
	HeaderRow        int     `json:"header_row"`
	PercentageColumn string  `json:"percentage_column,omitempty"`
	Confidence       float64 `json:"confidence"`
}

// ExcelImportConfig contains the configuration for importing a plan from Excel
type ExcelImportConfig struct {
	CategoryColumn string `json:"category_column"`
	ValueColumn    string `json:"value_column"`
	HeaderRow      int    `json:"header_row"`
}

// ExcelImportResult contains the result of importing a plan from Excel
type ExcelImportResult struct {
	Plan               *repository.UserPlan
	CategoriesImported int
	ItemsImported      int
}

// ============================================================================
// Item Config Methods
// ============================================================================

// ListItemConfigs retrieves all item configs for a user
func (s *PlanService) ListItemConfigs(ctx context.Context, userID uuid.UUID) ([]*repository.ItemConfig, error) {
	return s.repo.ListItemConfigs(ctx, userID)
}

// GetItemConfigByID retrieves a specific item config
func (s *PlanService) GetItemConfigByID(ctx context.Context, configID uuid.UUID) (*repository.ItemConfig, error) {
	return s.repo.GetItemConfigByID(ctx, configID)
}

// CreateItemConfig creates a new item config
func (s *PlanService) CreateItemConfig(ctx context.Context, config *repository.ItemConfig) error {
	return s.repo.CreateItemConfig(ctx, config)
}

// UpdateItemConfig updates an existing item config
func (s *PlanService) UpdateItemConfig(ctx context.Context, config *repository.ItemConfig) error {
	return s.repo.UpdateItemConfig(ctx, config)
}

// DeleteItemConfig deletes an item config
func (s *PlanService) DeleteItemConfig(ctx context.Context, configID uuid.UUID) error {
	return s.repo.DeleteItemConfig(ctx, configID)
}

// ItemsByTabResult represents the result of GetItemsByTab
type ItemsByTabResult struct {
	Items         []repository.PlanItemWithConfig
	TotalBudgeted int64
	TotalActual   int64
}

// GetItemsByTab returns items for a plan filtered by target tab
func (s *PlanService) GetItemsByTab(ctx context.Context, planID uuid.UUID, targetTab repository.TargetTab) (*ItemsByTabResult, error) {
	items, totalBudgeted, totalActual, err := s.repo.GetItemsByTabWithTotals(ctx, planID, targetTab)
	if err != nil {
		return nil, err
	}
	return &ItemsByTabResult{
		Items:         items,
		TotalBudgeted: totalBudgeted,
		TotalActual:   totalActual,
	}, nil
}

// marshalLabels converts a map to JSON bytes
func marshalLabels(labels map[string]string) []byte {
	if labels == nil {
		return []byte("{}")
	}
	b, _ := json.Marshal(labels)
	return b
}
