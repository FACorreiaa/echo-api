// Package repository provides data access for plan-related entities.
package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// PlanSourceType represents the source of a plan
type PlanSourceType string

const (
	PlanSourceManual   PlanSourceType = "manual"
	PlanSourceExcel    PlanSourceType = "excel"
	PlanSourceTemplate PlanSourceType = "template"
)

// PlanStatus represents the status of a plan
type PlanStatus string

const (
	PlanStatusDraft    PlanStatus = "draft"
	PlanStatusActive   PlanStatus = "active"
	PlanStatusArchived PlanStatus = "archived"
)

// WidgetType represents the UI widget type for a plan item
type WidgetType string

const (
	WidgetTypeInput    WidgetType = "input"
	WidgetTypeSlider   WidgetType = "slider"
	WidgetTypeToggle   WidgetType = "toggle"
	WidgetTypeReadonly WidgetType = "readonly"
)

// FieldType represents the data type of a plan item
type FieldType string

const (
	FieldTypeCurrency   FieldType = "currency"
	FieldTypePercentage FieldType = "percentage"
	FieldTypeNumber     FieldType = "number"
	FieldTypeText       FieldType = "text"
)

// ItemBehavior represents the mathematical behavior of an item type
type ItemBehavior string

const (
	ItemBehaviorOutflow   ItemBehavior = "outflow"   // Reduces surplus (expenses)
	ItemBehaviorInflow    ItemBehavior = "inflow"    // Adds to surplus (income)
	ItemBehaviorAsset     ItemBehavior = "asset"     // Tracked on balance sheet (+)
	ItemBehaviorLiability ItemBehavior = "liability" // Tracked on balance sheet (-)
)

// TargetTab represents which UI tab displays items of this type
type TargetTab string

const (
	TargetTabBudgets     TargetTab = "budgets"
	TargetTabRecurring   TargetTab = "recurring"
	TargetTabGoals       TargetTab = "goals"
	TargetTabIncome      TargetTab = "income"
	TargetTabPortfolio   TargetTab = "portfolio"
	TargetTabLiabilities TargetTab = "liabilities"
)

// UserPlan represents a user's financial plan
type UserPlan struct {
	ID                 uuid.UUID      `db:"id"`
	UserID             uuid.UUID      `db:"user_id"`
	Name               string         `db:"name"`
	Description        *string        `db:"description"`
	Status             PlanStatus     `db:"status"`
	SourceType         PlanSourceType `db:"source_type"`
	SourceFileID       *uuid.UUID     `db:"source_file_id"`
	ExcelSheetName     *string        `db:"excel_sheet_name"`
	Config             []byte         `db:"config"` // JSONB
	TotalIncomeMinor   int64          `db:"total_income_minor"`
	TotalExpensesMinor int64          `db:"total_expenses_minor"`
	CurrencyCode       string         `db:"currency_code"`
	CreatedAt          time.Time      `db:"created_at"`
	UpdatedAt          time.Time      `db:"updated_at"`
}

// PlanCategoryGroup represents a high-level category grouping
type PlanCategoryGroup struct {
	ID            uuid.UUID `db:"id"`
	PlanID        uuid.UUID `db:"plan_id"`
	Name          string    `db:"name"`
	Color         *string   `db:"color"`
	TargetPercent float64   `db:"target_percent"`
	SortOrder     int       `db:"sort_order"`
	Labels        []byte    `db:"labels"` // JSONB
	CreatedAt     time.Time `db:"created_at"`
}

// PlanCategory represents a category within a plan
type PlanCategory struct {
	ID        uuid.UUID  `db:"id"`
	PlanID    uuid.UUID  `db:"plan_id"`
	GroupID   *uuid.UUID `db:"group_id"`
	Name      string     `db:"name"`
	Icon      *string    `db:"icon"`
	Color     *string    `db:"color"`
	SortOrder int        `db:"sort_order"`
	Labels    []byte     `db:"labels"` // JSONB
	CreatedAt time.Time  `db:"created_at"`
}

// PlanItem represents a single budget line item
type PlanItem struct {
	ID            uuid.UUID  `db:"id"`
	PlanID        uuid.UUID  `db:"plan_id"`
	CategoryID    *uuid.UUID `db:"category_id"`
	Name          string     `db:"name"`
	BudgetedMinor int64      `db:"budgeted_minor"`
	ActualMinor   int64      `db:"actual_minor"`
	ExcelCell     *string    `db:"excel_cell"`
	Formula       *string    `db:"formula"`
	WidgetType    WidgetType `db:"widget_type"`
	FieldType     FieldType  `db:"field_type"`
	SortOrder     int        `db:"sort_order"`
	MinValue      *int64     `db:"min_value"`
	MaxValue      *int64     `db:"max_value"`
	Labels        []byte     `db:"labels"`    // JSONB
	ConfigID      *uuid.UUID `db:"config_id"` // Link to dynamic item config
	CreatedAt     time.Time  `db:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at"`
}

// ItemConfig represents a user-configurable item type
type ItemConfig struct {
	ID        uuid.UUID    `db:"id"`
	UserID    uuid.UUID    `db:"user_id"`
	Label     string       `db:"label"`
	ShortCode string       `db:"short_code"`
	Behavior  ItemBehavior `db:"behavior"`
	TargetTab TargetTab    `db:"target_tab"`
	ColorHex  string       `db:"color_hex"`
	Icon      string       `db:"icon"`
	IsSystem  bool         `db:"is_system"`
	SortOrder int          `db:"sort_order"`
	CreatedAt time.Time    `db:"created_at"`
	UpdatedAt time.Time    `db:"updated_at"`
}

// PlanRepository defines the interface for plan data access
type PlanRepository interface {
	// Plans
	CreatePlan(ctx context.Context, plan *UserPlan) error
	GetPlanByID(ctx context.Context, planID uuid.UUID) (*UserPlan, error)
	ListPlansByUser(ctx context.Context, userID uuid.UUID, status *PlanStatus, limit, offset int) ([]*UserPlan, int, error)
	ListAllActivePlans(ctx context.Context, limit, offset int) ([]*UserPlan, error) // For cron jobs
	UpdatePlan(ctx context.Context, plan *UserPlan) error
	DeletePlan(ctx context.Context, planID uuid.UUID) error
	SetActivePlan(ctx context.Context, userID, planID uuid.UUID) error

	// Category Groups
	CreateCategoryGroup(ctx context.Context, group *PlanCategoryGroup) error
	GetCategoryGroupsByPlan(ctx context.Context, planID uuid.UUID) ([]*PlanCategoryGroup, error)

	// Categories
	CreateCategory(ctx context.Context, category *PlanCategory) error
	GetCategoriesByPlan(ctx context.Context, planID uuid.UUID) ([]*PlanCategory, error)
	GetCategoriesByGroup(ctx context.Context, groupID uuid.UUID) ([]*PlanCategory, error)

	// Items
	CreateItem(ctx context.Context, item *PlanItem) error
	GetItemsByPlan(ctx context.Context, planID uuid.UUID) ([]*PlanItem, error)
	GetItemsByCategory(ctx context.Context, categoryID uuid.UUID) ([]*PlanItem, error)
	UpdateItem(ctx context.Context, item *PlanItem) error
	UpdateItemBudget(ctx context.Context, itemID uuid.UUID, budgetedMinor int64) error
	UpdatePlanItemActual(ctx context.Context, itemID uuid.UUID, actualMinor int64) error

	// Bulk operations
	CreatePlanWithStructure(ctx context.Context, plan *UserPlan, groups []*PlanCategoryGroup, categories []*PlanCategory, items []*PlanItem) error
	DuplicatePlan(ctx context.Context, sourcePlanID uuid.UUID, newName string, userID uuid.UUID) (*UserPlan, error)

	// Item Configs (dynamic type configurations)
	ListItemConfigs(ctx context.Context, userID uuid.UUID) ([]*ItemConfig, error)
	GetItemConfigByID(ctx context.Context, configID uuid.UUID) (*ItemConfig, error)
	CreateItemConfig(ctx context.Context, config *ItemConfig) error
	UpdateItemConfig(ctx context.Context, config *ItemConfig) error
	DeleteItemConfig(ctx context.Context, configID uuid.UUID) error

	// Filtered queries
	GetItemsByTabWithTotals(ctx context.Context, planID uuid.UUID, targetTab TargetTab) ([]PlanItemWithConfig, int64, int64, error)
}

// CreatePlanInput is used for creating a new plan with its full structure
type CreatePlanInput struct {
	Name           string
	Description    *string
	CurrencyCode   string
	SourceType     PlanSourceType
	SourceFileID   *uuid.UUID
	ExcelSheetName *string
	Config         []byte
	CategoryGroups []CreateCategoryGroupInput
}

// CreateCategoryGroupInput is used for creating category groups
type CreateCategoryGroupInput struct {
	Name          string
	Color         *string
	TargetPercent float64
	Labels        map[string]string
	Categories    []CreateCategoryInput
}

// CreateCategoryInput is used for creating categories
type CreateCategoryInput struct {
	Name   string
	Icon   *string
	Labels map[string]string
	Items  []CreateItemInput
}

// CreateItemInput is used for creating items
type CreateItemInput struct {
	Name          string
	BudgetedMinor int64
	WidgetType    WidgetType
	FieldType     FieldType
	ExcelCell     *string
	Formula       *string
	Labels        map[string]string
}
