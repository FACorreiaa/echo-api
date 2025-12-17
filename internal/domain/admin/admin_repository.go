package admin

import (
	"context"

	"github.com/google/uuid"

	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/common"
)

//revive:disable-next-line:exported
type AdminRepo interface {
	// GetUserByID retrieves a user's full profile by their unique ID.
	GetUserByID(ctx context.Context, userID uuid.UUID) (*common.UserProfile, error)
	// UpdateProfile updates mutable fields on a user's profile.
	// It takes the userID and a struct containing only the fields to be updated (use pointers).
	UpdateProfile(ctx context.Context, userID uuid.UUID, params common.UpdateProfileParams) error
	// DeactivateUser marks a user as inactive (soft delete).
	// Consider if this should also invalidate sessions/tokens.
	DeactivateUser(ctx context.Context, userID uuid.UUID) error
	// ReactivateUser marks a user as active.
	ReactivateUser(ctx context.Context, userID uuid.UUID) error
}
