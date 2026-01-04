package balance

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AccountBalanceData holds the computed balance for an account
type AccountBalanceData struct {
	AccountID        uuid.UUID
	AccountName      string
	AccountType      int32
	CashBalanceCents int64
	InvestmentCents  int64
	Change24hCents   int64
	LastActivity     time.Time
	CurrencyCode     string
}

// DailyBalanceData holds a single day's balance
type DailyBalanceData struct {
	Date         time.Time
	BalanceCents int64
	ChangeCents  int64
	CurrencyCode string
}

// BalanceSummary holds aggregate stats
type BalanceSummary struct {
	TotalNetWorthCents   int64
	SafeToSpendCents     int64
	TotalInvestmentCents int64
	UpcomingBillsCents   int64
	IsEstimated          bool
	CurrencyCode         string
}

// Repository handles balance queries
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new balance repository
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// GetAccountBalances computes balance for each account by summing transactions
func (r *Repository) GetAccountBalances(ctx context.Context, userID uuid.UUID) ([]AccountBalanceData, error) {
	query := `
		WITH account_balances AS (
			SELECT 
				t.account_id,
				a.name AS account_name,
				a.type AS account_type,
				COALESCE(SUM(t.amount_minor), 0) AS total_balance,
				MAX(t.posted_at) AS last_activity
			FROM transactions t
			LEFT JOIN accounts a ON a.id = t.account_id
			WHERE t.user_id = $1
			GROUP BY t.account_id, a.name, a.type
		),
		daily_change AS (
			SELECT 
				account_id,
				COALESCE(SUM(amount_minor), 0) AS change_24h
			FROM transactions
			WHERE user_id = $1 
			  AND posted_at >= NOW() - INTERVAL '24 hours'
			GROUP BY account_id
		)
		SELECT 
			ab.account_id,
			COALESCE(ab.account_name, 'Unknown Account'),
			ab.account_type,
			ab.total_balance,
			COALESCE(dc.change_24h, 0),
			ab.last_activity
		FROM account_balances ab
		LEFT JOIN daily_change dc ON dc.account_id = ab.account_id
		ORDER BY ab.total_balance DESC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var balances []AccountBalanceData
	for rows.Next() {
		var b AccountBalanceData
		var accountIDStr *string    // Nullable - transactions may not have account
		var accountName *string     // Nullable
		var accountType *string     // Nullable string for enum
		var lastActivity *time.Time // Nullable
		err := rows.Scan(
			&accountIDStr,
			&accountName,
			&accountType,
			&b.CashBalanceCents,
			&b.Change24hCents,
			&lastActivity,
		)
		if err != nil {
			return nil, err
		}

		// Handle nullable fields
		if accountIDStr != nil {
			b.AccountID, _ = uuid.Parse(*accountIDStr)
		}
		if accountName != nil {
			b.AccountName = *accountName
		} else {
			b.AccountName = "Unknown Account"
		}
		if lastActivity != nil {
			b.LastActivity = *lastActivity
		}
		b.CurrencyCode = "EUR" // Default, could be fetched from account

		// Map account type enum to int32
		// 0 = unknown, 5 = investment
		if accountType != nil {
			switch *accountType {
			case "investment":
				b.AccountType = 5
			case "checking":
				b.AccountType = 1
			case "savings":
				b.AccountType = 2
			case "credit_card":
				b.AccountType = 3
			case "cash":
				b.AccountType = 4
			case "loan":
				b.AccountType = 6
			default:
				b.AccountType = 0
			}
		}

		// Separate cash vs investment based on account type
		// AccountType: 5 = INVESTMENT
		if b.AccountType == 5 {
			b.InvestmentCents = b.CashBalanceCents
			b.CashBalanceCents = 0
		}

		balances = append(balances, b)
	}

	return balances, nil
}

// GetTotalBalance computes the total balance across all accounts
func (r *Repository) GetTotalBalance(ctx context.Context, userID uuid.UUID) (int64, error) {
	query := `
		SELECT COALESCE(SUM(amount_minor), 0)
		FROM transactions
		WHERE user_id = $1
	`
	var total int64
	err := r.db.QueryRow(ctx, query, userID).Scan(&total)
	return total, err
}

// GetUpcomingBills sums the expected recurring subscription amounts
func (r *Repository) GetUpcomingBills(ctx context.Context, userID uuid.UUID) (int64, error) {
	// Sum recurring subscriptions expected in next 30 days
	query := `
		SELECT COALESCE(SUM(ABS(amount_minor)), 0)
		FROM recurring_subscriptions
		WHERE user_id = $1 
		  AND is_active = true
		  AND next_expected_at <= NOW() + INTERVAL '30 days'
	`
	var total int64
	err := r.db.QueryRow(ctx, query, userID).Scan(&total)
	if err != nil {
		// Table might not exist yet, return 0
		return 0, nil
	}
	return total, nil
}

// GetBalanceHistory returns daily balance snapshots
func (r *Repository) GetBalanceHistory(ctx context.Context, userID uuid.UUID, days int) ([]DailyBalanceData, error) {
	query := `
		WITH RECURSIVE dates AS (
			SELECT CURRENT_DATE - ($2::integer) + 1 AS date
			UNION ALL
			SELECT date + 1 FROM dates WHERE date < CURRENT_DATE
		),
		daily_totals AS (
			SELECT 
				DATE(posted_at) AS date,
				SUM(amount_minor) AS daily_sum
			FROM transactions
			WHERE user_id = $1
			  AND posted_at >= CURRENT_DATE - ($2::integer)
			GROUP BY DATE(posted_at)
		),
		running_balance AS (
			SELECT 
				d.date,
				COALESCE(dt.daily_sum, 0) AS daily_change,
				SUM(COALESCE(dt.daily_sum, 0)) OVER (ORDER BY d.date) AS balance
			FROM dates d
			LEFT JOIN daily_totals dt ON dt.date = d.date
		)
		SELECT date, balance, daily_change
		FROM running_balance
		ORDER BY date
	`

	rows, err := r.db.Query(ctx, query, userID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []DailyBalanceData
	for rows.Next() {
		var d DailyBalanceData
		err := rows.Scan(&d.Date, &d.BalanceCents, &d.ChangeCents)
		if err != nil {
			return nil, err
		}
		d.CurrencyCode = "EUR"
		history = append(history, d)
	}

	return history, nil
}

// GetBalanceStats computes summary statistics for a period
func (r *Repository) GetBalanceStats(ctx context.Context, userID uuid.UUID, days int) (highest, lowest, average int64, err error) {
	query := `
		WITH daily_balances AS (
			SELECT 
				DATE(posted_at) AS date,
				SUM(SUM(amount_minor)) OVER (ORDER BY DATE(posted_at)) AS running_balance
			FROM transactions
			WHERE user_id = $1
			  AND posted_at >= CURRENT_DATE - ($2::integer)
			GROUP BY DATE(posted_at)
		)
		SELECT 
			COALESCE(MAX(running_balance), 0),
			COALESCE(MIN(running_balance), 0),
			COALESCE(AVG(running_balance), 0)::bigint
		FROM daily_balances
	`
	err = r.db.QueryRow(ctx, query, userID, days).Scan(&highest, &lowest, &average)
	return
}
