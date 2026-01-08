package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresPlanRepository implements PlanRepository using PostgreSQL
type PostgresPlanRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresPlanRepository creates a new PostgreSQL-backed plan repository
func NewPostgresPlanRepository(pool *pgxpool.Pool) *PostgresPlanRepository {
	return &PostgresPlanRepository{pool: pool}
}

// ============================================================================
// Plans
// ============================================================================

// CreatePlan creates a new plan
func (r *PostgresPlanRepository) CreatePlan(ctx context.Context, plan *UserPlan) error {
	if plan.ID == uuid.Nil {
		plan.ID = uuid.New()
	}

	query := `
		INSERT INTO user_plans (
			id, user_id, name, description, status, source_type,
			source_file_id, excel_sheet_name, config, currency_code
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := r.pool.Exec(ctx, query,
		plan.ID, plan.UserID, plan.Name, plan.Description, plan.Status, plan.SourceType,
		plan.SourceFileID, plan.ExcelSheetName, plan.Config, plan.CurrencyCode,
	)
	if err != nil {
		return fmt.Errorf("failed to create plan: %w", err)
	}

	return nil
}

// GetPlanByID retrieves a plan by ID
func (r *PostgresPlanRepository) GetPlanByID(ctx context.Context, planID uuid.UUID) (*UserPlan, error) {
	query := `
		SELECT id, user_id, name, description, status, source_type,
		       source_file_id, excel_sheet_name, config,
		       total_income_minor, total_expenses_minor, currency_code,
		       created_at, updated_at
		FROM user_plans WHERE id = $1
	`

	var plan UserPlan
	err := r.pool.QueryRow(ctx, query, planID).Scan(
		&plan.ID, &plan.UserID, &plan.Name, &plan.Description, &plan.Status, &plan.SourceType,
		&plan.SourceFileID, &plan.ExcelSheetName, &plan.Config,
		&plan.TotalIncomeMinor, &plan.TotalExpensesMinor, &plan.CurrencyCode,
		&plan.CreatedAt, &plan.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get plan: %w", err)
	}

	return &plan, nil
}

// ListPlansByUser lists all plans for a user
func (r *PostgresPlanRepository) ListPlansByUser(ctx context.Context, userID uuid.UUID, status *PlanStatus, limit, offset int) ([]*UserPlan, int, error) {
	// Count query
	countArgs := []any{userID}
	countQuery := `SELECT COUNT(*) FROM user_plans WHERE user_id = $1 AND status != 'archived'`
	if status != nil {
		countQuery += ` AND status = $2`
		countArgs = append(countArgs, *status)
	}

	var total int
	if err := r.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count plans: %w", err)
	}

	// List query
	args := []any{userID}
	query := `
		SELECT id, user_id, name, description, status, source_type,
		       source_file_id, excel_sheet_name, config,
		       total_income_minor, total_expenses_minor, currency_code,
		       created_at, updated_at
		FROM user_plans
		WHERE user_id = $1 AND status != 'archived'
	`
	argIdx := 2
	if status != nil {
		query += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, *status)
		argIdx++
	}
	query += fmt.Sprintf(` ORDER BY updated_at DESC LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list plans: %w", err)
	}
	defer rows.Close()

	var plans []*UserPlan
	for rows.Next() {
		var p UserPlan
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.Name, &p.Description, &p.Status, &p.SourceType,
			&p.SourceFileID, &p.ExcelSheetName, &p.Config,
			&p.TotalIncomeMinor, &p.TotalExpensesMinor, &p.CurrencyCode,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan plan: %w", err)
		}
		plans = append(plans, &p)
	}

	return plans, total, nil
}

// ListAllActivePlans retrieves all active plans across all users (for cron jobs)
func (r *PostgresPlanRepository) ListAllActivePlans(ctx context.Context, limit, offset int) ([]*UserPlan, error) {
	query := `
		SELECT id, user_id, name, description, status, source_type,
		       source_file_id, excel_sheet_name, config,
		       total_income_minor, total_expenses_minor, currency_code,
		       created_at, updated_at
		FROM user_plans
		WHERE status = 'active'
		ORDER BY updated_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list active plans: %w", err)
	}
	defer rows.Close()

	var plans []*UserPlan
	for rows.Next() {
		var p UserPlan
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.Name, &p.Description, &p.Status, &p.SourceType,
			&p.SourceFileID, &p.ExcelSheetName, &p.Config,
			&p.TotalIncomeMinor, &p.TotalExpensesMinor, &p.CurrencyCode,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan active plan: %w", err)
		}
		plans = append(plans, &p)
	}

	return plans, nil
}

// UpdatePlan updates an existing plan
func (r *PostgresPlanRepository) UpdatePlan(ctx context.Context, plan *UserPlan) error {
	query := `
		UPDATE user_plans SET
			name = $2, description = $3, status = $4, config = $5
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, query, plan.ID, plan.Name, plan.Description, plan.Status, plan.Config)
	if err != nil {
		return fmt.Errorf("failed to update plan: %w", err)
	}

	return nil
}

// DeletePlan soft-deletes a plan by setting status to archived
func (r *PostgresPlanRepository) DeletePlan(ctx context.Context, planID uuid.UUID) error {
	query := `UPDATE user_plans SET status = 'archived' WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, planID)
	if err != nil {
		return fmt.Errorf("failed to delete plan: %w", err)
	}
	return nil
}

// SetActivePlan marks a plan as active (deactivating any other active plan for the user)
func (r *PostgresPlanRepository) SetActivePlan(ctx context.Context, userID, planID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Deactivate current active plan
	_, err = tx.Exec(ctx, `UPDATE user_plans SET status = 'draft' WHERE user_id = $1 AND status = 'active'`, userID)
	if err != nil {
		return fmt.Errorf("failed to deactivate plans: %w", err)
	}

	// Activate selected plan
	_, err = tx.Exec(ctx, `UPDATE user_plans SET status = 'active' WHERE id = $1 AND user_id = $2`, planID, userID)
	if err != nil {
		return fmt.Errorf("failed to activate plan: %w", err)
	}

	return tx.Commit(ctx)
}

// ============================================================================
// Category Groups
// ============================================================================

// CreateCategoryGroup creates a new category group
func (r *PostgresPlanRepository) CreateCategoryGroup(ctx context.Context, group *PlanCategoryGroup) error {
	if group.ID == uuid.Nil {
		group.ID = uuid.New()
	}

	query := `
		INSERT INTO plan_category_groups (id, plan_id, name, color, target_percent, sort_order, labels)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.pool.Exec(ctx, query,
		group.ID, group.PlanID, group.Name, group.Color, group.TargetPercent, group.SortOrder, group.Labels,
	)
	if err != nil {
		return fmt.Errorf("failed to create category group: %w", err)
	}

	return nil
}

// GetCategoryGroupsByPlan retrieves all category groups for a plan
func (r *PostgresPlanRepository) GetCategoryGroupsByPlan(ctx context.Context, planID uuid.UUID) ([]*PlanCategoryGroup, error) {
	query := `
		SELECT id, plan_id, name, color, target_percent, sort_order, labels, created_at
		FROM plan_category_groups
		WHERE plan_id = $1
		ORDER BY sort_order
	`

	rows, err := r.pool.Query(ctx, query, planID)
	if err != nil {
		return nil, fmt.Errorf("failed to get category groups: %w", err)
	}
	defer rows.Close()

	var groups []*PlanCategoryGroup
	for rows.Next() {
		var g PlanCategoryGroup
		if err := rows.Scan(&g.ID, &g.PlanID, &g.Name, &g.Color, &g.TargetPercent, &g.SortOrder, &g.Labels, &g.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan category group: %w", err)
		}
		groups = append(groups, &g)
	}

	return groups, nil
}

// ============================================================================
// Categories
// ============================================================================

// CreateCategory creates a new category
func (r *PostgresPlanRepository) CreateCategory(ctx context.Context, category *PlanCategory) error {
	if category.ID == uuid.Nil {
		category.ID = uuid.New()
	}

	query := `
		INSERT INTO plan_categories (id, plan_id, group_id, name, icon, color, sort_order, labels)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.pool.Exec(ctx, query,
		category.ID, category.PlanID, category.GroupID, category.Name, category.Icon, category.Color, category.SortOrder, category.Labels,
	)
	if err != nil {
		return fmt.Errorf("failed to create category: %w", err)
	}

	return nil
}

// GetCategoriesByPlan retrieves all categories for a plan
func (r *PostgresPlanRepository) GetCategoriesByPlan(ctx context.Context, planID uuid.UUID) ([]*PlanCategory, error) {
	query := `
		SELECT id, plan_id, group_id, name, icon, color, sort_order, labels, created_at
		FROM plan_categories
		WHERE plan_id = $1
		ORDER BY sort_order
	`

	rows, err := r.pool.Query(ctx, query, planID)
	if err != nil {
		return nil, fmt.Errorf("failed to get categories: %w", err)
	}
	defer rows.Close()

	var categories []*PlanCategory
	for rows.Next() {
		var c PlanCategory
		if err := rows.Scan(&c.ID, &c.PlanID, &c.GroupID, &c.Name, &c.Icon, &c.Color, &c.SortOrder, &c.Labels, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}
		categories = append(categories, &c)
	}

	return categories, nil
}

// GetCategoriesByGroup retrieves all categories for a group
func (r *PostgresPlanRepository) GetCategoriesByGroup(ctx context.Context, groupID uuid.UUID) ([]*PlanCategory, error) {
	query := `
		SELECT id, plan_id, group_id, name, icon, color, sort_order, labels, created_at
		FROM plan_categories
		WHERE group_id = $1
		ORDER BY sort_order
	`

	rows, err := r.pool.Query(ctx, query, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get categories by group: %w", err)
	}
	defer rows.Close()

	var categories []*PlanCategory
	for rows.Next() {
		var c PlanCategory
		if err := rows.Scan(&c.ID, &c.PlanID, &c.GroupID, &c.Name, &c.Icon, &c.Color, &c.SortOrder, &c.Labels, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}
		categories = append(categories, &c)
	}

	return categories, nil
}

// ============================================================================
// Items
// ============================================================================

// CreateItem creates a new plan item
func (r *PostgresPlanRepository) CreateItem(ctx context.Context, item *PlanItem) error {
	if item.ID == uuid.Nil {
		item.ID = uuid.New()
	}

	query := `
		INSERT INTO plan_items (
			id, plan_id, category_id, name, budgeted_minor, actual_minor,
			excel_cell, formula, widget_type, field_type, sort_order,
			min_value, max_value, labels
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	_, err := r.pool.Exec(ctx, query,
		item.ID, item.PlanID, item.CategoryID, item.Name, item.BudgetedMinor, item.ActualMinor,
		item.ExcelCell, item.Formula, item.WidgetType, item.FieldType, item.SortOrder,
		item.MinValue, item.MaxValue, item.Labels,
	)
	if err != nil {
		return fmt.Errorf("failed to create item: %w", err)
	}

	return nil
}

// GetItemsByPlan retrieves all items for a plan
func (r *PostgresPlanRepository) GetItemsByPlan(ctx context.Context, planID uuid.UUID) ([]*PlanItem, error) {
	query := `
		SELECT id, plan_id, category_id, name, budgeted_minor, actual_minor,
		       excel_cell, formula, widget_type, field_type, sort_order,
		       min_value, max_value, labels, created_at, updated_at
		FROM plan_items
		WHERE plan_id = $1
		ORDER BY sort_order
	`

	rows, err := r.pool.Query(ctx, query, planID)
	if err != nil {
		return nil, fmt.Errorf("failed to get items: %w", err)
	}
	defer rows.Close()

	var items []*PlanItem
	for rows.Next() {
		var i PlanItem
		if err := rows.Scan(
			&i.ID, &i.PlanID, &i.CategoryID, &i.Name, &i.BudgetedMinor, &i.ActualMinor,
			&i.ExcelCell, &i.Formula, &i.WidgetType, &i.FieldType, &i.SortOrder,
			&i.MinValue, &i.MaxValue, &i.Labels, &i.CreatedAt, &i.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan item: %w", err)
		}
		items = append(items, &i)
	}

	return items, nil
}

// GetItemsByCategory retrieves all items for a category
func (r *PostgresPlanRepository) GetItemsByCategory(ctx context.Context, categoryID uuid.UUID) ([]*PlanItem, error) {
	query := `
		SELECT id, plan_id, category_id, name, budgeted_minor, actual_minor,
		       excel_cell, formula, widget_type, field_type, sort_order,
		       min_value, max_value, labels, created_at, updated_at
		FROM plan_items
		WHERE category_id = $1
		ORDER BY sort_order
	`

	rows, err := r.pool.Query(ctx, query, categoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get items by category: %w", err)
	}
	defer rows.Close()

	var items []*PlanItem
	for rows.Next() {
		var i PlanItem
		if err := rows.Scan(
			&i.ID, &i.PlanID, &i.CategoryID, &i.Name, &i.BudgetedMinor, &i.ActualMinor,
			&i.ExcelCell, &i.Formula, &i.WidgetType, &i.FieldType, &i.SortOrder,
			&i.MinValue, &i.MaxValue, &i.Labels, &i.CreatedAt, &i.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan item: %w", err)
		}
		items = append(items, &i)
	}

	return items, nil
}

// UpdateItem updates an existing plan item
func (r *PostgresPlanRepository) UpdateItem(ctx context.Context, item *PlanItem) error {
	query := `
		UPDATE plan_items SET
			name = $2, budgeted_minor = $3, actual_minor = $4,
			widget_type = $5, field_type = $6, labels = $7
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, query,
		item.ID, item.Name, item.BudgetedMinor, item.ActualMinor,
		item.WidgetType, item.FieldType, item.Labels,
	)
	if err != nil {
		return fmt.Errorf("failed to update item: %w", err)
	}

	return nil
}

// UpdateItemBudget updates just the budgeted amount for an item
func (r *PostgresPlanRepository) UpdateItemBudget(ctx context.Context, itemID uuid.UUID, budgetedMinor int64) error {
	query := `UPDATE plan_items SET budgeted_minor = $2 WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, itemID, budgetedMinor)
	if err != nil {
		return fmt.Errorf("failed to update item budget: %w", err)
	}
	return nil
}

// UpdatePlanItemActual updates just the actual amount for an item (from transaction sync)
func (r *PostgresPlanRepository) UpdatePlanItemActual(ctx context.Context, itemID uuid.UUID, actualMinor int64) error {
	query := `UPDATE plan_items SET actual_minor = $2, updated_at = NOW() WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, itemID, actualMinor)
	if err != nil {
		return fmt.Errorf("failed to update item actual: %w", err)
	}
	return nil
}

// ============================================================================
// Bulk Operations
// ============================================================================

// CreatePlanWithStructure creates a plan with all its related entities in a transaction
func (r *PostgresPlanRepository) CreatePlanWithStructure(ctx context.Context, plan *UserPlan, groups []*PlanCategoryGroup, categories []*PlanCategory, items []*PlanItem) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Create plan
	if plan.ID == uuid.Nil {
		plan.ID = uuid.New()
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO user_plans (
			id, user_id, name, description, status, source_type,
			source_file_id, excel_sheet_name, config, currency_code
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`,
		plan.ID, plan.UserID, plan.Name, plan.Description, plan.Status, plan.SourceType,
		plan.SourceFileID, plan.ExcelSheetName, plan.Config, plan.CurrencyCode,
	)
	if err != nil {
		return fmt.Errorf("failed to create plan: %w", err)
	}

	// Create groups
	for _, g := range groups {
		if g.ID == uuid.Nil {
			g.ID = uuid.New()
		}
		g.PlanID = plan.ID
		_, err = tx.Exec(ctx, `
			INSERT INTO plan_category_groups (id, plan_id, name, color, target_percent, sort_order, labels)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, g.ID, g.PlanID, g.Name, g.Color, g.TargetPercent, g.SortOrder, g.Labels)
		if err != nil {
			return fmt.Errorf("failed to create category group: %w", err)
		}
	}

	// Create categories
	for _, c := range categories {
		if c.ID == uuid.Nil {
			c.ID = uuid.New()
		}
		c.PlanID = plan.ID
		_, err = tx.Exec(ctx, `
			INSERT INTO plan_categories (id, plan_id, group_id, name, icon, color, sort_order, labels)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, c.ID, c.PlanID, c.GroupID, c.Name, c.Icon, c.Color, c.SortOrder, c.Labels)
		if err != nil {
			return fmt.Errorf("failed to create category: %w", err)
		}
	}

	// Create items
	for _, i := range items {
		if i.ID == uuid.Nil {
			i.ID = uuid.New()
		}
		i.PlanID = plan.ID
		_, err = tx.Exec(ctx, `
			INSERT INTO plan_items (
				id, plan_id, category_id, name, budgeted_minor, actual_minor,
				excel_cell, formula, widget_type, field_type, sort_order,
				min_value, max_value, labels
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		`,
			i.ID, i.PlanID, i.CategoryID, i.Name, i.BudgetedMinor, i.ActualMinor,
			i.ExcelCell, i.Formula, i.WidgetType, i.FieldType, i.SortOrder,
			i.MinValue, i.MaxValue, i.Labels,
		)
		if err != nil {
			return fmt.Errorf("failed to create item: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// DuplicatePlan creates a copy of an existing plan
func (r *PostgresPlanRepository) DuplicatePlan(ctx context.Context, sourcePlanID uuid.UUID, newName string, userID uuid.UUID) (*UserPlan, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	newPlanID := uuid.New()

	// Copy plan
	_, err = tx.Exec(ctx, `
		INSERT INTO user_plans (id, user_id, name, description, status, source_type, config, currency_code)
		SELECT $1, user_id, $2, description, 'draft', source_type, config, currency_code
		FROM user_plans WHERE id = $3 AND user_id = $4
	`, newPlanID, newName, sourcePlanID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to duplicate plan: %w", err)
	}

	// Map old group IDs to new group IDs
	groupMapping := make(map[uuid.UUID]uuid.UUID)
	groupRows, err := tx.Query(ctx, `SELECT id FROM plan_category_groups WHERE plan_id = $1`, sourcePlanID)
	if err != nil {
		return nil, fmt.Errorf("failed to get source groups: %w", err)
	}
	for groupRows.Next() {
		var oldID uuid.UUID
		groupRows.Scan(&oldID)
		newID := uuid.New()
		groupMapping[oldID] = newID

		_, err = tx.Exec(ctx, `
			INSERT INTO plan_category_groups (id, plan_id, name, color, target_percent, sort_order, labels)
			SELECT $1, $2, name, color, target_percent, sort_order, labels
			FROM plan_category_groups WHERE id = $3
		`, newID, newPlanID, oldID)
		if err != nil {
			groupRows.Close()
			return nil, fmt.Errorf("failed to duplicate group: %w", err)
		}
	}
	groupRows.Close()

	// Map old category IDs to new category IDs
	categoryMapping := make(map[uuid.UUID]uuid.UUID)
	catRows, err := tx.Query(ctx, `SELECT id, group_id FROM plan_categories WHERE plan_id = $1`, sourcePlanID)
	if err != nil {
		return nil, fmt.Errorf("failed to get source categories: %w", err)
	}
	for catRows.Next() {
		var oldID uuid.UUID
		var oldGroupID *uuid.UUID
		catRows.Scan(&oldID, &oldGroupID)
		newID := uuid.New()
		categoryMapping[oldID] = newID

		var newGroupID *uuid.UUID
		if oldGroupID != nil {
			if g, ok := groupMapping[*oldGroupID]; ok {
				newGroupID = &g
			}
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO plan_categories (id, plan_id, group_id, name, icon, color, sort_order, labels)
			SELECT $1, $2, $3, name, icon, color, sort_order, labels
			FROM plan_categories WHERE id = $4
		`, newID, newPlanID, newGroupID, oldID)
		if err != nil {
			catRows.Close()
			return nil, fmt.Errorf("failed to duplicate category: %w", err)
		}
	}
	catRows.Close()

	// Duplicate items
	itemRows, err := tx.Query(ctx, `SELECT id, category_id FROM plan_items WHERE plan_id = $1`, sourcePlanID)
	if err != nil {
		return nil, fmt.Errorf("failed to get source items: %w", err)
	}
	for itemRows.Next() {
		var oldItemID uuid.UUID
		var oldCatID *uuid.UUID
		itemRows.Scan(&oldItemID, &oldCatID)
		newItemID := uuid.New()

		var newCatID *uuid.UUID
		if oldCatID != nil {
			if c, ok := categoryMapping[*oldCatID]; ok {
				newCatID = &c
			}
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO plan_items (id, plan_id, category_id, name, budgeted_minor, actual_minor,
				excel_cell, formula, widget_type, field_type, sort_order, min_value, max_value, labels)
			SELECT $1, $2, $3, name, budgeted_minor, 0, excel_cell, formula, widget_type, field_type,
				sort_order, min_value, max_value, labels
			FROM plan_items WHERE id = $4
		`, newItemID, newPlanID, newCatID, oldItemID)
		if err != nil {
			itemRows.Close()
			return nil, fmt.Errorf("failed to duplicate item: %w", err)
		}
	}
	itemRows.Close()

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}

	return r.GetPlanByID(ctx, newPlanID)
}

// ============================================================================
// Item Configs (Dynamic Type Configurations)
// ============================================================================

// ListItemConfigs retrieves all item configs for a user
func (r *PostgresPlanRepository) ListItemConfigs(ctx context.Context, userID uuid.UUID) ([]*ItemConfig, error) {
	query := `
		SELECT id, user_id, label, short_code, behavior, target_tab,
		       color_hex, icon, is_system, sort_order, created_at, updated_at
		FROM plan_item_configs
		WHERE user_id = $1
		ORDER BY sort_order, label
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list item configs: %w", err)
	}
	defer rows.Close()

	var configs []*ItemConfig
	for rows.Next() {
		var c ItemConfig
		if err := rows.Scan(
			&c.ID, &c.UserID, &c.Label, &c.ShortCode, &c.Behavior, &c.TargetTab,
			&c.ColorHex, &c.Icon, &c.IsSystem, &c.SortOrder, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan item config: %w", err)
		}
		configs = append(configs, &c)
	}

	return configs, nil
}

// GetItemConfigByID retrieves a config by ID
func (r *PostgresPlanRepository) GetItemConfigByID(ctx context.Context, configID uuid.UUID) (*ItemConfig, error) {
	query := `
		SELECT id, user_id, label, short_code, behavior, target_tab,
		       color_hex, icon, is_system, sort_order, created_at, updated_at
		FROM plan_item_configs
		WHERE id = $1
	`

	var c ItemConfig
	err := r.pool.QueryRow(ctx, query, configID).Scan(
		&c.ID, &c.UserID, &c.Label, &c.ShortCode, &c.Behavior, &c.TargetTab,
		&c.ColorHex, &c.Icon, &c.IsSystem, &c.SortOrder, &c.CreatedAt, &c.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get item config: %w", err)
	}

	return &c, nil
}

// CreateItemConfig creates a new item config
func (r *PostgresPlanRepository) CreateItemConfig(ctx context.Context, config *ItemConfig) error {
	if config.ID == uuid.Nil {
		config.ID = uuid.New()
	}

	query := `
		INSERT INTO plan_item_configs (
			id, user_id, label, short_code, behavior, target_tab,
			color_hex, icon, is_system, sort_order
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := r.pool.Exec(ctx, query,
		config.ID, config.UserID, config.Label, config.ShortCode, config.Behavior, config.TargetTab,
		config.ColorHex, config.Icon, config.IsSystem, config.SortOrder,
	)
	if err != nil {
		return fmt.Errorf("failed to create item config: %w", err)
	}

	return nil
}

// UpdateItemConfig updates an existing item config
func (r *PostgresPlanRepository) UpdateItemConfig(ctx context.Context, config *ItemConfig) error {
	query := `
		UPDATE plan_item_configs SET
			label = $2, short_code = $3, behavior = $4, target_tab = $5,
			color_hex = $6, icon = $7, sort_order = $8, updated_at = NOW()
		WHERE id = $1 AND is_system = false
	`

	result, err := r.pool.Exec(ctx, query,
		config.ID, config.Label, config.ShortCode, config.Behavior, config.TargetTab,
		config.ColorHex, config.Icon, config.SortOrder,
	)
	if err != nil {
		return fmt.Errorf("failed to update item config: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("item config not found or is a system config")
	}

	return nil
}

// DeleteItemConfig deletes an item config (only non-system configs)
func (r *PostgresPlanRepository) DeleteItemConfig(ctx context.Context, configID uuid.UUID) error {
	query := `DELETE FROM plan_item_configs WHERE id = $1 AND is_system = false`

	result, err := r.pool.Exec(ctx, query, configID)
	if err != nil {
		return fmt.Errorf("failed to delete item config: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("item config not found or is a system config")
	}

	return nil
}

// marshalLabels converts a map to JSON bytes
func marshalLabels(labels map[string]string) []byte {
	if labels == nil {
		return []byte("{}")
	}
	b, _ := json.Marshal(labels)
	return b
}
