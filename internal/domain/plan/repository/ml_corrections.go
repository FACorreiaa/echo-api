// Package repository provides data access for ML corrections.
package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MLCorrection represents a user's ML correction stored in the database
type MLCorrection struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	Term         string
	PredictedTag string
	CorrectedTag string
	ModelType    string
	SourceFileID *uuid.UUID
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// MLCorrectionRepository handles ML correction persistence
type MLCorrectionRepository struct {
	pool *pgxpool.Pool
}

// NewMLCorrectionRepository creates a new repository instance
func NewMLCorrectionRepository(pool *pgxpool.Pool) *MLCorrectionRepository {
	return &MLCorrectionRepository{pool: pool}
}

// SaveCorrection saves or updates a user's ML correction
func (r *MLCorrectionRepository) SaveCorrection(ctx context.Context, userID uuid.UUID, term, predictedTag, correctedTag, modelType string, sourceFileID *uuid.UUID) error {
	query := `
		INSERT INTO user_ml_corrections (user_id, term, predicted_tag, corrected_tag, model_type, source_file_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, term, model_type) 
		DO UPDATE SET 
			predicted_tag = EXCLUDED.predicted_tag,
			corrected_tag = EXCLUDED.corrected_tag,
			source_file_id = EXCLUDED.source_file_id,
			updated_at = NOW()
	`

	_, err := r.pool.Exec(ctx, query, userID, term, predictedTag, correctedTag, modelType, sourceFileID)
	return err
}

// GetUserCorrections retrieves all corrections for a user
func (r *MLCorrectionRepository) GetUserCorrections(ctx context.Context, userID uuid.UUID) ([]MLCorrection, error) {
	query := `
		SELECT id, user_id, term, predicted_tag, corrected_tag, model_type, source_file_id, created_at, updated_at
		FROM user_ml_corrections
		WHERE user_id = $1
		ORDER BY updated_at DESC
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var corrections []MLCorrection
	for rows.Next() {
		var c MLCorrection
		var predictedTag, sourceFileID *string

		err := rows.Scan(
			&c.ID,
			&c.UserID,
			&c.Term,
			&predictedTag,
			&c.CorrectedTag,
			&c.ModelType,
			&sourceFileID,
			&c.CreatedAt,
			&c.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if predictedTag != nil {
			c.PredictedTag = *predictedTag
		}
		if sourceFileID != nil {
			if id, err := uuid.Parse(*sourceFileID); err == nil {
				c.SourceFileID = &id
			}
		}

		corrections = append(corrections, c)
	}

	return corrections, rows.Err()
}

// GetCorrectionsByModelType retrieves corrections for a specific model type
func (r *MLCorrectionRepository) GetCorrectionsByModelType(ctx context.Context, userID uuid.UUID, modelType string) ([]MLCorrection, error) {
	query := `
		SELECT id, user_id, term, predicted_tag, corrected_tag, model_type, source_file_id, created_at, updated_at
		FROM user_ml_corrections
		WHERE user_id = $1 AND model_type = $2
		ORDER BY updated_at DESC
	`

	rows, err := r.pool.Query(ctx, query, userID, modelType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var corrections []MLCorrection
	for rows.Next() {
		var c MLCorrection
		var predictedTag, sourceFileID *string

		err := rows.Scan(
			&c.ID,
			&c.UserID,
			&c.Term,
			&predictedTag,
			&c.CorrectedTag,
			&c.ModelType,
			&sourceFileID,
			&c.CreatedAt,
			&c.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if predictedTag != nil {
			c.PredictedTag = *predictedTag
		}
		if sourceFileID != nil {
			if id, err := uuid.Parse(*sourceFileID); err == nil {
				c.SourceFileID = &id
			}
		}

		corrections = append(corrections, c)
	}

	return corrections, rows.Err()
}

// DeleteCorrection removes a specific correction
func (r *MLCorrectionRepository) DeleteCorrection(ctx context.Context, userID uuid.UUID, term, modelType string) error {
	query := `DELETE FROM user_ml_corrections WHERE user_id = $1 AND term = $2 AND model_type = $3`
	_, err := r.pool.Exec(ctx, query, userID, term, modelType)
	return err
}

// GetMostCorrectedTerms returns analytics on most frequently corrected terms (global)
func (r *MLCorrectionRepository) GetMostCorrectedTerms(ctx context.Context, limit int) ([]struct {
	Term  string
	Count int
}, error) {
	query := `
		SELECT term, COUNT(*) as correction_count
		FROM user_ml_corrections
		GROUP BY term
		ORDER BY correction_count DESC
		LIMIT $1
	`

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []struct {
		Term  string
		Count int
	}
	for rows.Next() {
		var r struct {
			Term  string
			Count int
		}
		if err := rows.Scan(&r.Term, &r.Count); err != nil {
			return nil, err
		}
		results = append(results, r)
	}

	return results, rows.Err()
}
