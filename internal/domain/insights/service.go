package insights

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	authrepo "github.com/FACorreiaa/smart-finance-tracker/internal/domain/auth/repository"
	"github.com/FACorreiaa/smart-finance-tracker/pkg/push"
)

// SpendingPulse contains the computed insights for the dashboard
type SpendingPulse struct {
	// Spending comparison
	CurrentMonthSpend int64   // In cents
	LastMonthSpend    int64   // In cents (through same day)
	SpendDelta        int64   // Current - Last
	PacePercent       float64 // (Current / Last) * 100, 100 = on track

	// Alerts
	IsOverPace  bool    // True if spending pace > threshold
	PaceMessage string  // Human-readable pace status
	OverPaceBy  float64 // How much over pace (e.g., 1.25 = 25% over)

	// Context
	DayOfMonth       int
	TransactionCount int
	TopCategories    []TopCategory
	SurpriseExpenses []SurpriseExpense

	// Timestamps
	AsOfDate          time.Time
	CurrentMonthStart time.Time
	LastMonthStart    time.Time
}

// DashboardBlock represents a single block for the bento grid dashboard
type DashboardBlock struct {
	Type     string // "status", "hook", "cta"
	Title    string
	Subtitle string
	Value    string
	Icon     string
	Color    string // For status indication
	Action   string // Optional action identifier
}

// Service handles insights business logic
type Service struct {
	repo     InsightsRepository
	push     *push.Service
	authRepo authrepo.AuthRepository
	logger   *slog.Logger
}

// NewService creates a new insights service
func NewService(repo InsightsRepository, pushSvc *push.Service, authRepo authrepo.AuthRepository, logger *slog.Logger) *Service {
	return &Service{
		repo:     repo,
		push:     pushSvc,
		authRepo: authRepo,
		logger:   logger,
	}
}

const (
	// PaceThreshold is the percentage above which we consider "over pace"
	PaceThreshold = 125.0 // 25% over last month's pace

	// NotificationThreshold triggers a pace notification
	NotificationThreshold = 120.0 // 20% over
)

// GetSpendingPulse computes the spending pulse for a user
func (s *Service) GetSpendingPulse(ctx context.Context, userID uuid.UUID, asOf time.Time) (*SpendingPulse, error) {
	// Get raw spending data
	data, err := s.repo.GetSpendingPulseData(ctx, userID, asOf)
	if err != nil {
		return nil, err
	}

	// Get transaction count
	txCount, err := s.repo.GetTransactionCount(ctx, userID, asOf)
	if err != nil {
		txCount = 0 // Non-critical
	}

	// Get top categories
	categories, err := s.repo.GetTopCategories(ctx, userID, asOf, 5)
	if err != nil {
		categories = nil // Non-critical
	}

	// Get surprise expenses
	surprises, err := s.repo.GetSurpriseExpenses(ctx, userID, asOf, 3)
	if err != nil {
		surprises = nil // Non-critical
	}

	// Compute pace
	pulse := &SpendingPulse{
		CurrentMonthSpend: data.CurrentMonthSpend,
		LastMonthSpend:    data.LastMonthSpend,
		SpendDelta:        data.CurrentMonthSpend - data.LastMonthSpend,
		DayOfMonth:        data.DayOfMonth,
		TransactionCount:  txCount,
		TopCategories:     categories,
		SurpriseExpenses:  surprises,
		AsOfDate:          data.AsOfDate,
		CurrentMonthStart: data.CurrentMonthStart,
		LastMonthStart:    data.LastMonthStart,
	}

	// Calculate pace percentage
	if data.LastMonthSpend > 0 {
		pulse.PacePercent = float64(data.CurrentMonthSpend) / float64(data.LastMonthSpend) * 100
		pulse.OverPaceBy = pulse.PacePercent / 100.0
	} else if data.CurrentMonthSpend > 0 {
		pulse.PacePercent = 100 // No baseline, assume on track
		pulse.OverPaceBy = 1.0
	} else {
		pulse.PacePercent = 0
		pulse.OverPaceBy = 0
	}

	// Determine pace status
	pulse.IsOverPace = pulse.PacePercent > PaceThreshold
	pulse.PaceMessage = s.getPaceMessage(pulse.PacePercent, data.CurrentMonthSpend, data.LastMonthSpend)

	return pulse, nil
}

// ShouldNotify checks if a pace notification should be triggered
func (s *Service) ShouldNotify(pulse *SpendingPulse) bool {
	return pulse.PacePercent > NotificationThreshold && pulse.LastMonthSpend > 0
}

// GetDashboardBlocks returns blocks for the bento grid dashboard
func (s *Service) GetDashboardBlocks(ctx context.Context, userID uuid.UUID, asOf time.Time) ([]DashboardBlock, error) {
	pulse, err := s.GetSpendingPulse(ctx, userID, asOf)
	if err != nil {
		return nil, err
	}

	blocks := make([]DashboardBlock, 0, 3)

	// Block 1: Status - Pace indicator
	statusColor := "green"
	if pulse.PacePercent > 110 {
		statusColor = "yellow"
	}
	if pulse.IsOverPace {
		statusColor = "red"
	}

	blocks = append(blocks, DashboardBlock{
		Type:     "status",
		Title:    pulse.PaceMessage,
		Subtitle: s.getStatusSubtitle(pulse),
		Value:    formatMoney(pulse.CurrentMonthSpend),
		Icon:     "trending-up",
		Color:    statusColor,
	})

	// Block 2: Hook - Top category or surprise expense
	if len(pulse.SurpriseExpenses) > 0 {
		surprise := pulse.SurpriseExpenses[0]
		blocks = append(blocks, DashboardBlock{
			Type:     "hook",
			Title:    "New This Month",
			Subtitle: surprise.MerchantName,
			Value:    formatMoney(surprise.AmountCents),
			Icon:     "alert-circle",
			Color:    "blue",
		})
	} else if len(pulse.TopCategories) > 0 {
		top := pulse.TopCategories[0]
		blocks = append(blocks, DashboardBlock{
			Type:     "hook",
			Title:    "Top Category",
			Subtitle: top.CategoryName,
			Value:    formatMoney(top.AmountCents),
			Icon:     "pie-chart",
			Color:    "purple",
		})
	}

	// Block 3: CTA - Action item
	// TODO: Check for uncategorized transactions
	blocks = append(blocks, DashboardBlock{
		Type:     "cta",
		Title:    "Review Transactions",
		Subtitle: s.getTransactionCTA(pulse.TransactionCount),
		Icon:     "check-circle",
		Color:    "gray",
		Action:   "review_transactions",
	})

	return blocks, nil
}

// getPaceMessage returns a human-readable pace message
func (s *Service) getPaceMessage(_ float64, current, last int64) string {
	if last == 0 {
		if current == 0 {
			return "No spending yet"
		}
		return "First month tracking"
	}

	diff := current - last
	if diff > 0 {
		return "Spending ahead"
	} else if diff < 0 {
		return "Under budget"
	}
	return "On track"
}

// getStatusSubtitle returns context for the status block
func (s *Service) getStatusSubtitle(pulse *SpendingPulse) string {
	if pulse.LastMonthSpend == 0 {
		return "Start of your tracking journey"
	}

	diff := pulse.CurrentMonthSpend - pulse.LastMonthSpend
	if diff > 0 {
		return formatMoney(diff) + " more than this time last month"
	} else if diff < 0 {
		return formatMoney(-diff) + " less than this time last month"
	}
	return "Same as this time last month"
}

// getTransactionCTA returns CTA text for transaction count
func (s *Service) getTransactionCTA(count int) string {
	if count == 0 {
		return "Import your first transactions"
	}
	return formatInt(count) + " transactions this month"
}

// formatMoney formats cents as currency string
func formatMoney(cents int64) string {
	dollars := float64(cents) / 100
	if dollars >= 1000 {
		return "$" + formatFloat(dollars/1000, 1) + "k"
	}
	return "$" + formatFloat(dollars, 2)
}

func formatFloat(f float64, decimals int) string {
	if decimals == 1 {
		return formatFloatPrecision(f, 1)
	}
	return formatFloatPrecision(f, 2)
}

func formatFloatPrecision(f float64, precision int) string {
	// Simple formatting
	switch precision {
	case 1:
		return floatToString(f, 1)
	default:
		return floatToString(f, 2)
	}
}

func floatToString(f float64, decimals int) string {
	format := "%." + string(rune('0'+decimals)) + "f"
	return sprintf(format, f)
}

func sprintf(format string, args ...interface{}) string {
	// Use standard fmt
	return fmt.Sprintf(format, args...)
}

func formatInt(n int) string {
	return fmt.Sprintf("%d", n)
}

// TriggerPaceAlert creates a pace warning alert if conditions are met
func (s *Service) TriggerPaceAlert(ctx context.Context, userID uuid.UUID, pulse *SpendingPulse) error {
	// Only trigger if over notification threshold
	if pulse.PacePercent < NotificationThreshold {
		return nil
	}

	// Check if alert already sent today
	today := time.Now()
	hasAlert, err := s.repo.HasAlertToday(ctx, userID, AlertTypePaceWarning, today)
	if err != nil || hasAlert {
		return err // Already alerted today or error
	}

	// Determine severity
	severity := AlertSeverityWarning
	if pulse.PacePercent >= 150 {
		severity = AlertSeverityCritical
	} else if pulse.PacePercent < 130 {
		severity = AlertSeverityInfo
	}

	// Create the alert
	alert := &Alert{
		UserID:    userID,
		AlertType: AlertTypePaceWarning,
		Severity:  severity,
		Title:     pulse.PaceMessage,
		Message:   fmt.Sprintf("You've spent %s this month, which is %.0f%% of last month's pace by day %d.", formatMoney(pulse.CurrentMonthSpend), pulse.PacePercent, pulse.DayOfMonth),
		Metadata: map[string]any{
			"current_spend": pulse.CurrentMonthSpend,
			"last_spend":    pulse.LastMonthSpend,
			"pace_percent":  pulse.PacePercent,
			"day_of_month":  pulse.DayOfMonth,
			"as_of_date":    pulse.AsOfDate.Format("2006-01-02"),
		},
		AlertDate: today,
	}

	if err := s.repo.CreateAlert(ctx, alert); err != nil {
		return err
	}

	// Send push notification if user has a push token
	if s.push != nil && s.authRepo != nil {
		go func() {
			pushCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			token, err := s.authRepo.GetExpoPushToken(pushCtx, userID)
			if err != nil {
				if s.logger != nil {
					s.logger.Warn("failed to get push token for user", "userID", userID, "error", err)
				}
				return
			}
			if token == "" {
				return // No push token registered
			}

			msg := &push.Message{
				To:    token,
				Title: alert.Title,
				Body:  alert.Message,
				Data: map[string]any{
					"alert_type":   string(alert.AlertType),
					"severity":     string(alert.Severity),
					"pace_percent": pulse.PacePercent,
				},
			}

			if err := s.push.Send(pushCtx, msg); err != nil && s.logger != nil {
				s.logger.Warn("failed to send push notification", "userID", userID, "error", err)
			}
		}()
	}

	return nil
}

// GetUnreadAlerts returns unread alerts for a user
func (s *Service) GetUnreadAlerts(ctx context.Context, userID uuid.UUID, limit int) ([]Alert, error) {
	return s.repo.GetUnreadAlerts(ctx, userID, limit)
}

// MarkAlertRead marks an alert as read
func (s *Service) MarkAlertRead(ctx context.Context, alertID uuid.UUID) error {
	return s.repo.MarkAlertRead(ctx, alertID)
}

// MarkAlertDismissed marks an alert as dismissed
func (s *Service) MarkAlertDismissed(ctx context.Context, alertID uuid.UUID) error {
	return s.repo.MarkAlertDismissed(ctx, alertID)
}
