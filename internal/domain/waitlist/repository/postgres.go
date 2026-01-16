// Package repository provides data access for waitlist entities.
package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WaitlistStatus represents the state of a waitlist entry
type WaitlistStatus string

const (
	StatusPending WaitlistStatus = "pending"
	StatusInvited WaitlistStatus = "invited"
	StatusJoined  WaitlistStatus = "joined"
)

// WaitlistEntry represents a single waitlist signup
type WaitlistEntry struct {
	ID         uuid.UUID      `db:"id"`
	Email      string         `db:"email"`
	Status     WaitlistStatus `db:"status"`
	InviteCode *string        `db:"invite_code"`
	CreatedAt  time.Time      `db:"created_at"`
	InvitedAt  *time.Time     `db:"invited_at"`
	JoinedAt   *time.Time     `db:"joined_at"`
}

// WaitlistStats contains aggregate metrics
type WaitlistStats struct {
	TotalSignups    int
	PendingCount    int
	InvitedCount    int
	JoinedCount     int
	SignupsToday    int
	SignupsThisWeek int
}

// WaitlistRepository defines the interface for waitlist data access
type WaitlistRepository interface {
	Add(ctx context.Context, email string) (*WaitlistEntry, error)
	GetByEmail(ctx context.Context, email string) (*WaitlistEntry, error)
	GetByID(ctx context.Context, id uuid.UUID) (*WaitlistEntry, error)
	GetByInviteCode(ctx context.Context, code string) (*WaitlistEntry, error)
	List(ctx context.Context, status *WaitlistStatus, limit, offset int) ([]*WaitlistEntry, int, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status WaitlistStatus, inviteCode *string) error
	GetStats(ctx context.Context) (*WaitlistStats, error)
	GetPosition(ctx context.Context, id uuid.UUID) (int, error)
}

// PostgresWaitlistRepository implements WaitlistRepository using PostgreSQL
type PostgresWaitlistRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresWaitlistRepository creates a new PostgreSQL-backed waitlist repository
func NewPostgresWaitlistRepository(pool *pgxpool.Pool) *PostgresWaitlistRepository {
	return &PostgresWaitlistRepository{pool: pool}
}

// Add adds a new email to the waitlist
func (r *PostgresWaitlistRepository) Add(ctx context.Context, email string) (*WaitlistEntry, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	query := `
		INSERT INTO waitlist (email, status) 
		VALUES ($1, 'pending')
		ON CONFLICT (LOWER(email)) DO NOTHING
		RETURNING id, email, status, invite_code, created_at, invited_at, joined_at
	`

	var entry WaitlistEntry
	err := r.pool.QueryRow(ctx, query, email).Scan(
		&entry.ID, &entry.Email, &entry.Status, &entry.InviteCode,
		&entry.CreatedAt, &entry.InvitedAt, &entry.JoinedAt,
	)
	if err == pgx.ErrNoRows {
		// Email already exists, fetch and return it
		return r.GetByEmail(ctx, email)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to add to waitlist: %w", err)
	}
	return &entry, nil
}

// GetByEmail retrieves a waitlist entry by email
func (r *PostgresWaitlistRepository) GetByEmail(ctx context.Context, email string) (*WaitlistEntry, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	query := `
		SELECT id, email, status, invite_code, created_at, invited_at, joined_at
		FROM waitlist WHERE LOWER(email) = $1
	`

	var entry WaitlistEntry
	err := r.pool.QueryRow(ctx, query, email).Scan(
		&entry.ID, &entry.Email, &entry.Status, &entry.InviteCode,
		&entry.CreatedAt, &entry.InvitedAt, &entry.JoinedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get waitlist entry by email: %w", err)
	}
	return &entry, nil
}

// GetByID retrieves a waitlist entry by ID
func (r *PostgresWaitlistRepository) GetByID(ctx context.Context, id uuid.UUID) (*WaitlistEntry, error) {
	query := `
		SELECT id, email, status, invite_code, created_at, invited_at, joined_at
		FROM waitlist WHERE id = $1
	`

	var entry WaitlistEntry
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&entry.ID, &entry.Email, &entry.Status, &entry.InviteCode,
		&entry.CreatedAt, &entry.InvitedAt, &entry.JoinedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get waitlist entry by ID: %w", err)
	}
	return &entry, nil
}

// GetByInviteCode retrieves a waitlist entry by invite code
func (r *PostgresWaitlistRepository) GetByInviteCode(ctx context.Context, code string) (*WaitlistEntry, error) {
	query := `
		SELECT id, email, status, invite_code, created_at, invited_at, joined_at
		FROM waitlist WHERE invite_code = $1
	`

	var entry WaitlistEntry
	err := r.pool.QueryRow(ctx, query, code).Scan(
		&entry.ID, &entry.Email, &entry.Status, &entry.InviteCode,
		&entry.CreatedAt, &entry.InvitedAt, &entry.JoinedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get waitlist entry by invite code: %w", err)
	}
	return &entry, nil
}

// List returns paginated waitlist entries
func (r *PostgresWaitlistRepository) List(ctx context.Context, status *WaitlistStatus, limit, offset int) ([]*WaitlistEntry, int, error) {
	// Count query
	countQuery := `SELECT COUNT(*) FROM waitlist`
	countArgs := []any{}
	if status != nil {
		countQuery += ` WHERE status = $1`
		countArgs = append(countArgs, *status)
	}

	var total int
	if err := r.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count waitlist entries: %w", err)
	}

	// List query
	query := `
		SELECT id, email, status, invite_code, created_at, invited_at, joined_at
		FROM waitlist
	`
	args := []any{}
	argIdx := 1

	if status != nil {
		query += fmt.Sprintf(` WHERE status = $%d`, argIdx)
		args = append(args, *status)
		argIdx++
	}

	query += ` ORDER BY created_at DESC`
	query += fmt.Sprintf(` LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list waitlist entries: %w", err)
	}
	defer rows.Close()

	var entries []*WaitlistEntry
	for rows.Next() {
		var entry WaitlistEntry
		if err := rows.Scan(
			&entry.ID, &entry.Email, &entry.Status, &entry.InviteCode,
			&entry.CreatedAt, &entry.InvitedAt, &entry.JoinedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan waitlist entry: %w", err)
		}
		entries = append(entries, &entry)
	}

	return entries, total, nil
}

// UpdateStatus updates the status of a waitlist entry
func (r *PostgresWaitlistRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status WaitlistStatus, inviteCode *string) error {
	var query string
	var args []any

	switch status {
	case StatusInvited:
		query = `UPDATE waitlist SET status = $2, invite_code = $3, invited_at = NOW() WHERE id = $1`
		args = []any{id, status, inviteCode}
	case StatusJoined:
		query = `UPDATE waitlist SET status = $2, joined_at = NOW() WHERE id = $1`
		args = []any{id, status}
	default:
		query = `UPDATE waitlist SET status = $2 WHERE id = $1`
		args = []any{id, status}
	}

	_, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update waitlist status: %w", err)
	}
	return nil
}

// GetStats returns aggregate waitlist metrics
func (r *PostgresWaitlistRepository) GetStats(ctx context.Context) (*WaitlistStats, error) {
	query := `
		SELECT 
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE status = 'pending') as pending,
			COUNT(*) FILTER (WHERE status = 'invited') as invited,
			COUNT(*) FILTER (WHERE status = 'joined') as joined,
			COUNT(*) FILTER (WHERE created_at >= CURRENT_DATE) as today,
			COUNT(*) FILTER (WHERE created_at >= CURRENT_DATE - INTERVAL '7 days') as this_week
		FROM waitlist
	`

	var stats WaitlistStats
	err := r.pool.QueryRow(ctx, query).Scan(
		&stats.TotalSignups, &stats.PendingCount, &stats.InvitedCount,
		&stats.JoinedCount, &stats.SignupsToday, &stats.SignupsThisWeek,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get waitlist stats: %w", err)
	}
	return &stats, nil
}

// GetPosition returns the queue position for a waitlist entry
func (r *PostgresWaitlistRepository) GetPosition(ctx context.Context, id uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*) + 1 
		FROM waitlist 
		WHERE created_at < (SELECT created_at FROM waitlist WHERE id = $1)
		AND status = 'pending'
	`

	var position int
	err := r.pool.QueryRow(ctx, query, id).Scan(&position)
	if err != nil {
		return 0, fmt.Errorf("failed to get waitlist position: %w", err)
	}
	return position, nil
}
