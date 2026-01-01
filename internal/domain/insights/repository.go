package insights

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SpendingPulseData contains the raw data for pulse calculation
type SpendingPulseData struct {
	CurrentMonthSpend int64     // Spend this month through asOf day
	LastMonthSpend    int64     // Spend last month through same day
	CurrentMonthStart time.Time // Start of current month
	LastMonthStart    time.Time // Start of last month
	AsOfDate          time.Time // The reference date
	DayOfMonth        int       // Day of month for the asOf date
}

// SurpriseExpense represents a significant expense not seen in previous period
type SurpriseExpense struct {
	TransactionID uuid.UUID
	Description   string
	MerchantName  string
	AmountCents   int64
	PostedAt      time.Time
	CategoryName  *string
}

// TopCategory represents spending by category
type TopCategory struct {
	CategoryID   *uuid.UUID
	CategoryName string
	AmountCents  int64
	TxCount      int
}

// InsightsRepository defines the interface for insights data access
type InsightsRepository interface {
	GetSpendingPulseData(ctx context.Context, userID uuid.UUID, asOf time.Time) (*SpendingPulseData, error)
	GetTransactionCount(ctx context.Context, userID uuid.UUID, asOf time.Time) (int, error)
	GetTopCategories(ctx context.Context, userID uuid.UUID, asOf time.Time, limit int) ([]TopCategory, error)
	GetSurpriseExpenses(ctx context.Context, userID uuid.UUID, asOf time.Time, limit int) ([]SurpriseExpense, error)
	HasAlertToday(ctx context.Context, userID uuid.UUID, alertType AlertType, date time.Time) (bool, error)
	CreateAlert(ctx context.Context, alert *Alert) error
	GetUnreadAlerts(ctx context.Context, userID uuid.UUID, limit int) ([]Alert, error)
	MarkAlertRead(ctx context.Context, alertID uuid.UUID) error
	MarkAlertDismissed(ctx context.Context, alertID uuid.UUID) error
}

// Ensure Repository implements InsightsRepository
var _ InsightsRepository = (*Repository)(nil)

// Repository handles database queries for insights
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new insights repository
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// GetSpendingPulseData fetches spending data for current vs last month comparison
func (r *Repository) GetSpendingPulseData(ctx context.Context, userID uuid.UUID, asOf time.Time) (*SpendingPulseData, error) {
	// Calculate date ranges
	year, month, day := asOf.Date()
	currentMonthStart := time.Date(year, month, 1, 0, 0, 0, 0, asOf.Location())
	lastMonthStart := currentMonthStart.AddDate(0, -1, 0)

	// Current month: from start of month to asOf (inclusive)
	currentMonthEnd := asOf.AddDate(0, 0, 1) // End of asOf day

	// Last month: from start of last month to same day last month
	lastMonthSameDay := time.Date(year, month-1, day, 23, 59, 59, 0, asOf.Location())
	// Handle edge case: if current day > last month's max days
	lastMonthLastDay := lastMonthStart.AddDate(0, 1, -1).Day()
	if day > lastMonthLastDay {
		lastMonthSameDay = time.Date(year, month-1, lastMonthLastDay, 23, 59, 59, 0, asOf.Location())
	}

	// Query current month spend (expenses only, negative amounts)
	var currentSpend int64
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(ABS(amount_minor)), 0)
		FROM transactions
		WHERE user_id = $1
		  AND posted_at >= $2
		  AND posted_at < $3
		  AND amount_minor < 0
	`, userID, currentMonthStart, currentMonthEnd).Scan(&currentSpend)
	if err != nil {
		return nil, err
	}

	// Query last month spend through same day
	var lastSpend int64
	err = r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(ABS(amount_minor)), 0)
		FROM transactions
		WHERE user_id = $1
		  AND posted_at >= $2
		  AND posted_at <= $3
		  AND amount_minor < 0
	`, userID, lastMonthStart, lastMonthSameDay).Scan(&lastSpend)
	if err != nil {
		return nil, err
	}

	return &SpendingPulseData{
		CurrentMonthSpend: currentSpend,
		LastMonthSpend:    lastSpend,
		CurrentMonthStart: currentMonthStart,
		LastMonthStart:    lastMonthStart,
		AsOfDate:          asOf,
		DayOfMonth:        day,
	}, nil
}

// GetSurpriseExpenses finds high-value transactions in current month not in last month
func (r *Repository) GetSurpriseExpenses(ctx context.Context, userID uuid.UUID, asOf time.Time, limit int) ([]SurpriseExpense, error) {
	year, month, _ := asOf.Date()
	currentMonthStart := time.Date(year, month, 1, 0, 0, 0, 0, asOf.Location())
	lastMonthStart := currentMonthStart.AddDate(0, -1, 0)
	lastMonthEnd := currentMonthStart

	// Find transactions this month that have no similar merchant in last month
	query := `
		WITH current_month_txs AS (
			SELECT t.id, t.description, t.merchant_name, t.amount_minor, t.posted_at, c.name as category_name
			FROM transactions t
			LEFT JOIN categories c ON t.category_id = c.id
			WHERE t.user_id = $1
			  AND t.posted_at >= $2
			  AND t.posted_at < $3
			  AND t.amount_minor < 0
		),
		last_month_merchants AS (
			SELECT DISTINCT COALESCE(merchant_name, description) as merchant
			FROM transactions
			WHERE user_id = $1
			  AND posted_at >= $4
			  AND posted_at < $5
		)
		SELECT cm.id, cm.description, cm.merchant_name, cm.amount_minor, cm.posted_at, cm.category_name
		FROM current_month_txs cm
		WHERE COALESCE(cm.merchant_name, cm.description) NOT IN (SELECT merchant FROM last_month_merchants)
		ORDER BY ABS(cm.amount_minor) DESC
		LIMIT $6
	`

	rows, err := r.db.Query(ctx, query,
		userID,
		currentMonthStart,
		asOf.AddDate(0, 0, 1),
		lastMonthStart,
		lastMonthEnd,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var expenses []SurpriseExpense
	for rows.Next() {
		var e SurpriseExpense
		var merchantName *string
		if err := rows.Scan(&e.TransactionID, &e.Description, &merchantName, &e.AmountCents, &e.PostedAt, &e.CategoryName); err != nil {
			return nil, err
		}
		if merchantName != nil {
			e.MerchantName = *merchantName
		} else {
			e.MerchantName = e.Description
		}
		expenses = append(expenses, e)
	}

	return expenses, rows.Err()
}

// GetTopCategories returns spending by category for current month
func (r *Repository) GetTopCategories(ctx context.Context, userID uuid.UUID, asOf time.Time, limit int) ([]TopCategory, error) {
	year, month, _ := asOf.Date()
	currentMonthStart := time.Date(year, month, 1, 0, 0, 0, 0, asOf.Location())

	query := `
		SELECT t.category_id, COALESCE(c.name, 'Uncategorized') as category_name,
		       SUM(ABS(t.amount_minor)) as total_amount,
		       COUNT(*) as tx_count
		FROM transactions t
		LEFT JOIN categories c ON t.category_id = c.id
		WHERE t.user_id = $1
		  AND t.posted_at >= $2
		  AND t.posted_at < $3
		  AND t.amount_minor < 0
		GROUP BY t.category_id, c.name
		ORDER BY total_amount DESC
		LIMIT $4
	`

	rows, err := r.db.Query(ctx, query, userID, currentMonthStart, asOf.AddDate(0, 0, 1), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []TopCategory
	for rows.Next() {
		var c TopCategory
		if err := rows.Scan(&c.CategoryID, &c.CategoryName, &c.AmountCents, &c.TxCount); err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}

	return categories, rows.Err()
}

// GetTransactionCount returns the number of transactions for current month
func (r *Repository) GetTransactionCount(ctx context.Context, userID uuid.UUID, asOf time.Time) (int, error) {
	year, month, _ := asOf.Date()
	currentMonthStart := time.Date(year, month, 1, 0, 0, 0, 0, asOf.Location())

	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM transactions
		WHERE user_id = $1
		  AND posted_at >= $2
		  AND posted_at < $3
	`, userID, currentMonthStart, asOf.AddDate(0, 0, 1)).Scan(&count)

	return count, err
}

// AlertType defines the type of alert
type AlertType string

const (
	AlertTypePaceWarning     AlertType = "pace_warning"
	AlertTypeSurpriseExpense AlertType = "surprise_expense"
	AlertTypeGoalProgress    AlertType = "goal_progress"
	AlertTypeSubscriptionDue AlertType = "subscription_due"
)

// AlertSeverity defines the severity level
type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityCritical AlertSeverity = "critical"
)

// Alert represents a user notification
type Alert struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	AlertType     AlertType
	Severity      AlertSeverity
	Title         string
	Message       string
	Metadata      map[string]any
	ReferenceType *string
	ReferenceID   *uuid.UUID
	IsRead        bool
	IsDismissed   bool
	AlertDate     time.Time
	CreatedAt     time.Time
	ReadAt        *time.Time
	DismissedAt   *time.Time
}

// CreateAlert creates a new alert (with deduplication - one per type per day)
func (r *Repository) CreateAlert(ctx context.Context, alert *Alert) error {
	query := `
		INSERT INTO alerts (user_id, alert_type, severity, title, message, metadata, reference_type, reference_id, alert_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (user_id, alert_type, alert_date) DO NOTHING
		RETURNING id, created_at
	`

	err := r.db.QueryRow(ctx, query,
		alert.UserID,
		alert.AlertType,
		alert.Severity,
		alert.Title,
		alert.Message,
		alert.Metadata,
		alert.ReferenceType,
		alert.ReferenceID,
		alert.AlertDate,
	).Scan(&alert.ID, &alert.CreatedAt)

	// Ignore duplicate (ON CONFLICT DO NOTHING)
	if err != nil && err.Error() == "no rows in result set" {
		return nil
	}
	return err
}

// HasAlertToday checks if an alert of given type was already sent today
func (r *Repository) HasAlertToday(ctx context.Context, userID uuid.UUID, alertType AlertType, date time.Time) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM alerts
			WHERE user_id = $1 AND alert_type = $2 AND alert_date = $3
		)
	`, userID, alertType, date.Format("2006-01-02")).Scan(&exists)

	return exists, err
}

// GetUnreadAlerts returns unread alerts for a user
func (r *Repository) GetUnreadAlerts(ctx context.Context, userID uuid.UUID, limit int) ([]Alert, error) {
	query := `
		SELECT id, user_id, alert_type, severity, title, message, metadata,
		       reference_type, reference_id, is_read, is_dismissed, alert_date, created_at
		FROM alerts
		WHERE user_id = $1 AND is_read = false AND is_dismissed = false
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.db.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []Alert
	for rows.Next() {
		var a Alert
		if err := rows.Scan(
			&a.ID, &a.UserID, &a.AlertType, &a.Severity, &a.Title, &a.Message, &a.Metadata,
			&a.ReferenceType, &a.ReferenceID, &a.IsRead, &a.IsDismissed, &a.AlertDate, &a.CreatedAt,
		); err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}

	return alerts, rows.Err()
}

// MarkAlertRead marks an alert as read
func (r *Repository) MarkAlertRead(ctx context.Context, alertID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE alerts SET is_read = true, read_at = NOW() WHERE id = $1
	`, alertID)
	return err
}

// MarkAlertDismissed marks an alert as dismissed
func (r *Repository) MarkAlertDismissed(ctx context.Context, alertID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE alerts SET is_dismissed = true, dismissed_at = NOW() WHERE id = $1
	`, alertID)
	return err
}
