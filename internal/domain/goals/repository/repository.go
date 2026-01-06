// Package repository provides database operations for goals.
package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// GoalType represents the type of goal
type GoalType string

const (
	GoalTypeSave        GoalType = "save"
	GoalTypePayDownDebt GoalType = "pay_down_debt"
	GoalTypeSpendCap    GoalType = "spend_cap"
)

// GoalStatus represents the status of a goal
type GoalStatus string

const (
	GoalStatusActive    GoalStatus = "active"
	GoalStatusPaused    GoalStatus = "paused"
	GoalStatusCompleted GoalStatus = "completed"
	GoalStatusArchived  GoalStatus = "archived"
)

// Goal represents a financial goal
type Goal struct {
	ID                 uuid.UUID
	UserID             uuid.UUID
	Name               string
	Type               GoalType
	Status             GoalStatus
	TargetAmountMinor  int64
	CurrencyCode       string
	CurrentAmountMinor int64
	StartAt            time.Time
	EndAt              time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// GoalContribution represents a contribution to a goal
type GoalContribution struct {
	ID            uuid.UUID
	GoalID        uuid.UUID
	AmountMinor   int64
	CurrencyCode  string
	Note          *string
	TransactionID *uuid.UUID
	ContributedAt time.Time
	CreatedAt     time.Time
}

// GoalRepository defines the interface for goal persistence operations
type GoalRepository interface {
	// CRUD operations
	Create(ctx context.Context, goal *Goal) error
	GetByID(ctx context.Context, id uuid.UUID) (*Goal, error)
	Update(ctx context.Context, goal *Goal) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByUserID(ctx context.Context, userID uuid.UUID, statusFilter *GoalStatus) ([]*Goal, error)

	// Contribution operations
	AddContribution(ctx context.Context, contribution *GoalContribution) error
	ListContributions(ctx context.Context, goalID uuid.UUID, limit int) ([]*GoalContribution, error)

	// Progress operations
	UpdateCurrentAmount(ctx context.Context, goalID uuid.UUID, amountMinor int64) error
}
