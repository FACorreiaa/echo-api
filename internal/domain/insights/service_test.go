package insights_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/insights"
)

// MockInsightsRepo is a mock implementation of the insights repository
type MockInsightsRepo struct {
	alerts       []insights.Alert
	alertsByUser map[uuid.UUID][]insights.Alert
	alertToday   bool
}

func NewMockInsightsRepo() *MockInsightsRepo {
	return &MockInsightsRepo{
		alerts:       make([]insights.Alert, 0),
		alertsByUser: make(map[uuid.UUID][]insights.Alert),
	}
}

func (m *MockInsightsRepo) GetSpendingPulseData(ctx context.Context, userID uuid.UUID, asOf time.Time) (*insights.SpendingPulseData, error) {
	return &insights.SpendingPulseData{
		CurrentMonthSpend: 50000, // $500
		LastMonthSpend:    40000, // $400
		DayOfMonth:        15,
		AsOfDate:          asOf,
	}, nil
}

func (m *MockInsightsRepo) GetTransactionCount(ctx context.Context, userID uuid.UUID, asOf time.Time) (int, error) {
	return 25, nil
}

func (m *MockInsightsRepo) GetTopCategories(ctx context.Context, userID uuid.UUID, asOf time.Time, limit int) ([]insights.TopCategory, error) {
	return []insights.TopCategory{
		{CategoryName: "Food", AmountCents: 15000, TxCount: 10},
		{CategoryName: "Transport", AmountCents: 8000, TxCount: 5},
	}, nil
}

func (m *MockInsightsRepo) GetSurpriseExpenses(ctx context.Context, userID uuid.UUID, asOf time.Time, limit int) ([]insights.SurpriseExpense, error) {
	return []insights.SurpriseExpense{}, nil
}

func (m *MockInsightsRepo) HasAlertToday(ctx context.Context, userID uuid.UUID, alertType insights.AlertType, date time.Time) (bool, error) {
	return m.alertToday, nil
}

func (m *MockInsightsRepo) CreateAlert(ctx context.Context, alert *insights.Alert) error {
	alert.ID = uuid.New()
	alert.CreatedAt = time.Now()
	m.alerts = append(m.alerts, *alert)
	m.alertsByUser[alert.UserID] = append(m.alertsByUser[alert.UserID], *alert)
	return nil
}

func (m *MockInsightsRepo) GetUnreadAlerts(ctx context.Context, userID uuid.UUID, limit int) ([]insights.Alert, error) {
	alerts := m.alertsByUser[userID]
	result := make([]insights.Alert, 0)
	for _, a := range alerts {
		if !a.IsRead && !a.IsDismissed {
			result = append(result, a)
		}
	}
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (m *MockInsightsRepo) MarkAlertRead(ctx context.Context, alertID uuid.UUID) error {
	for i := range m.alerts {
		if m.alerts[i].ID == alertID {
			m.alerts[i].IsRead = true
			now := time.Now()
			m.alerts[i].ReadAt = &now
			// Update user alerts too
			for j := range m.alertsByUser[m.alerts[i].UserID] {
				if m.alertsByUser[m.alerts[i].UserID][j].ID == alertID {
					m.alertsByUser[m.alerts[i].UserID][j].IsRead = true
					m.alertsByUser[m.alerts[i].UserID][j].ReadAt = &now
				}
			}
			return nil
		}
	}
	return nil
}

func (m *MockInsightsRepo) MarkAlertDismissed(ctx context.Context, alertID uuid.UUID) error {
	for i := range m.alerts {
		if m.alerts[i].ID == alertID {
			m.alerts[i].IsDismissed = true
			now := time.Now()
			m.alerts[i].DismissedAt = &now
			// Update user alerts too
			for j := range m.alertsByUser[m.alerts[i].UserID] {
				if m.alertsByUser[m.alerts[i].UserID][j].ID == alertID {
					m.alertsByUser[m.alerts[i].UserID][j].IsDismissed = true
					m.alertsByUser[m.alerts[i].UserID][j].DismissedAt = &now
				}
			}
			return nil
		}
	}
	return nil
}

// Import insights mocks
func (m *MockInsightsRepo) GetImportInsights(ctx context.Context, importJobID uuid.UUID) (*insights.ImportJobInsights, error) {
	return nil, nil
}

func (m *MockInsightsRepo) UpsertImportInsights(ctx context.Context, i *insights.ImportJobInsights) error {
	return nil
}

func (m *MockInsightsRepo) GetDataSourceHealth(ctx context.Context, userID uuid.UUID) ([]insights.DataSourceHealth, error) {
	return nil, nil
}

func (m *MockInsightsRepo) RefreshDataSourceHealth(ctx context.Context) error {
	return nil
}

func (m *MockInsightsRepo) DB() *pgxpool.Pool {
	return nil // Mock returns nil - wrapped queries won't work in tests
}

// SetAlertToday sets whether an alert exists today (for deduplication tests)
func (m *MockInsightsRepo) SetAlertToday(exists bool) {
	m.alertToday = exists
}

// GetAlerts returns all alerts (for assertions)
func (m *MockInsightsRepo) GetAlerts() []insights.Alert {
	return m.alerts
}

func TestTriggerPaceAlert_CreatesAlertWhenOverThreshold(t *testing.T) {
	repo := NewMockInsightsRepo()
	svc := insights.NewService(repo, nil, nil, nil)

	userID := uuid.New()
	pulse := &insights.SpendingPulse{
		CurrentMonthSpend: 60000, // $600
		LastMonthSpend:    40000, // $400
		PacePercent:       150.0, // 50% over
		PaceMessage:       "Spending ahead",
		DayOfMonth:        15,
		AsOfDate:          time.Now(),
	}

	err := svc.TriggerPaceAlert(context.Background(), userID, pulse)
	require.NoError(t, err)

	alerts := repo.GetAlerts()
	assert.Len(t, alerts, 1)
	assert.Equal(t, insights.AlertTypePaceWarning, alerts[0].AlertType)
	assert.Equal(t, insights.AlertSeverityCritical, alerts[0].Severity) // 150% is critical
	assert.Equal(t, userID, alerts[0].UserID)
}

func TestTriggerPaceAlert_NoAlertUnderThreshold(t *testing.T) {
	repo := NewMockInsightsRepo()
	svc := insights.NewService(repo, nil, nil, nil)

	userID := uuid.New()
	pulse := &insights.SpendingPulse{
		CurrentMonthSpend: 45000, // $450
		LastMonthSpend:    40000, // $400
		PacePercent:       112.5, // Only 12.5% over, under 120% threshold
		PaceMessage:       "Slightly ahead",
		DayOfMonth:        15,
		AsOfDate:          time.Now(),
	}

	err := svc.TriggerPaceAlert(context.Background(), userID, pulse)
	require.NoError(t, err)

	alerts := repo.GetAlerts()
	assert.Len(t, alerts, 0) // No alert created
}

func TestTriggerPaceAlert_Deduplication(t *testing.T) {
	repo := NewMockInsightsRepo()
	repo.SetAlertToday(true) // Simulate existing alert
	svc := insights.NewService(repo, nil, nil, nil)

	userID := uuid.New()
	pulse := &insights.SpendingPulse{
		CurrentMonthSpend: 60000,
		LastMonthSpend:    40000,
		PacePercent:       150.0,
		PaceMessage:       "Spending ahead",
		DayOfMonth:        15,
		AsOfDate:          time.Now(),
	}

	err := svc.TriggerPaceAlert(context.Background(), userID, pulse)
	require.NoError(t, err)

	alerts := repo.GetAlerts()
	assert.Len(t, alerts, 0) // No duplicate alert
}

func TestTriggerPaceAlert_SeverityLevels(t *testing.T) {
	tests := []struct {
		name             string
		pacePercent      float64
		expectedSeverity insights.AlertSeverity
	}{
		{"warning at 120%", 120.0, insights.AlertSeverityInfo},
		{"warning at 129%", 129.0, insights.AlertSeverityInfo},
		{"warning at 130%", 130.0, insights.AlertSeverityWarning},
		{"warning at 149%", 149.0, insights.AlertSeverityWarning},
		{"critical at 150%", 150.0, insights.AlertSeverityCritical},
		{"critical at 200%", 200.0, insights.AlertSeverityCritical},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockInsightsRepo()
			svc := insights.NewService(repo, nil, nil, nil)

			pulse := &insights.SpendingPulse{
				PacePercent: tt.pacePercent,
				PaceMessage: "Test",
				DayOfMonth:  15,
				AsOfDate:    time.Now(),
			}

			err := svc.TriggerPaceAlert(context.Background(), uuid.New(), pulse)
			require.NoError(t, err)

			alerts := repo.GetAlerts()
			require.Len(t, alerts, 1)
			assert.Equal(t, tt.expectedSeverity, alerts[0].Severity)
		})
	}
}

func TestGetUnreadAlerts(t *testing.T) {
	repo := NewMockInsightsRepo()
	svc := insights.NewService(repo, nil, nil, nil)

	userID := uuid.New()

	// Create multiple alerts
	for i := 0; i < 5; i++ {
		pulse := &insights.SpendingPulse{
			PacePercent: 130.0,
			PaceMessage: "Test",
			DayOfMonth:  15 + i,
			AsOfDate:    time.Now().AddDate(0, 0, i),
		}
		repo.SetAlertToday(false) // Allow creation
		_ = svc.TriggerPaceAlert(context.Background(), userID, pulse)
	}

	// Get unread alerts
	alerts, err := svc.GetUnreadAlerts(context.Background(), userID, 10)
	require.NoError(t, err)
	assert.Len(t, alerts, 5)
}

func TestMarkAlertRead(t *testing.T) {
	repo := NewMockInsightsRepo()
	svc := insights.NewService(repo, nil, nil, nil)

	userID := uuid.New()
	pulse := &insights.SpendingPulse{
		PacePercent: 130.0,
		PaceMessage: "Test",
		DayOfMonth:  15,
		AsOfDate:    time.Now(),
	}
	err := svc.TriggerPaceAlert(context.Background(), userID, pulse)
	require.NoError(t, err)

	alertID := repo.GetAlerts()[0].ID

	// Mark as read
	err = svc.MarkAlertRead(context.Background(), alertID)
	require.NoError(t, err)

	// Verify it's marked
	alerts := repo.GetAlerts()
	assert.True(t, alerts[0].IsRead)
	assert.NotNil(t, alerts[0].ReadAt)

	// Verify unread count is now 0
	unread, err := svc.GetUnreadAlerts(context.Background(), userID, 10)
	require.NoError(t, err)
	assert.Len(t, unread, 0)
}

func TestMarkAlertDismissed(t *testing.T) {
	repo := NewMockInsightsRepo()
	svc := insights.NewService(repo, nil, nil, nil)

	userID := uuid.New()
	pulse := &insights.SpendingPulse{
		PacePercent: 130.0,
		PaceMessage: "Test",
		DayOfMonth:  15,
		AsOfDate:    time.Now(),
	}
	err := svc.TriggerPaceAlert(context.Background(), userID, pulse)
	require.NoError(t, err)

	alertID := repo.GetAlerts()[0].ID

	// Dismiss
	err = svc.MarkAlertDismissed(context.Background(), alertID)
	require.NoError(t, err)

	// Verify it's dismissed
	alerts := repo.GetAlerts()
	assert.True(t, alerts[0].IsDismissed)
	assert.NotNil(t, alerts[0].DismissedAt)

	// Verify unread count is now 0
	unread, err := svc.GetUnreadAlerts(context.Background(), userID, 10)
	require.NoError(t, err)
	assert.Len(t, unread, 0)
}

func TestSpendingPulse_CalculatesPaceCorrectly(t *testing.T) {
	repo := NewMockInsightsRepo()
	svc := insights.NewService(repo, nil, nil, nil)

	userID := uuid.New()
	pulse, err := svc.GetSpendingPulse(context.Background(), userID, time.Now())
	require.NoError(t, err)

	// Mock returns $500 current, $400 last
	// Pace = (500/400) * 100 = 125%
	assert.Equal(t, int64(50000), pulse.CurrentMonthSpend)
	assert.Equal(t, int64(40000), pulse.LastMonthSpend)
	assert.InDelta(t, 125.0, pulse.PacePercent, 0.1)
	// IsOverPace is true when pace > PaceThreshold (125%)
	// At exactly 125%, IsOverPace is false (not strictly over)
	assert.False(t, pulse.IsOverPace) // 125% == threshold, not over
}
