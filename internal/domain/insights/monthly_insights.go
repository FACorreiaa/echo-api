package insights

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
)

// InsightChange represents a significant change detected this month
type InsightChange struct {
	Type           InsightChangeType
	Title          string
	Description    string
	AmountChange   int64   // The delta amount in cents
	PercentChange  float64 // Percentage change
	CategoryID     *uuid.UUID
	CategoryName   *string
	MerchantName   *string
	Icon           string
	Sentiment      InsightChangeSentiment
}

// InsightChangeType defines the type of change
type InsightChangeType string

const (
	InsightChangeTypeCategoryIncrease     InsightChangeType = "category_increase"
	InsightChangeTypeCategoryDecrease     InsightChangeType = "category_decrease"
	InsightChangeTypeNewMerchant          InsightChangeType = "new_merchant"
	InsightChangeTypeMerchantIncrease     InsightChangeType = "merchant_increase"
	InsightChangeTypeSubscriptionDetected InsightChangeType = "subscription_detected"
	InsightChangeTypeGoalProgress         InsightChangeType = "goal_progress"
	InsightChangeTypeIncomeChange         InsightChangeType = "income_change"
	InsightChangeTypeSavingsRate          InsightChangeType = "savings_rate"
)

// InsightChangeSentiment indicates whether the change is good or bad
type InsightChangeSentiment string

const (
	InsightChangeSentimentPositive InsightChangeSentiment = "positive"
	InsightChangeSentimentNegative InsightChangeSentiment = "negative"
	InsightChangeSentimentNeutral  InsightChangeSentiment = "neutral"
)

// ActionRecommendation represents a recommended action
type ActionRecommendation struct {
	Type            ActionType
	Title           string
	Description     string
	CTAText         string
	CTAAction       string
	PotentialImpact int64 // In cents
	Priority        ActionPriority
	Icon            string
}

// ActionType defines the type of recommended action
type ActionType string

const (
	ActionTypeReviewSubscriptions    ActionType = "review_subscriptions"
	ActionTypeReduceCategory         ActionType = "reduce_category"
	ActionTypeContributeToGoal       ActionType = "contribute_to_goal"
	ActionTypeCategorizeTransactions ActionType = "categorize_transactions"
	ActionTypeSetBudget              ActionType = "set_budget"
	ActionTypeReviewLargeExpense     ActionType = "review_large_expense"
)

// ActionPriority defines the priority of an action
type ActionPriority string

const (
	ActionPriorityLow    ActionPriority = "low"
	ActionPriorityMedium ActionPriority = "medium"
	ActionPriorityHigh   ActionPriority = "high"
)

// MonthlyInsights contains the full monthly insights report
type MonthlyInsights struct {
	ID                 uuid.UUID
	UserID             uuid.UUID
	MonthStart         time.Time
	TotalSpend         int64
	TotalIncome        int64
	Net                int64
	TopCategories      []TopCategory
	TopMerchants       []MerchantSpend
	Highlights         []string
	Changes            []InsightChange
	RecommendedAction  *ActionRecommendation
	SpendVsLastMonth   int64
	SpendChangePercent float64
	CreatedAt          time.Time
}

// MerchantSpend represents spending at a merchant
type MerchantSpend struct {
	MerchantName string
	AmountCents  int64
	TxCount      int
}

// GetMonthlyInsights generates monthly insights with "3 things changed" and "1 action"
func (s *Service) GetMonthlyInsights(ctx context.Context, userID uuid.UUID, monthStart time.Time) (*MonthlyInsights, error) {
	// Normalize to first of month
	year, month, _ := monthStart.Date()
	monthStart = time.Date(year, month, 1, 0, 0, 0, 0, monthStart.Location())
	monthEnd := monthStart.AddDate(0, 1, 0)
	lastMonthStart := monthStart.AddDate(0, -1, 0)
	lastMonthEnd := monthStart

	insights := &MonthlyInsights{
		ID:         uuid.New(),
		UserID:     userID,
		MonthStart: monthStart,
		CreatedAt:  time.Now(),
	}

	// Get current month totals
	currentSpend, currentIncome, err := s.getMonthTotals(ctx, userID, monthStart, monthEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to get current month totals: %w", err)
	}
	insights.TotalSpend = currentSpend
	insights.TotalIncome = currentIncome
	insights.Net = currentIncome - currentSpend

	// Get last month totals for comparison
	lastSpend, _, err := s.getMonthTotals(ctx, userID, lastMonthStart, lastMonthEnd)
	if err != nil {
		lastSpend = 0
	}
	insights.SpendVsLastMonth = currentSpend - lastSpend
	if lastSpend > 0 {
		insights.SpendChangePercent = float64(currentSpend-lastSpend) / float64(lastSpend) * 100
	}

	// Get top categories
	categories, err := s.repo.GetTopCategories(ctx, userID, monthEnd.AddDate(0, 0, -1), 5)
	if err == nil {
		insights.TopCategories = categories
	}

	// Get top merchants
	merchants, err := s.getTopMerchants(ctx, userID, monthStart, monthEnd, 5)
	if err == nil {
		insights.TopMerchants = merchants
	}

	// Generate "3 things that changed"
	insights.Changes = s.detectChanges(ctx, userID, monthStart, monthEnd, lastMonthStart, lastMonthEnd)

	// Generate "1 action to take"
	insights.RecommendedAction = s.generateRecommendation(ctx, userID, insights)

	// Generate highlights
	insights.Highlights = s.generateHighlights(insights)

	return insights, nil
}

// getMonthTotals returns total spending and income for a month
func (s *Service) getMonthTotals(ctx context.Context, userID uuid.UUID, start, end time.Time) (spend, income int64, err error) {
	query := `
		SELECT
			COALESCE(SUM(CASE WHEN amount_minor < 0 THEN ABS(amount_minor) ELSE 0 END), 0) as spend,
			COALESCE(SUM(CASE WHEN amount_minor > 0 THEN amount_minor ELSE 0 END), 0) as income
		FROM transactions
		WHERE user_id = $1 AND posted_at >= $2 AND posted_at < $3
	`
	err = s.repo.DB().QueryRow(ctx, query, userID, start, end).Scan(&spend, &income)
	return
}

// getTopMerchants returns top merchants by spend for a period
func (s *Service) getTopMerchants(ctx context.Context, userID uuid.UUID, start, end time.Time, limit int) ([]MerchantSpend, error) {
	query := `
		SELECT COALESCE(merchant_name, description) as merchant,
			   SUM(ABS(amount_minor)) as total,
			   COUNT(*) as tx_count
		FROM transactions
		WHERE user_id = $1 AND posted_at >= $2 AND posted_at < $3 AND amount_minor < 0
		GROUP BY COALESCE(merchant_name, description)
		ORDER BY total DESC
		LIMIT $4
	`
	rows, err := s.repo.DB().Query(ctx, query, userID, start, end, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var merchants []MerchantSpend
	for rows.Next() {
		var m MerchantSpend
		if err := rows.Scan(&m.MerchantName, &m.AmountCents, &m.TxCount); err != nil {
			continue
		}
		merchants = append(merchants, m)
	}
	return merchants, nil
}

// detectChanges identifies the top 3 significant changes this month
func (s *Service) detectChanges(ctx context.Context, userID uuid.UUID, currentStart, currentEnd, lastStart, lastEnd time.Time) []InsightChange {
	var allChanges []InsightChange

	// 1. Detect category changes
	categoryChanges := s.detectCategoryChanges(ctx, userID, currentStart, currentEnd, lastStart, lastEnd)
	allChanges = append(allChanges, categoryChanges...)

	// 2. Detect new merchants
	newMerchants := s.detectNewMerchants(ctx, userID, currentStart, currentEnd, lastStart, lastEnd)
	allChanges = append(allChanges, newMerchants...)

	// 3. Detect income changes
	incomeChange := s.detectIncomeChange(ctx, userID, currentStart, currentEnd, lastStart, lastEnd)
	if incomeChange != nil {
		allChanges = append(allChanges, *incomeChange)
	}

	// Sort by absolute impact and take top 3
	sort.Slice(allChanges, func(i, j int) bool {
		return math.Abs(float64(allChanges[i].AmountChange)) > math.Abs(float64(allChanges[j].AmountChange))
	})

	if len(allChanges) > 3 {
		allChanges = allChanges[:3]
	}

	return allChanges
}

// detectCategoryChanges finds categories with significant spending changes
func (s *Service) detectCategoryChanges(ctx context.Context, userID uuid.UUID, currentStart, currentEnd, lastStart, lastEnd time.Time) []InsightChange {
	query := `
		WITH current_month AS (
			SELECT category_id, COALESCE(c.name, 'Uncategorized') as cat_name, SUM(ABS(amount_minor)) as total
			FROM transactions t
			LEFT JOIN categories c ON t.category_id = c.id
			WHERE t.user_id = $1 AND posted_at >= $2 AND posted_at < $3 AND amount_minor < 0
			GROUP BY category_id, c.name
		),
		last_month AS (
			SELECT category_id, SUM(ABS(amount_minor)) as total
			FROM transactions
			WHERE user_id = $1 AND posted_at >= $4 AND posted_at < $5 AND amount_minor < 0
			GROUP BY category_id
		)
		SELECT cm.category_id, cm.cat_name, cm.total as current_total, COALESCE(lm.total, 0) as last_total
		FROM current_month cm
		LEFT JOIN last_month lm ON cm.category_id = lm.category_id OR (cm.category_id IS NULL AND lm.category_id IS NULL)
		WHERE ABS(cm.total - COALESCE(lm.total, 0)) > 1000  -- Minimum €10 change
		ORDER BY ABS(cm.total - COALESCE(lm.total, 0)) DESC
		LIMIT 5
	`

	rows, err := s.repo.DB().Query(ctx, query, userID, currentStart, currentEnd, lastStart, lastEnd)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var changes []InsightChange
	for rows.Next() {
		var catID *uuid.UUID
		var catName string
		var currentTotal, lastTotal int64

		if err := rows.Scan(&catID, &catName, &currentTotal, &lastTotal); err != nil {
			continue
		}

		delta := currentTotal - lastTotal
		var pctChange float64
		if lastTotal > 0 {
			pctChange = float64(delta) / float64(lastTotal) * 100
		}

		change := InsightChange{
			AmountChange:  delta,
			PercentChange: pctChange,
			CategoryID:    catID,
			CategoryName:  &catName,
		}

		if delta > 0 {
			change.Type = InsightChangeTypeCategoryIncrease
			change.Title = fmt.Sprintf("%s increased", catName)
			change.Description = fmt.Sprintf("You spent €%.2f more on %s than last month", float64(delta)/100, catName)
			change.Icon = "trending-up"
			change.Sentiment = InsightChangeSentimentNegative
		} else {
			change.Type = InsightChangeTypeCategoryDecrease
			change.Title = fmt.Sprintf("%s decreased", catName)
			change.Description = fmt.Sprintf("You spent €%.2f less on %s than last month", float64(-delta)/100, catName)
			change.Icon = "trending-down"
			change.Sentiment = InsightChangeSentimentPositive
		}

		changes = append(changes, change)
	}

	return changes
}

// detectNewMerchants finds new merchants not seen last month
func (s *Service) detectNewMerchants(ctx context.Context, userID uuid.UUID, currentStart, currentEnd, lastStart, lastEnd time.Time) []InsightChange {
	query := `
		WITH current_merchants AS (
			SELECT COALESCE(merchant_name, description) as merchant, SUM(ABS(amount_minor)) as total
			FROM transactions
			WHERE user_id = $1 AND posted_at >= $2 AND posted_at < $3 AND amount_minor < 0
			GROUP BY COALESCE(merchant_name, description)
		),
		last_merchants AS (
			SELECT DISTINCT COALESCE(merchant_name, description) as merchant
			FROM transactions
			WHERE user_id = $1 AND posted_at >= $4 AND posted_at < $5
		)
		SELECT cm.merchant, cm.total
		FROM current_merchants cm
		WHERE cm.merchant NOT IN (SELECT merchant FROM last_merchants)
		  AND cm.total > 2000  -- Minimum €20
		ORDER BY cm.total DESC
		LIMIT 3
	`

	rows, err := s.repo.DB().Query(ctx, query, userID, currentStart, currentEnd, lastStart, lastEnd)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var changes []InsightChange
	for rows.Next() {
		var merchantName string
		var total int64

		if err := rows.Scan(&merchantName, &total); err != nil {
			continue
		}

		change := InsightChange{
			Type:         InsightChangeTypeNewMerchant,
			Title:        "New merchant",
			Description:  fmt.Sprintf("Started spending at %s (€%.2f)", merchantName, float64(total)/100),
			AmountChange: total,
			MerchantName: &merchantName,
			Icon:         "plus-circle",
			Sentiment:    InsightChangeSentimentNeutral,
		}
		changes = append(changes, change)
	}

	return changes
}

// detectIncomeChange detects significant income changes
func (s *Service) detectIncomeChange(ctx context.Context, userID uuid.UUID, currentStart, currentEnd, lastStart, lastEnd time.Time) *InsightChange {
	query := `
		SELECT
			COALESCE(SUM(CASE WHEN posted_at >= $2 AND posted_at < $3 THEN amount_minor ELSE 0 END), 0) as current_income,
			COALESCE(SUM(CASE WHEN posted_at >= $4 AND posted_at < $5 THEN amount_minor ELSE 0 END), 0) as last_income
		FROM transactions
		WHERE user_id = $1 AND amount_minor > 0
	`

	var currentIncome, lastIncome int64
	if err := s.repo.DB().QueryRow(ctx, query, userID, currentStart, currentEnd, lastStart, lastEnd).Scan(&currentIncome, &lastIncome); err != nil {
		return nil
	}

	delta := currentIncome - lastIncome
	if math.Abs(float64(delta)) < 5000 { // Less than €50 change
		return nil
	}

	var pctChange float64
	if lastIncome > 0 {
		pctChange = float64(delta) / float64(lastIncome) * 100
	}

	change := &InsightChange{
		Type:          InsightChangeTypeIncomeChange,
		AmountChange:  delta,
		PercentChange: pctChange,
		Icon:          "dollar-sign",
	}

	if delta > 0 {
		change.Title = "Income increased"
		change.Description = fmt.Sprintf("You received €%.2f more this month", float64(delta)/100)
		change.Sentiment = InsightChangeSentimentPositive
	} else {
		change.Title = "Income decreased"
		change.Description = fmt.Sprintf("You received €%.2f less this month", float64(-delta)/100)
		change.Sentiment = InsightChangeSentimentNegative
	}

	return change
}

// generateRecommendation creates the single most impactful action
func (s *Service) generateRecommendation(ctx context.Context, userID uuid.UUID, insights *MonthlyInsights) *ActionRecommendation {
	// Priority order of recommendations:

	// 1. Check for uncategorized transactions
	uncategorizedCount, uncategorizedAmount := s.getUncategorizedStats(ctx, userID, insights.MonthStart)
	if uncategorizedCount > 5 {
		return &ActionRecommendation{
			Type:            ActionTypeCategorizeTransactions,
			Title:           "Categorize transactions",
			Description:     fmt.Sprintf("You have %d uncategorized transactions (€%.2f)", uncategorizedCount, float64(uncategorizedAmount)/100),
			CTAText:         "Review Now",
			CTAAction:       "categorize",
			PotentialImpact: 0,
			Priority:        ActionPriorityMedium,
			Icon:            "tag",
		}
	}

	// 2. Check for high spending category
	if len(insights.TopCategories) > 0 && len(insights.Changes) > 0 {
		for _, change := range insights.Changes {
			if change.Type == InsightChangeTypeCategoryIncrease && change.AmountChange > 5000 {
				return &ActionRecommendation{
					Type:            ActionTypeReduceCategory,
					Title:           fmt.Sprintf("Review %s spending", *change.CategoryName),
					Description:     fmt.Sprintf("Spending increased by €%.2f this month", float64(change.AmountChange)/100),
					CTAText:         "View Breakdown",
					CTAAction:       fmt.Sprintf("category/%s", change.CategoryID),
					PotentialImpact: change.AmountChange / 2, // Assume 50% reduction possible
					Priority:        ActionPriorityHigh,
					Icon:            "alert-triangle",
				}
			}
		}
	}

	// 3. Default: Review transactions
	return &ActionRecommendation{
		Type:        ActionTypeReviewLargeExpense,
		Title:       "Review your spending",
		Description: fmt.Sprintf("You spent €%.2f this month", float64(insights.TotalSpend)/100),
		CTAText:     "View Details",
		CTAAction:   "transactions",
		Priority:    ActionPriorityLow,
		Icon:        "eye",
	}
}

// getUncategorizedStats returns uncategorized transaction count and total
func (s *Service) getUncategorizedStats(ctx context.Context, userID uuid.UUID, monthStart time.Time) (count int, amount int64) {
	query := `
		SELECT COUNT(*), COALESCE(SUM(ABS(amount_minor)), 0)
		FROM transactions
		WHERE user_id = $1 AND posted_at >= $2 AND category_id IS NULL
	`
	_ = s.repo.DB().QueryRow(ctx, query, userID, monthStart).Scan(&count, &amount)
	return
}

// generateHighlights creates human-readable highlights
func (s *Service) generateHighlights(insights *MonthlyInsights) []string {
	var highlights []string

	// Net position
	if insights.Net > 0 {
		highlights = append(highlights, fmt.Sprintf("You saved €%.2f this month", float64(insights.Net)/100))
	} else if insights.Net < 0 {
		highlights = append(highlights, fmt.Sprintf("You spent €%.2f more than you earned", float64(-insights.Net)/100))
	}

	// Comparison to last month
	if insights.SpendVsLastMonth > 0 {
		highlights = append(highlights, fmt.Sprintf("Spending is up %.0f%% vs last month", insights.SpendChangePercent))
	} else if insights.SpendVsLastMonth < 0 {
		highlights = append(highlights, fmt.Sprintf("Spending is down %.0f%% vs last month", -insights.SpendChangePercent))
	}

	// Top category
	if len(insights.TopCategories) > 0 {
		top := insights.TopCategories[0]
		highlights = append(highlights, fmt.Sprintf("Top spending: %s (€%.2f)", top.CategoryName, float64(top.AmountCents)/100))
	}

	return highlights
}
