// Package service provides business logic for subscription management and detection.
package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/subscriptions/repository"
)

// ReviewReason represents why a subscription should be reviewed
type ReviewReason string

const (
	ReviewReasonUnused        ReviewReason = "unused"
	ReviewReasonPriceIncrease ReviewReason = "price_increase"
	ReviewReasonDuplicate     ReviewReason = "duplicate"
	ReviewReasonHighCost      ReviewReason = "high_cost"
	ReviewReasonNew           ReviewReason = "new"
)

// SubscriptionReviewItem represents a subscription that needs review
type SubscriptionReviewItem struct {
	Subscription      *repository.RecurringSubscription
	Reason            ReviewReason
	ReasonMessage     string
	RecommendedCancel bool
}

// DetectionResult contains the results of subscription detection
type DetectionResult struct {
	Detected     []*repository.RecurringSubscription
	NewCount     int
	UpdatedCount int
}

// Service provides subscription management business logic
type Service struct {
	repo repository.SubscriptionRepository
}

// NewService creates a new subscriptions service
func NewService(repo repository.SubscriptionRepository) *Service {
	return &Service{repo: repo}
}

// ListSubscriptions retrieves all subscriptions for a user
func (s *Service) ListSubscriptions(ctx context.Context, userID uuid.UUID, statusFilter *repository.RecurringStatus, includeCanceled bool) ([]*repository.RecurringSubscription, error) {
	return s.repo.ListByUserID(ctx, userID, statusFilter, includeCanceled)
}

// GetSubscription retrieves a subscription by ID
func (s *Service) GetSubscription(ctx context.Context, id uuid.UUID) (*repository.RecurringSubscription, error) {
	return s.repo.GetByID(ctx, id)
}

// UpdateStatus updates the status of a subscription
func (s *Service) UpdateStatus(ctx context.Context, id uuid.UUID, status repository.RecurringStatus) (*repository.RecurringSubscription, error) {
	if err := s.repo.UpdateStatus(ctx, id, status); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

// DetectSubscriptions analyzes transaction history to find recurring patterns
func (s *Service) DetectSubscriptions(ctx context.Context, userID uuid.UUID, since time.Time, minOccurrences int) (*DetectionResult, error) {
	if minOccurrences < 2 {
		minOccurrences = 2
	}

	// Get transaction groups by merchant
	groups, err := s.repo.GetMerchantTransactionGroups(ctx, userID, since, minOccurrences)
	if err != nil {
		return nil, fmt.Errorf("failed to get merchant groups: %w", err)
	}

	result := &DetectionResult{
		Detected: make([]*repository.RecurringSubscription, 0),
	}

	for _, group := range groups {
		// Analyze the pattern to determine cadence
		cadence, confidence := s.detectCadence(group.TransactionDates)
		if confidence < 0.5 {
			// Not confident enough in the pattern
			continue
		}

		// Calculate average amount (consistent amounts indicate subscription)
		avgAmount, amountVariance := s.calculateAmountStats(group.AmountPerTx)
		if amountVariance > 0.3 {
			// Amount varies too much (>30%), likely not a subscription
			continue
		}

		// Check if subscription already exists
		existing, err := s.repo.GetByUserAndMerchant(ctx, userID, group.MerchantName)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			continue
		}

		firstSeen := group.TransactionDates[0]
		lastSeen := group.TransactionDates[len(group.TransactionDates)-1]
		nextExpected := s.calculateNextExpected(lastSeen, cadence)

		if existing != nil {
			// Update existing subscription
			existing.AmountMinor = avgAmount
			existing.Cadence = cadence
			existing.LastSeenAt = &lastSeen
			existing.NextExpectedAt = nextExpected
			existing.OccurrenceCount = len(group.TransactionDates)

			if err := s.repo.Update(ctx, existing); err == nil {
				result.Detected = append(result.Detected, existing)
				result.UpdatedCount++
			}
		} else {
			// Create new subscription
			sub := &repository.RecurringSubscription{
				ID:              uuid.New(),
				UserID:          userID,
				MerchantName:    group.MerchantName,
				AmountMinor:     avgAmount,
				CurrencyCode:    "EUR", // TODO: detect from transactions
				Cadence:         cadence,
				Status:          repository.RecurringStatusActive,
				FirstSeenAt:     &firstSeen,
				LastSeenAt:      &lastSeen,
				NextExpectedAt:  nextExpected,
				OccurrenceCount: len(group.TransactionDates),
				CategoryID:      group.CategoryID,
			}

			if err := s.repo.Create(ctx, sub); err == nil {
				result.Detected = append(result.Detected, sub)
				result.NewCount++
			}
		}
	}

	return result, nil
}

// detectCadence analyzes transaction dates to determine the recurring pattern
func (s *Service) detectCadence(dates []time.Time) (repository.RecurringCadence, float64) {
	if len(dates) < 2 {
		return repository.RecurringCadenceUnknown, 0
	}

	// Sort dates
	sortedDates := make([]time.Time, len(dates))
	copy(sortedDates, dates)
	sort.Slice(sortedDates, func(i, j int) bool {
		return sortedDates[i].Before(sortedDates[j])
	})

	// Calculate intervals between transactions
	var intervals []float64
	for i := 1; i < len(sortedDates); i++ {
		days := sortedDates[i].Sub(sortedDates[i-1]).Hours() / 24
		intervals = append(intervals, days)
	}

	// Calculate average interval
	var sum float64
	for _, interval := range intervals {
		sum += interval
	}
	avgInterval := sum / float64(len(intervals))

	// Calculate variance to determine confidence
	var variance float64
	for _, interval := range intervals {
		variance += math.Pow(interval-avgInterval, 2)
	}
	variance = math.Sqrt(variance / float64(len(intervals)))

	// Normalize variance relative to interval
	normalizedVariance := variance / avgInterval
	confidence := 1.0 - math.Min(normalizedVariance, 1.0)

	// Determine cadence based on average interval
	var cadence repository.RecurringCadence
	switch {
	case avgInterval >= 5 && avgInterval <= 9:
		cadence = repository.RecurringCadenceWeekly
	case avgInterval >= 25 && avgInterval <= 35:
		cadence = repository.RecurringCadenceMonthly
	case avgInterval >= 85 && avgInterval <= 100:
		cadence = repository.RecurringCadenceQuarterly
	case avgInterval >= 350 && avgInterval <= 380:
		cadence = repository.RecurringCadenceAnnual
	default:
		cadence = repository.RecurringCadenceUnknown
		confidence *= 0.5 // Reduce confidence for unknown patterns
	}

	return cadence, confidence
}

// calculateAmountStats computes average amount and variance
func (s *Service) calculateAmountStats(amounts []int64) (avgAmount int64, variance float64) {
	if len(amounts) == 0 {
		return 0, 1.0
	}

	// Calculate average
	var sum int64
	for _, amt := range amounts {
		sum += amt
	}
	avg := float64(sum) / float64(len(amounts))

	// Calculate variance
	var varianceSum float64
	for _, amt := range amounts {
		varianceSum += math.Pow(float64(amt)-avg, 2)
	}
	stdDev := math.Sqrt(varianceSum / float64(len(amounts)))

	// Normalize variance
	if avg > 0 {
		variance = stdDev / avg
	} else {
		variance = 1.0
	}

	return int64(avg), variance
}

// calculateNextExpected predicts when the next charge will occur
func (s *Service) calculateNextExpected(lastSeen time.Time, cadence repository.RecurringCadence) *time.Time {
	var next time.Time
	switch cadence {
	case repository.RecurringCadenceWeekly:
		next = lastSeen.AddDate(0, 0, 7)
	case repository.RecurringCadenceMonthly:
		next = lastSeen.AddDate(0, 1, 0)
	case repository.RecurringCadenceQuarterly:
		next = lastSeen.AddDate(0, 3, 0)
	case repository.RecurringCadenceAnnual:
		next = lastSeen.AddDate(1, 0, 0)
	default:
		next = lastSeen.AddDate(0, 1, 0) // Default to monthly
	}
	return &next
}

// GetReviewChecklist returns subscriptions that should be reviewed
func (s *Service) GetReviewChecklist(ctx context.Context, userID uuid.UUID) ([]*SubscriptionReviewItem, int64, error) {
	subs, err := s.repo.ListByUserID(ctx, userID, nil, false)
	if err != nil {
		return nil, 0, err
	}

	var items []*SubscriptionReviewItem
	var potentialSavings int64
	now := time.Now()

	for _, sub := range subs {
		item := s.evaluateForReview(sub, now)
		if item != nil {
			items = append(items, item)
			if item.RecommendedCancel {
				potentialSavings += s.normalizeToMonthly(sub.AmountMinor, sub.Cadence)
			}
		}
	}

	return items, potentialSavings, nil
}

// evaluateForReview checks if a subscription needs review
func (s *Service) evaluateForReview(sub *repository.RecurringSubscription, now time.Time) *SubscriptionReviewItem {
	// Check if it's new (detected in last 30 days)
	if sub.CreatedAt.After(now.AddDate(0, 0, -30)) {
		return &SubscriptionReviewItem{
			Subscription:      sub,
			Reason:            ReviewReasonNew,
			ReasonMessage:     "New subscription detected. Confirm this is expected.",
			RecommendedCancel: false,
		}
	}

	// Check if unused (no transaction in expected + buffer period)
	if sub.LastSeenAt != nil {
		expectedGap := s.getCadenceDays(sub.Cadence)
		daysSinceLastSeen := int(now.Sub(*sub.LastSeenAt).Hours() / 24)

		if daysSinceLastSeen > expectedGap*2 {
			return &SubscriptionReviewItem{
				Subscription:      sub,
				Reason:            ReviewReasonUnused,
				ReasonMessage:     fmt.Sprintf("No charges in %d days. May no longer be active.", daysSinceLastSeen),
				RecommendedCancel: true,
			}
		}
	}

	// Check if high cost (top 20% by amount)
	monthlyAmount := s.normalizeToMonthly(sub.AmountMinor, sub.Cadence)
	if monthlyAmount > 5000 { // €50/month threshold
		return &SubscriptionReviewItem{
			Subscription:      sub,
			Reason:            ReviewReasonHighCost,
			ReasonMessage:     fmt.Sprintf("High monthly cost: €%.2f. Worth reviewing.", float64(monthlyAmount)/100),
			RecommendedCancel: false,
		}
	}

	return nil
}

// normalizeToMonthly converts any cadence to monthly equivalent
func (s *Service) normalizeToMonthly(amount int64, cadence repository.RecurringCadence) int64 {
	switch cadence {
	case repository.RecurringCadenceWeekly:
		return amount * 4
	case repository.RecurringCadenceMonthly:
		return amount
	case repository.RecurringCadenceQuarterly:
		return amount / 3
	case repository.RecurringCadenceAnnual:
		return amount / 12
	default:
		return amount
	}
}

// getCadenceDays returns the expected number of days between charges
func (s *Service) getCadenceDays(cadence repository.RecurringCadence) int {
	switch cadence {
	case repository.RecurringCadenceWeekly:
		return 7
	case repository.RecurringCadenceMonthly:
		return 30
	case repository.RecurringCadenceQuarterly:
		return 90
	case repository.RecurringCadenceAnnual:
		return 365
	default:
		return 30
	}
}

// GetTotalMonthlySubscriptionCost calculates total monthly subscription cost
func (s *Service) GetTotalMonthlySubscriptionCost(ctx context.Context, userID uuid.UUID) (int64, int, error) {
	status := repository.RecurringStatusActive
	subs, err := s.repo.ListByUserID(ctx, userID, &status, false)
	if err != nil {
		return 0, 0, err
	}

	var total int64
	for _, sub := range subs {
		total += s.normalizeToMonthly(sub.AmountMinor, sub.Cadence)
	}

	return total, len(subs), nil
}
