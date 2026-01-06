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

// PostgresSubscriptionRepository implements SubscriptionRepository using PostgreSQL
type PostgresSubscriptionRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresSubscriptionRepository creates a new PostgreSQL subscription repository
func NewPostgresSubscriptionRepository(pool *pgxpool.Pool) *PostgresSubscriptionRepository {
	return &PostgresSubscriptionRepository{pool: pool}
}

// Create inserts a new subscription
func (r *PostgresSubscriptionRepository) Create(ctx context.Context, sub *RecurringSubscription) error {
	query := `
		INSERT INTO recurring_subscriptions (id, user_id, merchant_name, amount_minor, currency_code, cadence, status, first_seen_at, last_seen_at, next_expected_at, occurrence_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING created_at, updated_at`

	if sub.ID == uuid.Nil {
		sub.ID = uuid.New()
	}

	err := r.pool.QueryRow(ctx, query,
		sub.ID,
		sub.UserID,
		sub.MerchantName,
		sub.AmountMinor,
		sub.CurrencyCode,
		sub.Cadence,
		sub.Status,
		sub.FirstSeenAt,
		sub.LastSeenAt,
		sub.NextExpectedAt,
		sub.OccurrenceCount,
	).Scan(&sub.CreatedAt, &sub.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create subscription: %w", err)
	}
	return nil
}

// GetByID retrieves a subscription by ID
func (r *PostgresSubscriptionRepository) GetByID(ctx context.Context, id uuid.UUID) (*RecurringSubscription, error) {
	query := `
		SELECT id, user_id, merchant_name, amount_minor, currency_code, cadence, status,
			first_seen_at, last_seen_at, next_expected_at, occurrence_count, created_at, updated_at
		FROM recurring_subscriptions
		WHERE id = $1`

	sub := &RecurringSubscription{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&sub.ID,
		&sub.UserID,
		&sub.MerchantName,
		&sub.AmountMinor,
		&sub.CurrencyCode,
		&sub.Cadence,
		&sub.Status,
		&sub.FirstSeenAt,
		&sub.LastSeenAt,
		&sub.NextExpectedAt,
		&sub.OccurrenceCount,
		&sub.CreatedAt,
		&sub.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}
	return sub, nil
}

// Update updates an existing subscription
func (r *PostgresSubscriptionRepository) Update(ctx context.Context, sub *RecurringSubscription) error {
	query := `
		UPDATE recurring_subscriptions
		SET merchant_name = $2, amount_minor = $3, cadence = $4, status = $5,
			last_seen_at = $6, next_expected_at = $7, occurrence_count = $8
		WHERE id = $1
		RETURNING updated_at`

	err := r.pool.QueryRow(ctx, query,
		sub.ID,
		sub.MerchantName,
		sub.AmountMinor,
		sub.Cadence,
		sub.Status,
		sub.LastSeenAt,
		sub.NextExpectedAt,
		sub.OccurrenceCount,
	).Scan(&sub.UpdatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return sql.ErrNoRows
	}
	if err != nil {
		return fmt.Errorf("failed to update subscription: %w", err)
	}
	return nil
}

// Delete removes a subscription
func (r *PostgresSubscriptionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM recurring_subscriptions WHERE id = $1`
	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete subscription: %w", err)
	}
	if result.RowsAffected() == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// ListByUserID retrieves all subscriptions for a user
func (r *PostgresSubscriptionRepository) ListByUserID(ctx context.Context, userID uuid.UUID, statusFilter *RecurringStatus, includeCanceled bool) ([]*RecurringSubscription, error) {
	query := `
		SELECT id, user_id, merchant_name, amount_minor, currency_code, cadence, status,
			first_seen_at, last_seen_at, next_expected_at, occurrence_count, created_at, updated_at
		FROM recurring_subscriptions
		WHERE user_id = $1`

	args := []interface{}{userID}
	argIdx := 2

	if statusFilter != nil {
		query += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, *statusFilter)
		argIdx++
	} else if !includeCanceled {
		query += ` AND status != 'canceled'`
	}

	query += ` ORDER BY amount_minor DESC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list subscriptions: %w", err)
	}
	defer rows.Close()

	var subs []*RecurringSubscription
	for rows.Next() {
		sub := &RecurringSubscription{}
		err := rows.Scan(
			&sub.ID,
			&sub.UserID,
			&sub.MerchantName,
			&sub.AmountMinor,
			&sub.CurrencyCode,
			&sub.Cadence,
			&sub.Status,
			&sub.FirstSeenAt,
			&sub.LastSeenAt,
			&sub.NextExpectedAt,
			&sub.OccurrenceCount,
			&sub.CreatedAt,
			&sub.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan subscription: %w", err)
		}
		subs = append(subs, sub)
	}
	return subs, nil
}

// GetByUserAndMerchant retrieves a subscription by user and merchant name
func (r *PostgresSubscriptionRepository) GetByUserAndMerchant(ctx context.Context, userID uuid.UUID, merchantName string) (*RecurringSubscription, error) {
	query := `
		SELECT id, user_id, merchant_name, amount_minor, currency_code, cadence, status,
			first_seen_at, last_seen_at, next_expected_at, occurrence_count, created_at, updated_at
		FROM recurring_subscriptions
		WHERE user_id = $1 AND LOWER(merchant_name) = LOWER($2)`

	sub := &RecurringSubscription{}
	err := r.pool.QueryRow(ctx, query, userID, merchantName).Scan(
		&sub.ID,
		&sub.UserID,
		&sub.MerchantName,
		&sub.AmountMinor,
		&sub.CurrencyCode,
		&sub.Cadence,
		&sub.Status,
		&sub.FirstSeenAt,
		&sub.LastSeenAt,
		&sub.NextExpectedAt,
		&sub.OccurrenceCount,
		&sub.CreatedAt,
		&sub.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}
	return sub, nil
}

// GetMerchantTransactionGroups finds recurring patterns in transactions
func (r *PostgresSubscriptionRepository) GetMerchantTransactionGroups(ctx context.Context, userID uuid.UUID, since time.Time, minOccurrences int) ([]*MerchantTransactionGroup, error) {
	query := `
		SELECT
			COALESCE(merchant_name, description) as merchant,
			COUNT(*) as tx_count,
			SUM(ABS(amount_minor)) as total_amount,
			ARRAY_AGG(posted_at ORDER BY posted_at) as dates,
			ARRAY_AGG(ABS(amount_minor) ORDER BY posted_at) as amounts,
			MAX(category_id) as category_id
		FROM transactions
		WHERE user_id = $1
			AND posted_at >= $2
			AND amount_minor < 0  -- Only expenses
			AND COALESCE(merchant_name, description) != ''
		GROUP BY COALESCE(merchant_name, description)
		HAVING COUNT(*) >= $3
		ORDER BY COUNT(*) DESC, SUM(ABS(amount_minor)) DESC
		LIMIT 100`

	rows, err := r.pool.Query(ctx, query, userID, since, minOccurrences)
	if err != nil {
		return nil, fmt.Errorf("failed to get merchant groups: %w", err)
	}
	defer rows.Close()

	var groups []*MerchantTransactionGroup
	for rows.Next() {
		group := &MerchantTransactionGroup{}
		err := rows.Scan(
			&group.MerchantName,
			new(int), // tx_count (we don't need it separately)
			&group.TotalAmount,
			&group.TransactionDates,
			&group.AmountPerTx,
			&group.CategoryID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan merchant group: %w", err)
		}
		groups = append(groups, group)
	}
	return groups, nil
}

// UpdateStatus updates the status of a subscription
func (r *PostgresSubscriptionRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status RecurringStatus) error {
	query := `UPDATE recurring_subscriptions SET status = $2 WHERE id = $1`
	result, err := r.pool.Exec(ctx, query, id, status)
	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}
	if result.RowsAffected() == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// IncrementOccurrence increments the occurrence count and updates last seen
func (r *PostgresSubscriptionRepository) IncrementOccurrence(ctx context.Context, id uuid.UUID, lastSeenAt time.Time) error {
	query := `
		UPDATE recurring_subscriptions
		SET occurrence_count = occurrence_count + 1, last_seen_at = $2
		WHERE id = $1`
	result, err := r.pool.Exec(ctx, query, id, lastSeenAt)
	if err != nil {
		return fmt.Errorf("failed to increment occurrence: %w", err)
	}
	if result.RowsAffected() == 0 {
		return sql.ErrNoRows
	}
	return nil
}
