// Package normalizer provides the override store for user merchant corrections.
package normalizer

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MerchantOverride represents a user's correction for a merchant
type MerchantOverride struct {
	ID            uuid.UUID  `json:"id"`
	UserID        uuid.UUID  `json:"user_id"`
	MatchPattern  string     `json:"match_pattern"`
	MatchType     string     `json:"match_type"` // "exact", "contains", "regex"
	MerchantName  string     `json:"merchant_name"`
	Category      *string    `json:"category,omitempty"`
	Subcategory   *string    `json:"subcategory,omitempty"`
	MatchCount    int        `json:"match_count"`
	LastMatchedAt *time.Time `json:"last_matched_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// OverrideStore manages user merchant overrides in the database
type OverrideStore struct {
	db *pgxpool.Pool
}

// NewOverrideStore creates a new override store
func NewOverrideStore(db *pgxpool.Pool) *OverrideStore {
	return &OverrideStore{db: db}
}

// SaveOverride creates or updates a user's merchant override
func (s *OverrideStore) SaveOverride(ctx context.Context, override MerchantOverride) (*MerchantOverride, error) {
	query := `
		INSERT INTO user_merchant_overrides (
			user_id, match_pattern, match_type, merchant_name, category, subcategory
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, match_pattern) DO UPDATE SET
			merchant_name = EXCLUDED.merchant_name,
			category = EXCLUDED.category,
			subcategory = EXCLUDED.subcategory,
			updated_at = now()
		RETURNING id, user_id, match_pattern, match_type, merchant_name, category, 
			subcategory, match_count, last_matched_at, created_at, updated_at
	`

	var result MerchantOverride
	err := s.db.QueryRow(ctx, query,
		override.UserID,
		override.MatchPattern,
		override.MatchType,
		override.MerchantName,
		override.Category,
		override.Subcategory,
	).Scan(
		&result.ID, &result.UserID, &result.MatchPattern, &result.MatchType,
		&result.MerchantName, &result.Category, &result.Subcategory,
		&result.MatchCount, &result.LastMatchedAt, &result.CreatedAt, &result.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// GetOverridesForUser returns all overrides for a user
func (s *OverrideStore) GetOverridesForUser(ctx context.Context, userID uuid.UUID) ([]MerchantOverride, error) {
	query := `
		SELECT id, user_id, match_pattern, match_type, merchant_name, category, 
			subcategory, match_count, last_matched_at, created_at, updated_at
		FROM user_merchant_overrides
		WHERE user_id = $1
		ORDER BY match_count DESC, updated_at DESC
	`

	rows, err := s.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var overrides []MerchantOverride
	for rows.Next() {
		var o MerchantOverride
		err := rows.Scan(
			&o.ID, &o.UserID, &o.MatchPattern, &o.MatchType,
			&o.MerchantName, &o.Category, &o.Subcategory,
			&o.MatchCount, &o.LastMatchedAt, &o.CreatedAt, &o.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		overrides = append(overrides, o)
	}
	return overrides, rows.Err()
}

// FindMatchingOverride finds the first override that matches the raw merchant
func (s *OverrideStore) FindMatchingOverride(ctx context.Context, userID uuid.UUID, rawMerchant string) (*MerchantOverride, error) {
	overrides, err := s.GetOverridesForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	upperRaw := strings.ToUpper(rawMerchant)

	for i := range overrides {
		o := &overrides[i]
		matched := false

		switch o.MatchType {
		case "exact":
			matched = strings.EqualFold(rawMerchant, o.MatchPattern)
		case "contains":
			matched = strings.Contains(upperRaw, strings.ToUpper(o.MatchPattern))
		case "regex":
			// For regex, we'd need to compile - skip for now to avoid errors
			matched = strings.Contains(upperRaw, strings.ToUpper(o.MatchPattern))
		}

		if matched {
			// Update match count asynchronously
			go s.incrementMatchCount(context.Background(), o.ID)
			return o, nil
		}
	}

	return nil, nil
}

// incrementMatchCount updates the match count for an override
func (s *OverrideStore) incrementMatchCount(ctx context.Context, id uuid.UUID) {
	query := `
		UPDATE user_merchant_overrides 
		SET match_count = match_count + 1, last_matched_at = now()
		WHERE id = $1
	`
	_, _ = s.db.Exec(ctx, query, id)
}

// DeleteOverride removes an override
func (s *OverrideStore) DeleteOverride(ctx context.Context, userID, overrideID uuid.UUID) error {
	query := `DELETE FROM user_merchant_overrides WHERE id = $1 AND user_id = $2`
	result, err := s.db.Exec(ctx, query, overrideID, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
