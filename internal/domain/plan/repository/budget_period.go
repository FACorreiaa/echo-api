// Package repository provides data access for budget periods
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BudgetPeriod represents a monthly snapshot of budget values
type BudgetPeriod struct {
	ID        uuid.UUID
	PlanID    uuid.UUID
	Year      int
	Month     int
	IsLocked  bool
	Notes     *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// BudgetPeriodItem represents item values for a specific period
type BudgetPeriodItem struct {
	ID            uuid.UUID
	PeriodID      uuid.UUID
	ItemID        uuid.UUID
	ItemName      string
	CategoryName  string
	BudgetedMinor int64
	ActualMinor   int64
	Notes         *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// BudgetPeriodWithItems is a period with its items
type BudgetPeriodWithItems struct {
	Period *BudgetPeriod
	Items  []*BudgetPeriodItem
}

// BudgetPeriodRepository defines the interface for budget period operations
type BudgetPeriodRepository interface {
	// GetOrCreatePeriod retrieves or creates a budget period for a specific month
	GetOrCreatePeriod(ctx context.Context, planID uuid.UUID, year, month int) (*BudgetPeriodWithItems, bool, error)

	// ListPeriods lists all budget periods for a plan
	ListPeriods(ctx context.Context, planID uuid.UUID) ([]*BudgetPeriod, error)

	// GetPeriodByID gets a period by ID with items
	GetPeriodByID(ctx context.Context, periodID uuid.UUID) (*BudgetPeriodWithItems, error)

	// UpdatePeriodItem updates a period item's values
	UpdatePeriodItem(ctx context.Context, periodItemID uuid.UUID, budgeted, actual *int64, notes *string) (*BudgetPeriodItem, error)

	// CopyPeriodItems copies items from one period to another
	CopyPeriodItems(ctx context.Context, sourcePeriodID uuid.UUID, targetPlanID uuid.UUID, targetYear, targetMonth int) (*BudgetPeriodWithItems, error)
}

// PostgresBudgetPeriodRepository implements BudgetPeriodRepository with PostgreSQL
type PostgresBudgetPeriodRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresBudgetPeriodRepository creates a new repository instance
func NewPostgresBudgetPeriodRepository(pool *pgxpool.Pool) *PostgresBudgetPeriodRepository {
	return &PostgresBudgetPeriodRepository{pool: pool}
}

// GetOrCreatePeriod retrieves or creates a budget period
func (r *PostgresBudgetPeriodRepository) GetOrCreatePeriod(ctx context.Context, planID uuid.UUID, year, month int) (*BudgetPeriodWithItems, bool, error) {
	// Try to find existing period
	var period BudgetPeriod
	err := r.pool.QueryRow(ctx, `
		SELECT id, plan_id, year, month, is_locked, notes, created_at, updated_at
		FROM budget_periods
		WHERE plan_id = $1 AND year = $2 AND month = $3
	`, planID, year, month).Scan(
		&period.ID, &period.PlanID, &period.Year, &period.Month,
		&period.IsLocked, &period.Notes, &period.CreatedAt, &period.UpdatedAt,
	)

	wasCreated := false
	if err == pgx.ErrNoRows {
		// Create new period (trigger will copy items)
		err = r.pool.QueryRow(ctx, `
			INSERT INTO budget_periods (plan_id, year, month)
			VALUES ($1, $2, $3)
			RETURNING id, plan_id, year, month, is_locked, notes, created_at, updated_at
		`, planID, year, month).Scan(
			&period.ID, &period.PlanID, &period.Year, &period.Month,
			&period.IsLocked, &period.Notes, &period.CreatedAt, &period.UpdatedAt,
		)
		if err != nil {
			return nil, false, fmt.Errorf("failed to create budget period: %w", err)
		}
		wasCreated = true
	} else if err != nil {
		return nil, false, fmt.Errorf("failed to get budget period: %w", err)
	}

	// Get period items with names
	items, err := r.getPeriodItems(ctx, period.ID)
	if err != nil {
		return nil, wasCreated, err
	}

	return &BudgetPeriodWithItems{Period: &period, Items: items}, wasCreated, nil
}

// ListPeriods lists all periods for a plan
func (r *PostgresBudgetPeriodRepository) ListPeriods(ctx context.Context, planID uuid.UUID) ([]*BudgetPeriod, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, plan_id, year, month, is_locked, notes, created_at, updated_at
		FROM budget_periods
		WHERE plan_id = $1
		ORDER BY year DESC, month DESC
	`, planID)
	if err != nil {
		return nil, fmt.Errorf("failed to list budget periods: %w", err)
	}
	defer rows.Close()

	var periods []*BudgetPeriod
	for rows.Next() {
		var p BudgetPeriod
		if err := rows.Scan(
			&p.ID, &p.PlanID, &p.Year, &p.Month,
			&p.IsLocked, &p.Notes, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan budget period: %w", err)
		}
		periods = append(periods, &p)
	}

	return periods, nil
}

// GetPeriodByID gets a period with items
func (r *PostgresBudgetPeriodRepository) GetPeriodByID(ctx context.Context, periodID uuid.UUID) (*BudgetPeriodWithItems, error) {
	var period BudgetPeriod
	err := r.pool.QueryRow(ctx, `
		SELECT id, plan_id, year, month, is_locked, notes, created_at, updated_at
		FROM budget_periods
		WHERE id = $1
	`, periodID).Scan(
		&period.ID, &period.PlanID, &period.Year, &period.Month,
		&period.IsLocked, &period.Notes, &period.CreatedAt, &period.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get budget period: %w", err)
	}

	items, err := r.getPeriodItems(ctx, periodID)
	if err != nil {
		return nil, err
	}

	return &BudgetPeriodWithItems{Period: &period, Items: items}, nil
}

// UpdatePeriodItem updates a period item
func (r *PostgresBudgetPeriodRepository) UpdatePeriodItem(ctx context.Context, periodItemID uuid.UUID, budgeted, actual *int64, notes *string) (*BudgetPeriodItem, error) {
	var item BudgetPeriodItem
	err := r.pool.QueryRow(ctx, `
		UPDATE budget_period_items
		SET 
			budgeted_minor = COALESCE($2, budgeted_minor),
			actual_minor = COALESCE($3, actual_minor),
			notes = COALESCE($4, notes),
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, period_id, item_id, budgeted_minor, actual_minor, notes, created_at, updated_at
	`, periodItemID, budgeted, actual, notes).Scan(
		&item.ID, &item.PeriodID, &item.ItemID,
		&item.BudgetedMinor, &item.ActualMinor, &item.Notes,
		&item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update period item: %w", err)
	}

	// Get item name
	r.pool.QueryRow(ctx, `
		SELECT pi.name, COALESCE(pc.name, '') as cat_name
		FROM plan_items pi
		LEFT JOIN plan_categories pc ON pi.category_id = pc.id
		WHERE pi.id = $1
	`, item.ItemID).Scan(&item.ItemName, &item.CategoryName)

	return &item, nil
}

// CopyPeriodItems copies items from source to new target period
func (r *PostgresBudgetPeriodRepository) CopyPeriodItems(ctx context.Context, sourcePeriodID uuid.UUID, targetPlanID uuid.UUID, targetYear, targetMonth int) (*BudgetPeriodWithItems, error) {
	// Get or create target period (will auto-create items from plan)
	target, _, err := r.GetOrCreatePeriod(ctx, targetPlanID, targetYear, targetMonth)
	if err != nil {
		return nil, err
	}

	// Copy budgeted values from source period
	_, err = r.pool.Exec(ctx, `
		UPDATE budget_period_items tgt
		SET budgeted_minor = src.budgeted_minor
		FROM budget_period_items src
		WHERE src.period_id = $1
		  AND tgt.period_id = $2
		  AND src.item_id = tgt.item_id
	`, sourcePeriodID, target.Period.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to copy period items: %w", err)
	}

	// Refetch items with updated values
	return r.GetPeriodByID(ctx, target.Period.ID)
}

// getPeriodItems gets all items for a period with names
func (r *PostgresBudgetPeriodRepository) getPeriodItems(ctx context.Context, periodID uuid.UUID) ([]*BudgetPeriodItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT 
			bpi.id, bpi.period_id, bpi.item_id,
			pi.name as item_name,
			COALESCE(pc.name, 'Uncategorized') as category_name,
			bpi.budgeted_minor, bpi.actual_minor, bpi.notes,
			bpi.created_at, bpi.updated_at
		FROM budget_period_items bpi
		JOIN plan_items pi ON bpi.item_id = pi.id
		LEFT JOIN plan_categories pc ON pi.category_id = pc.id
		WHERE bpi.period_id = $1
		ORDER BY pc.name, pi.name
	`, periodID)
	if err != nil {
		return nil, fmt.Errorf("failed to get period items: %w", err)
	}
	defer rows.Close()

	var items []*BudgetPeriodItem
	for rows.Next() {
		var item BudgetPeriodItem
		if err := rows.Scan(
			&item.ID, &item.PeriodID, &item.ItemID,
			&item.ItemName, &item.CategoryName,
			&item.BudgetedMinor, &item.ActualMinor, &item.Notes,
			&item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan period item: %w", err)
		}
		items = append(items, &item)
	}

	return items, nil
}
