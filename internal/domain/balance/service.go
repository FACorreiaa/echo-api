package balance

import (
	"context"

	"github.com/google/uuid"
)

// Service handles balance business logic
type Service struct {
	repo *Repository
}

// NewService creates a new balance service
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// BalanceResult holds the complete balance response
type BalanceResult struct {
	TotalNetWorthCents   int64
	SafeToSpendCents     int64
	TotalInvestmentCents int64
	UpcomingBillsCents   int64
	IsEstimated          bool
	CurrencyCode         string
	Accounts             []AccountBalanceData
}

// HistoryResult holds balance history response
type HistoryResult struct {
	History      []DailyBalanceData
	HighestCents int64
	LowestCents  int64
	AverageCents int64
	CurrencyCode string
}

// GetBalance computes the user's current balance
func (s *Service) GetBalance(ctx context.Context, userID uuid.UUID, accountID *uuid.UUID) (*BalanceResult, error) {
	// Get account balances
	accounts, err := s.repo.GetAccountBalances(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Filter by account if specified
	if accountID != nil {
		filtered := make([]AccountBalanceData, 0, 1)
		for _, a := range accounts {
			if a.AccountID == *accountID {
				filtered = append(filtered, a)
				break
			}
		}
		accounts = filtered
	}

	// Calculate totals
	var totalCash, totalInvestment int64
	for _, a := range accounts {
		totalCash += a.CashBalanceCents
		totalInvestment += a.InvestmentCents
	}
	totalNetWorth := totalCash + totalInvestment

	// Get upcoming bills
	upcomingBills, _ := s.repo.GetUpcomingBills(ctx, userID)

	// Safe to spend = cash - upcoming bills
	safeToSpend := totalCash - upcomingBills
	if safeToSpend < 0 {
		safeToSpend = 0
	}

	return &BalanceResult{
		TotalNetWorthCents:   totalNetWorth,
		SafeToSpendCents:     safeToSpend,
		TotalInvestmentCents: totalInvestment,
		UpcomingBillsCents:   upcomingBills,
		IsEstimated:          true, // Always true until we have real bank APIs
		CurrencyCode:         "EUR",
		Accounts:             accounts,
	}, nil
}

// GetBalanceHistory returns daily balance snapshots for charts
func (s *Service) GetBalanceHistory(ctx context.Context, userID uuid.UUID, days int, accountID *uuid.UUID) (*HistoryResult, error) {
	// Get history
	history, err := s.repo.GetBalanceHistory(ctx, userID, days)
	if err != nil {
		return nil, err
	}

	// Get stats
	highest, lowest, average, err := s.repo.GetBalanceStats(ctx, userID, days)
	if err != nil {
		// Non-fatal, continue with zeros
		highest, lowest, average = 0, 0, 0
	}

	return &HistoryResult{
		History:      history,
		HighestCents: highest,
		LowestCents:  lowest,
		AverageCents: average,
		CurrencyCode: "EUR",
	}, nil
}
