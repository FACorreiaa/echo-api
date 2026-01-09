// Package repository provides item filtering by target tab
package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PlanItemWithConfig represents an item with its config details
type PlanItemWithConfig struct {
	ID              uuid.UUID     `db:"id"`
	Name            string        `db:"name"`
	BudgetedMinor   int64         `db:"budgeted_minor"`
	ActualMinor     int64         `db:"actual_minor"`
	CategoryName    *string       `db:"category_name"`
	GroupName       *string       `db:"group_name"`
	ConfigID        *uuid.UUID    `db:"config_id"`
	ConfigLabel     *string       `db:"config_label"`
	ConfigShortCode *string       `db:"config_short_code"`
	ConfigColorHex  *string       `db:"config_color_hex"`
	Behavior        *ItemBehavior `db:"behavior"`
}

// GetItemsByTab returns items for a plan filtered by target tab
func (r *PostgresPlanRepository) GetItemsByTab(ctx context.Context, planID uuid.UUID, targetTab TargetTab) ([]PlanItemWithConfig, error) {
	query := `
		SELECT 
			pi.id,
			pi.name,
			pi.budgeted_minor,
			pi.actual_minor,
			pc.name AS category_name,
			pcg.name AS group_name,
			pi.config_id,
			pic.label AS config_label,
			pic.short_code AS config_short_code,
			pic.color_hex AS config_color_hex,
			pic.behavior
		FROM plan_items pi
		LEFT JOIN plan_categories pc ON pi.category_id = pc.id
		LEFT JOIN plan_category_groups pcg ON pc.group_id = pcg.id
		LEFT JOIN plan_item_configs pic ON pi.config_id = pic.id
		WHERE pcg.plan_id = $1
		  AND pic.target_tab = $2
		ORDER BY pcg.sort_order, pc.sort_order, pi.sort_order
	`

	rows, err := r.pool.Query(ctx, query, planID, targetTab)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []PlanItemWithConfig
	for rows.Next() {
		var item PlanItemWithConfig
		var behavior *string
		err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.BudgetedMinor,
			&item.ActualMinor,
			&item.CategoryName,
			&item.GroupName,
			&item.ConfigID,
			&item.ConfigLabel,
			&item.ConfigShortCode,
			&item.ConfigColorHex,
			&behavior,
		)
		if err != nil {
			return nil, err
		}
		if behavior != nil {
			b := ItemBehavior(*behavior)
			item.Behavior = &b
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

// GetItemsByTabWithTotals returns items and totals for a plan filtered by target tab
func (r *PostgresPlanRepository) GetItemsByTabWithTotals(ctx context.Context, planID uuid.UUID, targetTab TargetTab) ([]PlanItemWithConfig, int64, int64, error) {
	items, err := r.GetItemsByTab(ctx, planID, targetTab)
	if err != nil {
		return nil, 0, 0, err
	}

	var totalBudgeted, totalActual int64
	for _, item := range items {
		totalBudgeted += item.BudgetedMinor
		totalActual += item.ActualMinor
	}

	return items, totalBudgeted, totalActual, nil
}

// Suppress unused import warning
var _ = pgx.ErrNoRows
