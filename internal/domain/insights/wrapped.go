package insights

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// WrappedCard represents a single card in the wrapped summary
type WrappedCard struct {
	Title    string
	Subtitle string
	Body     string
	Accent   string // Color accent
}

// WrappedSummary contains all the wrapped cards for a period
type WrappedSummary struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Period      string // "month" or "year"
	PeriodStart time.Time
	PeriodEnd   time.Time
	Cards       []WrappedCard
	CreatedAt   time.Time
}

// GetWrapped generates a monthly/yearly wrapped summary
func (s *Service) GetWrapped(ctx context.Context, userID uuid.UUID, period string, periodStart, periodEnd time.Time) (*WrappedSummary, error) {
	summary := &WrappedSummary{
		ID:          uuid.New(),
		UserID:      userID,
		Period:      period,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		Cards:       make([]WrappedCard, 0),
		CreatedAt:   time.Now(),
	}

	// 1. Top Merchant Card
	if card, err := s.buildTopMerchantCard(ctx, userID, periodStart, periodEnd); err == nil && card != nil {
		summary.Cards = append(summary.Cards, *card)
	}

	// 2. Category Change Card
	if card, err := s.buildCategoryChangeCard(ctx, userID, periodStart, periodEnd); err == nil && card != nil {
		summary.Cards = append(summary.Cards, *card)
	}

	// 3. Net Worth Change Card
	if card, err := s.buildNetWorthCard(ctx, userID, periodStart, periodEnd); err == nil && card != nil {
		summary.Cards = append(summary.Cards, *card)
	}

	// 4. Transaction Count Card
	if card, err := s.buildTransactionCountCard(ctx, userID, periodStart, periodEnd); err == nil && card != nil {
		summary.Cards = append(summary.Cards, *card)
	}

	return summary, nil
}

// buildTopMerchantCard finds the most visited merchant
func (s *Service) buildTopMerchantCard(ctx context.Context, userID uuid.UUID, start, end time.Time) (*WrappedCard, error) {
	query := `
		SELECT COALESCE(merchant_name, description) as merchant, COUNT(*) as visits
		FROM transactions
		WHERE user_id = $1
		  AND posted_at >= $2
		  AND posted_at < $3
		  AND amount_minor < 0
		GROUP BY COALESCE(merchant_name, description)
		ORDER BY visits DESC
		LIMIT 1
	`

	var merchant string
	var visits int
	err := s.repo.DB().QueryRow(ctx, query, userID, start, end).Scan(&merchant, &visits)
	if err != nil {
		return nil, err
	}

	if merchant == "" || visits == 0 {
		return nil, nil
	}

	return &WrappedCard{
		Title:    "Top Merchant",
		Subtitle: fmt.Sprintf("You visited %d times", visits),
		Body:     merchant,
		Accent:   "#6366F1", // Echo indigo
	}, nil
}

// buildCategoryChangeCard compares spending by category to previous period
func (s *Service) buildCategoryChangeCard(ctx context.Context, userID uuid.UUID, start, end time.Time) (*WrappedCard, error) {
	// Calculate previous period
	duration := end.Sub(start)
	prevStart := start.Add(-duration)
	prevEnd := start

	query := `
		WITH current_period AS (
			SELECT c.name, ABS(SUM(t.amount_minor)) as total
			FROM transactions t
			LEFT JOIN categories c ON c.id = t.category_id
			WHERE t.user_id = $1
			  AND t.posted_at >= $2
			  AND t.posted_at < $3
			  AND t.amount_minor < 0
			GROUP BY c.name
		),
		previous_period AS (
			SELECT c.name, ABS(SUM(t.amount_minor)) as total
			FROM transactions t
			LEFT JOIN categories c ON c.id = t.category_id
			WHERE t.user_id = $1
			  AND t.posted_at >= $4
			  AND t.posted_at < $5
			  AND t.amount_minor < 0
			GROUP BY c.name
		)
		SELECT 
			COALESCE(cp.name, 'Uncategorized') as category,
			COALESCE(cp.total, 0) as current_total,
			COALESCE(pp.total, 0) as previous_total,
			COALESCE(cp.total, 0) - COALESCE(pp.total, 0) as delta
		FROM current_period cp
		FULL OUTER JOIN previous_period pp ON cp.name = pp.name
		ORDER BY ABS(COALESCE(cp.total, 0) - COALESCE(pp.total, 0)) DESC
		LIMIT 1
	`

	var category string
	var currentTotal, previousTotal, delta int64
	err := s.repo.DB().QueryRow(ctx, query, userID, start, end, prevStart, prevEnd).Scan(
		&category, &currentTotal, &previousTotal, &delta,
	)
	if err != nil {
		return nil, err
	}

	if category == "" {
		return nil, nil
	}

	// Determine direction
	direction := "more"
	accent := "#ef4444" // red
	if delta < 0 {
		direction = "less"
		accent = "#22c55e" // green
		delta = -delta
	}

	percent := 0.0
	if previousTotal > 0 {
		percent = float64(delta) / float64(previousTotal) * 100
	}

	return &WrappedCard{
		Title:    "Biggest Change",
		Subtitle: category,
		Body:     fmt.Sprintf("You spent %.0f%% %s on %s", percent, direction, category),
		Accent:   accent,
	}, nil
}

// buildNetWorthCard calculates net worth change
func (s *Service) buildNetWorthCard(ctx context.Context, userID uuid.UUID, start, end time.Time) (*WrappedCard, error) {
	query := `
		SELECT COALESCE(SUM(amount_minor), 0)
		FROM transactions
		WHERE user_id = $1
		  AND posted_at >= $2
		  AND posted_at < $3
	`

	var netChange int64
	err := s.repo.DB().QueryRow(ctx, query, userID, start, end).Scan(&netChange)
	if err != nil {
		return nil, err
	}

	direction := "increased"
	accent := "#22c55e"
	if netChange < 0 {
		direction = "decreased"
		accent = "#ef4444"
		netChange = -netChange
	}

	return &WrappedCard{
		Title:    "Net Worth",
		Subtitle: fmt.Sprintf("Your balance %s", direction),
		Body:     fmt.Sprintf("+€%.2f", float64(netChange)/100),
		Accent:   accent,
	}, nil
}

// buildTransactionCountCard counts transactions
func (s *Service) buildTransactionCountCard(ctx context.Context, userID uuid.UUID, start, end time.Time) (*WrappedCard, error) {
	query := `
		SELECT COUNT(*)
		FROM transactions
		WHERE user_id = $1
		  AND posted_at >= $2
		  AND posted_at < $3
	`

	var count int
	err := s.repo.DB().QueryRow(ctx, query, userID, start, end).Scan(&count)
	if err != nil {
		return nil, err
	}

	return &WrappedCard{
		Title:    "Activity",
		Subtitle: fmt.Sprintf("%d transactions", count),
		Body:     "You've been busy!",
		Accent:   "#6366F1",
	}, nil
}
