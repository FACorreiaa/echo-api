// Package service provides business logic for ML corrections.
package service

import (
	"context"
	"log/slog"

	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/plan/excel"
	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/plan/repository"
	"github.com/google/uuid"
)

// MLService manages ML model hydration and corrections.
// It bridges the MLPredictor (in-memory) with the MLCorrectionRepository (database).
type MLService struct {
	predictor *excel.MLPredictor
	corrRepo  *repository.MLCorrectionRepository
	logger    *slog.Logger
}

// NewMLService creates a new ML service instance.
// The predictor is the singleton from excel.GetMLPredictor().
func NewMLService(corrRepo *repository.MLCorrectionRepository, logger *slog.Logger) *MLService {
	return &MLService{
		predictor: excel.GetMLPredictor(),
		corrRepo:  corrRepo,
		logger:    logger,
	}
}

// HydrateForUser loads user-specific ML corrections from the database
// and teaches them to the in-memory predictor.
// This should be called before analyzing Excel files for a specific user
// to personalize the predictions based on their correction history.
func (s *MLService) HydrateForUser(ctx context.Context, userID uuid.UUID) error {
	s.logger.Info("hydrating ML predictor for user",
		"userID", userID.String(),
	)

	// Fetch all corrections for this user from the database
	corrections, err := s.corrRepo.GetUserCorrections(ctx, userID)
	if err != nil {
		s.logger.Error("failed to load user ML corrections",
			"userID", userID.String(),
			"error", err,
		)
		return err
	}

	if len(corrections) == 0 {
		s.logger.Debug("no ML corrections found for user",
			"userID", userID.String(),
		)
		return nil
	}

	// Convert repository corrections to excel.UserMLCorrection format
	excelCorrections := make([]excel.UserMLCorrection, len(corrections))
	for i, c := range corrections {
		excelCorrections[i] = excel.UserMLCorrection{
			Term:         c.Term,
			PredictedTag: excel.ItemTag(c.PredictedTag),
			CorrectedTag: excel.ItemTag(c.CorrectedTag),
		}
	}

	// Teach the predictor these corrections
	s.predictor.LearnBatch(excelCorrections)

	s.logger.Info("hydrated ML predictor with user corrections",
		"userID", userID.String(),
		"correctionCount", len(excelCorrections),
	)

	return nil
}

// SaveCorrection persists a user's ML correction to the database
// and immediately teaches it to the in-memory predictor.
func (s *MLService) SaveCorrection(
	ctx context.Context,
	userID uuid.UUID,
	term string,
	predictedTag string,
	correctedTag string,
	sourceFileID *uuid.UUID,
) error {
	// Save to database
	err := s.corrRepo.SaveCorrection(
		ctx,
		userID,
		term,
		predictedTag,
		correctedTag,
		"excel_import", // model type
		sourceFileID,
	)
	if err != nil {
		s.logger.Error("failed to save ML correction",
			"userID", userID.String(),
			"term", term,
			"error", err,
		)
		return err
	}

	// Immediately teach this correction to the in-memory predictor
	s.predictor.Learn(term, excel.ItemTag(correctedTag))

	s.logger.Info("saved and learned ML correction",
		"userID", userID.String(),
		"term", term,
		"from", predictedTag,
		"to", correctedTag,
	)

	return nil
}

// GetPredictor returns the underlying ML predictor for direct use.
func (s *MLService) GetPredictor() *excel.MLPredictor {
	return s.predictor
}
