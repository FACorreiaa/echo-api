package categorization

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CategoryRule represents a user-defined categorization rule
type CategoryRule struct {
	ID                 uuid.UUID
	UserID             uuid.UUID
	MatchPattern       string
	CleanName          *string
	AssignedCategoryID *uuid.UUID
	IsRecurring        bool
	Priority           int
}

// Merchant represents a normalized merchant entry
type Merchant struct {
	ID                uuid.UUID
	UserID            *uuid.UUID // nil = system/global
	RawPattern        string
	CleanName         string
	LogoURL           *string
	DefaultCategoryID *uuid.UUID
	IsSystem          bool
}

// CategorizationResult holds the result of categorizing a transaction
type CategorizationResult struct {
	CleanMerchantName string
	CategoryID        *uuid.UUID
	IsRecurring       bool
	RuleID            *uuid.UUID // Which rule matched, if any
	MerchantID        *uuid.UUID // Which merchant matched, if any
}

// Repository handles database operations for categorization
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new categorization repository
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// GetUserRules fetches all categorization rules for a user, ordered by priority
func (r *Repository) GetUserRules(ctx context.Context, userID uuid.UUID) ([]CategoryRule, error) {
	query := `
		SELECT id, user_id, match_pattern, clean_name, assigned_category_id, is_recurring, priority
		FROM category_rules
		WHERE user_id = $1
		ORDER BY priority DESC, created_at DESC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []CategoryRule
	for rows.Next() {
		var rule CategoryRule
		if err := rows.Scan(
			&rule.ID,
			&rule.UserID,
			&rule.MatchPattern,
			&rule.CleanName,
			&rule.AssignedCategoryID,
			&rule.IsRecurring,
			&rule.Priority,
		); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}

	return rules, rows.Err()
}

// GetMerchants fetches merchants (user-specific + system defaults)
func (r *Repository) GetMerchants(ctx context.Context, userID *uuid.UUID) ([]Merchant, error) {
	query := `
		SELECT id, user_id, raw_pattern, clean_name, logo_url, default_category_id, is_system
		FROM merchants
		WHERE user_id = $1 OR user_id IS NULL OR is_system = true
		ORDER BY 
			CASE WHEN user_id = $1 THEN 0 ELSE 1 END,
			is_system ASC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var merchants []Merchant
	for rows.Next() {
		var m Merchant
		if err := rows.Scan(
			&m.ID,
			&m.UserID,
			&m.RawPattern,
			&m.CleanName,
			&m.LogoURL,
			&m.DefaultCategoryID,
			&m.IsSystem,
		); err != nil {
			return nil, err
		}
		merchants = append(merchants, m)
	}

	return merchants, rows.Err()
}

// CreateRule creates a new categorization rule
func (r *Repository) CreateRule(ctx context.Context, rule *CategoryRule) error {
	query := `
		INSERT INTO category_rules (user_id, match_pattern, clean_name, assigned_category_id, is_recurring, priority)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`

	return r.db.QueryRow(ctx, query,
		rule.UserID,
		rule.MatchPattern,
		rule.CleanName,
		rule.AssignedCategoryID,
		rule.IsRecurring,
		rule.Priority,
	).Scan(&rule.ID)
}

// UpdateTransactionsMerchant updates merchant_name for matching transactions
func (r *Repository) UpdateTransactionsMerchant(ctx context.Context, userID uuid.UUID, pattern, cleanName string, categoryID *uuid.UUID) (int64, error) {
	query := `
		UPDATE transactions
		SET merchant_name = $3, category_id = $4
		WHERE user_id = $1 AND description ILIKE $2
	`

	result, err := r.db.Exec(ctx, query, userID, pattern, cleanName, categoryID)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected(), nil
}

// FindRuleByPattern checks if a rule already exists for this pattern
func (r *Repository) FindRuleByPattern(ctx context.Context, userID uuid.UUID, pattern string) (*CategoryRule, error) {
	query := `
		SELECT id, user_id, match_pattern, clean_name, assigned_category_id, is_recurring, priority
		FROM category_rules
		WHERE user_id = $1 AND match_pattern = $2
	`

	var rule CategoryRule
	err := r.db.QueryRow(ctx, query, userID, pattern).Scan(
		&rule.ID,
		&rule.UserID,
		&rule.MatchPattern,
		&rule.CleanName,
		&rule.AssignedCategoryID,
		&rule.IsRecurring,
		&rule.Priority,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &rule, nil
}

// matchPattern checks if a description matches a SQL LIKE pattern
func matchPattern(description, pattern string) bool {
	// Convert SQL LIKE pattern to simple contains check
	// Pattern like '%APPLE%' becomes case-insensitive contains check
	cleanPattern := strings.Trim(pattern, "%")
	return strings.Contains(strings.ToUpper(description), strings.ToUpper(cleanPattern))
}
