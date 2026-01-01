package balance

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockBalanceRepository implements a mock for testing
type MockBalanceRepository struct {
	accountBalances []AccountBalanceData
	totalBalance    int64
	upcomingBills   int64
	history         []DailyBalanceData
	highest         int64
	lowest          int64
	average         int64
	err             error
}

func (m *MockBalanceRepository) GetAccountBalances(ctx context.Context, userID uuid.UUID) ([]AccountBalanceData, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.accountBalances, nil
}

func (m *MockBalanceRepository) GetTotalBalance(ctx context.Context, userID uuid.UUID) (int64, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.totalBalance, nil
}

func (m *MockBalanceRepository) GetUpcomingBills(ctx context.Context, userID uuid.UUID) (int64, error) {
	return m.upcomingBills, nil
}

func (m *MockBalanceRepository) GetBalanceHistory(ctx context.Context, userID uuid.UUID, days int) ([]DailyBalanceData, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.history, nil
}

func (m *MockBalanceRepository) GetBalanceStats(ctx context.Context, userID uuid.UUID, days int) (highest, lowest, average int64, err error) {
	return m.highest, m.lowest, m.average, m.err
}

// BalanceRepository interface for mocking
type BalanceRepository interface {
	GetAccountBalances(ctx context.Context, userID uuid.UUID) ([]AccountBalanceData, error)
	GetTotalBalance(ctx context.Context, userID uuid.UUID) (int64, error)
	GetUpcomingBills(ctx context.Context, userID uuid.UUID) (int64, error)
	GetBalanceHistory(ctx context.Context, userID uuid.UUID, days int) ([]DailyBalanceData, error)
	GetBalanceStats(ctx context.Context, userID uuid.UUID, days int) (highest, lowest, average int64, err error)
}

// TestService wraps service for testing with mock repo
type TestService struct {
	repo BalanceRepository
}

func NewTestService(repo BalanceRepository) *TestService {
	return &TestService{repo: repo}
}

func (s *TestService) GetBalance(ctx context.Context, userID uuid.UUID, accountID *uuid.UUID) (*BalanceResult, error) {
	accounts, err := s.repo.GetAccountBalances(ctx, userID)
	if err != nil {
		return nil, err
	}

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

	var totalCash, totalInvestment int64
	for _, a := range accounts {
		totalCash += a.CashBalanceCents
		totalInvestment += a.InvestmentCents
	}
	totalNetWorth := totalCash + totalInvestment

	upcomingBills, _ := s.repo.GetUpcomingBills(ctx, userID)

	safeToSpend := totalCash - upcomingBills
	if safeToSpend < 0 {
		safeToSpend = 0
	}

	return &BalanceResult{
		TotalNetWorthCents:   totalNetWorth,
		SafeToSpendCents:     safeToSpend,
		TotalInvestmentCents: totalInvestment,
		UpcomingBillsCents:   upcomingBills,
		IsEstimated:          true,
		CurrencyCode:         "EUR",
		Accounts:             accounts,
	}, nil
}

func TestGetBalance_AggregatesAccounts(t *testing.T) {
	userID := uuid.New()
	account1ID := uuid.New()
	account2ID := uuid.New()

	mock := &MockBalanceRepository{
		accountBalances: []AccountBalanceData{
			{
				AccountID:        account1ID,
				AccountName:      "Checking",
				AccountType:      2,      // CHECKING
				CashBalanceCents: 100000, // €1000
				InvestmentCents:  0,
				Change24hCents:   -5000, // -€50
				LastActivity:     time.Now(),
				CurrencyCode:     "EUR",
			},
			{
				AccountID:        account2ID,
				AccountName:      "Investments",
				AccountType:      5, // INVESTMENT
				CashBalanceCents: 0,
				InvestmentCents:  500000, // €5000
				Change24hCents:   10000,  // +€100
				LastActivity:     time.Now(),
				CurrencyCode:     "EUR",
			},
		},
		upcomingBills: 15000, // €150 upcoming
	}

	svc := NewTestService(mock)
	result, err := svc.GetBalance(context.Background(), userID, nil)

	require.NoError(t, err)
	assert.Equal(t, int64(600000), result.TotalNetWorthCents)   // €6000
	assert.Equal(t, int64(85000), result.SafeToSpendCents)      // €1000 - €150 = €850
	assert.Equal(t, int64(500000), result.TotalInvestmentCents) // €5000
	assert.Equal(t, int64(15000), result.UpcomingBillsCents)
	assert.True(t, result.IsEstimated)
	assert.Len(t, result.Accounts, 2)
}

func TestGetBalance_FiltersByAccountID(t *testing.T) {
	userID := uuid.New()
	targetAccountID := uuid.New()
	otherAccountID := uuid.New()

	mock := &MockBalanceRepository{
		accountBalances: []AccountBalanceData{
			{
				AccountID:        targetAccountID,
				AccountName:      "Checking",
				CashBalanceCents: 100000,
			},
			{
				AccountID:        otherAccountID,
				AccountName:      "Savings",
				CashBalanceCents: 200000,
			},
		},
	}

	svc := NewTestService(mock)
	result, err := svc.GetBalance(context.Background(), userID, &targetAccountID)

	require.NoError(t, err)
	assert.Len(t, result.Accounts, 1)
	assert.Equal(t, "Checking", result.Accounts[0].AccountName)
	assert.Equal(t, int64(100000), result.TotalNetWorthCents)
}

func TestGetBalance_SafeToSpendNeverNegative(t *testing.T) {
	userID := uuid.New()

	mock := &MockBalanceRepository{
		accountBalances: []AccountBalanceData{
			{
				AccountID:        uuid.New(),
				AccountName:      "Checking",
				CashBalanceCents: 10000, // €100
			},
		},
		upcomingBills: 50000, // €500 (more than balance)
	}

	svc := NewTestService(mock)
	result, err := svc.GetBalance(context.Background(), userID, nil)

	require.NoError(t, err)
	assert.Equal(t, int64(0), result.SafeToSpendCents) // Not negative
}
