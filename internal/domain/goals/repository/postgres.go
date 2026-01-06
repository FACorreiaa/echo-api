package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresGoalRepository implements GoalRepository using PostgreSQL
type PostgresGoalRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresGoalRepository creates a new PostgreSQL goal repository
func NewPostgresGoalRepository(pool *pgxpool.Pool) *PostgresGoalRepository {
	return &PostgresGoalRepository{pool: pool}
}

// Create inserts a new goal
func (r *PostgresGoalRepository) Create(ctx context.Context, goal *Goal) error {
	query := `
		INSERT INTO goals (id, user_id, name, type, status, target_amount_minor, currency_code, current_amount_minor, start_at, end_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING created_at, updated_at`

	if goal.ID == uuid.Nil {
		goal.ID = uuid.New()
	}

	err := r.pool.QueryRow(ctx, query,
		goal.ID,
		goal.UserID,
		goal.Name,
		goal.Type,
		goal.Status,
		goal.TargetAmountMinor,
		goal.CurrencyCode,
		goal.CurrentAmountMinor,
		goal.StartAt,
		goal.EndAt,
	).Scan(&goal.CreatedAt, &goal.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create goal: %w", err)
	}
	return nil
}

// GetByID retrieves a goal by ID
func (r *PostgresGoalRepository) GetByID(ctx context.Context, id uuid.UUID) (*Goal, error) {
	query := `
		SELECT id, user_id, name, type, status, target_amount_minor, currency_code, current_amount_minor, start_at, end_at, created_at, updated_at
		FROM goals
		WHERE id = $1`

	goal := &Goal{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&goal.ID,
		&goal.UserID,
		&goal.Name,
		&goal.Type,
		&goal.Status,
		&goal.TargetAmountMinor,
		&goal.CurrencyCode,
		&goal.CurrentAmountMinor,
		&goal.StartAt,
		&goal.EndAt,
		&goal.CreatedAt,
		&goal.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get goal: %w", err)
	}
	return goal, nil
}

// Update updates an existing goal
func (r *PostgresGoalRepository) Update(ctx context.Context, goal *Goal) error {
	query := `
		UPDATE goals
		SET name = $2, type = $3, status = $4, target_amount_minor = $5, current_amount_minor = $6, end_at = $7
		WHERE id = $1
		RETURNING updated_at`

	err := r.pool.QueryRow(ctx, query,
		goal.ID,
		goal.Name,
		goal.Type,
		goal.Status,
		goal.TargetAmountMinor,
		goal.CurrentAmountMinor,
		goal.EndAt,
	).Scan(&goal.UpdatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return sql.ErrNoRows
	}
	if err != nil {
		return fmt.Errorf("failed to update goal: %w", err)
	}
	return nil
}

// Delete removes a goal
func (r *PostgresGoalRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM goals WHERE id = $1`
	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete goal: %w", err)
	}
	if result.RowsAffected() == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// ListByUserID retrieves all goals for a user
func (r *PostgresGoalRepository) ListByUserID(ctx context.Context, userID uuid.UUID, statusFilter *GoalStatus) ([]*Goal, error) {
	query := `
		SELECT id, user_id, name, type, status, target_amount_minor, currency_code, current_amount_minor, start_at, end_at, created_at, updated_at
		FROM goals
		WHERE user_id = $1`

	args := []interface{}{userID}
	if statusFilter != nil {
		query += ` AND status = $2`
		args = append(args, *statusFilter)
	}
	query += ` ORDER BY end_at ASC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list goals: %w", err)
	}
	defer rows.Close()

	var goals []*Goal
	for rows.Next() {
		goal := &Goal{}
		err := rows.Scan(
			&goal.ID,
			&goal.UserID,
			&goal.Name,
			&goal.Type,
			&goal.Status,
			&goal.TargetAmountMinor,
			&goal.CurrencyCode,
			&goal.CurrentAmountMinor,
			&goal.StartAt,
			&goal.EndAt,
			&goal.CreatedAt,
			&goal.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan goal: %w", err)
		}
		goals = append(goals, goal)
	}
	return goals, nil
}

// AddContribution adds a contribution to a goal
func (r *PostgresGoalRepository) AddContribution(ctx context.Context, contribution *GoalContribution) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Insert contribution
	if contribution.ID == uuid.Nil {
		contribution.ID = uuid.New()
	}
	if contribution.ContributedAt.IsZero() {
		contribution.ContributedAt = time.Now()
	}

	insertQuery := `
		INSERT INTO goal_contributions (id, goal_id, amount_minor, currency_code, note, transaction_id, contributed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at`

	err = tx.QueryRow(ctx, insertQuery,
		contribution.ID,
		contribution.GoalID,
		contribution.AmountMinor,
		contribution.CurrencyCode,
		contribution.Note,
		contribution.TransactionID,
		contribution.ContributedAt,
	).Scan(&contribution.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert contribution: %w", err)
	}

	// Update goal's current amount
	updateQuery := `
		UPDATE goals
		SET current_amount_minor = current_amount_minor + $2
		WHERE id = $1`
	_, err = tx.Exec(ctx, updateQuery, contribution.GoalID, contribution.AmountMinor)
	if err != nil {
		return fmt.Errorf("failed to update goal amount: %w", err)
	}

	return tx.Commit(ctx)
}

// ListContributions retrieves recent contributions for a goal
func (r *PostgresGoalRepository) ListContributions(ctx context.Context, goalID uuid.UUID, limit int) ([]*GoalContribution, error) {
	query := `
		SELECT id, goal_id, amount_minor, currency_code, note, transaction_id, contributed_at, created_at
		FROM goal_contributions
		WHERE goal_id = $1
		ORDER BY contributed_at DESC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, query, goalID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list contributions: %w", err)
	}
	defer rows.Close()

	var contributions []*GoalContribution
	for rows.Next() {
		c := &GoalContribution{}
		err := rows.Scan(
			&c.ID,
			&c.GoalID,
			&c.AmountMinor,
			&c.CurrencyCode,
			&c.Note,
			&c.TransactionID,
			&c.ContributedAt,
			&c.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan contribution: %w", err)
		}
		contributions = append(contributions, c)
	}
	return contributions, nil
}

// UpdateCurrentAmount directly sets the current amount for a goal
func (r *PostgresGoalRepository) UpdateCurrentAmount(ctx context.Context, goalID uuid.UUID, amountMinor int64) error {
	query := `UPDATE goals SET current_amount_minor = $2 WHERE id = $1`
	result, err := r.pool.Exec(ctx, query, goalID, amountMinor)
	if err != nil {
		return fmt.Errorf("failed to update current amount: %w", err)
	}
	if result.RowsAffected() == 0 {
		return sql.ErrNoRows
	}
	return nil
}
