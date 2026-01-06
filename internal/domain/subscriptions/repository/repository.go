// Package repository provides database operations for subscriptions.
package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// RecurringStatus represents the status of a subscription
type RecurringStatus string

const (
	RecurringStatusActive   RecurringStatus = "active"
	RecurringStatusPaused   RecurringStatus = "paused"
	RecurringStatusCanceled RecurringStatus = "canceled"
)

// RecurringCadence represents how often a subscription recurs
type RecurringCadence string

const (
	RecurringCadenceWeekly    RecurringCadence = "weekly"
	RecurringCadenceMonthly   RecurringCadence = "monthly"
	RecurringCadenceQuarterly RecurringCadence = "quarterly"
	RecurringCadenceAnnual    RecurringCadence = "annual"
	RecurringCadenceUnknown   RecurringCadence = "unknown"
)

// RecurringSubscription represents a detected recurring charge
type RecurringSubscription struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	MerchantName    string
	AmountMinor     int64
	CurrencyCode    string
	Cadence         RecurringCadence
	Status          RecurringStatus
	FirstSeenAt     *time.Time
	LastSeenAt      *time.Time
	NextExpectedAt  *time.Time
	OccurrenceCount int
	CategoryID      *uuid.UUID
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// MerchantTransactionGroup represents grouped transactions for a merchant
type MerchantTransactionGroup struct {
	MerchantName    string
	TotalAmount     int64
	TransactionDates []time.Time
	AmountPerTx     []int64
	CategoryID      *uuid.UUID
}

// SubscriptionRepository defines the interface for subscription persistence
type SubscriptionRepository interface {
	// CRUD operations
	Create(ctx context.Context, sub *RecurringSubscription) error
	GetByID(ctx context.Context, id uuid.UUID) (*RecurringSubscription, error)
	Update(ctx context.Context, sub *RecurringSubscription) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByUserID(ctx context.Context, userID uuid.UUID, statusFilter *RecurringStatus, includeCanceled bool) ([]*RecurringSubscription, error)

	// Detection
	GetByUserAndMerchant(ctx context.Context, userID uuid.UUID, merchantName string) (*RecurringSubscription, error)
	GetMerchantTransactionGroups(ctx context.Context, userID uuid.UUID, since time.Time, minOccurrences int) ([]*MerchantTransactionGroup, error)

	// Status management
	UpdateStatus(ctx context.Context, id uuid.UUID, status RecurringStatus) error
	IncrementOccurrence(ctx context.Context, id uuid.UUID, lastSeenAt time.Time) error
}
