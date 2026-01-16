// Package service provides business logic for waitlist management.
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"

	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/waitlist/repository"
	"github.com/google/uuid"
	"github.com/resend/resend-go/v2"
)

// WaitlistService handles waitlist business logic
type WaitlistService struct {
	repo         repository.WaitlistRepository
	resendClient *resend.Client
	logger       *slog.Logger
	fromEmail    string
}

// NewWaitlistService creates a new waitlist service
func NewWaitlistService(repo repository.WaitlistRepository, logger *slog.Logger) *WaitlistService {
	apiKey := os.Getenv("RESEND_API_KEY")
	var client *resend.Client
	if apiKey != "" {
		client = resend.NewClient(apiKey)
	}

	fromEmail := os.Getenv("RESEND_FROM_EMAIL")
	if fromEmail == "" {
		fromEmail = "Echo <hello@echo-os.com>"
	}

	return &WaitlistService{
		repo:         repo,
		resendClient: client,
		logger:       logger,
		fromEmail:    fromEmail,
	}
}

// AddToWaitlist adds an email to the waitlist and sends a confirmation email
func (s *WaitlistService) AddToWaitlist(ctx context.Context, email string) (*repository.WaitlistEntry, int, error) {
	// Add to database
	entry, err := s.repo.Add(ctx, email)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to add to waitlist: %w", err)
	}

	// Get queue position
	position, err := s.repo.GetPosition(ctx, entry.ID)
	if err != nil {
		s.logger.Warn("failed to get waitlist position", slog.Any("error", err))
		position = 0
	}

	// Send welcome email (async, don't block)
	go func() {
		if err := s.sendWelcomeEmail(entry.Email, position); err != nil {
			s.logger.Error("failed to send welcome email",
				slog.String("email", entry.Email),
				slog.Any("error", err),
			)
		}
	}()

	s.logger.Info("user added to waitlist",
		slog.String("email", entry.Email),
		slog.Int("position", position),
	)

	return entry, position, nil
}

// ListWaitlist returns paginated waitlist entries
func (s *WaitlistService) ListWaitlist(ctx context.Context, status *repository.WaitlistStatus, limit, offset int) ([]*repository.WaitlistEntry, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	return s.repo.List(ctx, status, limit, offset)
}

// SendInvite sends an early access invite email
func (s *WaitlistService) SendInvite(ctx context.Context, waitlistID uuid.UUID) (string, error) {
	// Get the entry
	entry, err := s.repo.GetByID(ctx, waitlistID)
	if err != nil {
		return "", fmt.Errorf("failed to get waitlist entry: %w", err)
	}
	if entry == nil {
		return "", fmt.Errorf("waitlist entry not found")
	}

	// Generate invite code
	inviteCode := generateInviteCode()

	// Update status
	if err := s.repo.UpdateStatus(ctx, waitlistID, repository.StatusInvited, &inviteCode); err != nil {
		return "", fmt.Errorf("failed to update status: %w", err)
	}

	// Send invite email
	if err := s.sendInviteEmail(entry.Email, inviteCode); err != nil {
		s.logger.Error("failed to send invite email",
			slog.String("email", entry.Email),
			slog.Any("error", err),
		)
		// Don't fail the request, status is already updated
	}

	s.logger.Info("invite sent",
		slog.String("email", entry.Email),
		slog.String("invite_code", inviteCode),
	)

	return inviteCode, nil
}

// GetStats returns waitlist metrics
func (s *WaitlistService) GetStats(ctx context.Context) (*repository.WaitlistStats, error) {
	return s.repo.GetStats(ctx)
}

// ValidateInviteCode checks if an invite code is valid and marks it as used
func (s *WaitlistService) ValidateInviteCode(ctx context.Context, code string) (*repository.WaitlistEntry, error) {
	entry, err := s.repo.GetByInviteCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil // Invalid code
	}
	if entry.Status == repository.StatusJoined {
		return nil, fmt.Errorf("invite code already used")
	}

	// Mark as joined
	if err := s.repo.UpdateStatus(ctx, entry.ID, repository.StatusJoined, nil); err != nil {
		return nil, err
	}

	return entry, nil
}

// sendWelcomeEmail sends the "You're on the waitlist" email
func (s *WaitlistService) sendWelcomeEmail(email string, position int) error {
	if s.resendClient == nil {
		s.logger.Warn("resend client not configured, skipping welcome email")
		return nil
	}

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
  <style>
    body { background-color: #050505; font-family: 'Outfit', sans-serif; margin: 0; padding: 40px 0; }
    .container { background-color: #0A0A0B; border: 1px solid rgba(255,255,255,0.1); border-radius: 12px; padding: 40px; max-width: 480px; margin: 0 auto; }
    .topLabel { color: #2da6fa; font-size: 12px; font-weight: 700; letter-spacing: 2px; text-align: center; }
    h1 { color: #ffffff; font-size: 28px; font-weight: 900; text-align: center; margin: 20px 0; }
    .text { color: #9ca3af; font-size: 16px; line-height: 24px; text-align: center; }
    .position { background: rgba(255,255,255,0.05); border-radius: 8px; padding: 20px; margin: 30px 0; text-align: center; }
    .positionLabel { color: #636366; font-size: 10px; font-weight: 700; letter-spacing: 1px; }
    .positionNumber { color: #ffffff; font-size: 48px; font-weight: 900; margin: 10px 0; }
    .footer { color: #636366; font-size: 12px; text-align: center; margin-top: 30px; }
  </style>
</head>
<body>
  <div class="container">
    <p class="topLabel">WAITLIST CONFIRMED</p>
    <h1>You're on the list.</h1>
    <p class="text">Thank you for joining the Echo waitlist. You'll be among the first to experience the future of personal finance.</p>
    <div class="position">
      <p class="positionLabel">YOUR POSITION</p>
      <p class="positionNumber">#%d</p>
    </div>
    <p class="footer">We'll notify you when it's your turn. Welcome to sovereignty.</p>
  </div>
</body>
</html>
`, position)

	_, err := s.resendClient.Emails.Send(&resend.SendEmailRequest{
		From:    s.fromEmail,
		To:      []string{email},
		Subject: "You're on the Echo waitlist! 🎉",
		Html:    html,
	})
	return err
}

// sendInviteEmail sends the "You're invited" email with activation code
func (s *WaitlistService) sendInviteEmail(email, inviteCode string) error {
	if s.resendClient == nil {
		s.logger.Warn("resend client not configured, skipping invite email")
		return nil
	}

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
  <style>
    body { background-color: #050505; font-family: 'Outfit', sans-serif; margin: 0; padding: 40px 0; }
    .container { background-color: #0A0A0B; border: 1px solid rgba(255,255,255,0.1); border-radius: 12px; padding: 40px; max-width: 480px; margin: 0 auto; }
    .topLabel { color: #2da6fa; font-size: 12px; font-weight: 700; letter-spacing: 2px; text-align: center; }
    h1 { color: #ffffff; font-size: 28px; font-weight: 900; text-align: center; margin: 20px 0; }
    .text { color: #9ca3af; font-size: 16px; line-height: 24px; text-align: center; }
    .codeSection { background: rgba(255,255,255,0.05); border-radius: 8px; padding: 20px; margin: 30px 0; text-align: center; }
    .codeLabel { color: #636366; font-size: 10px; font-weight: 700; letter-spacing: 1px; }
    .codeText { color: #ffffff; font-size: 32px; font-weight: 900; letter-spacing: 4px; margin: 10px 0; }
    .button { background-color: #2da6fa; border-radius: 6px; color: #ffffff; font-size: 16px; font-weight: 700; text-decoration: none; text-align: center; display: block; padding: 12px 20px; margin: 20px auto; max-width: 200px; }
    .footer { color: #636366; font-size: 12px; text-align: center; margin-top: 30px; }
  </style>
</head>
<body>
  <div class="container">
    <p class="topLabel">INVITATION GRANTED</p>
    <h1>Welcome to the OS.</h1>
    <p class="text">The wait is over. You've been granted early access to Echo, the first truly "Alive" Money Operating System.</p>
    <div class="codeSection">
      <p class="codeLabel">YOUR ACTIVATION CODE</p>
      <p class="codeText">%s</p>
    </div>
    <a class="button" href="https://echo-os.com/activate?code=%s">Initialize Your Echo</a>
    <p class="footer">This link is unique to you and will expire in 48 hours. Step into sovereignty.</p>
  </div>
</body>
</html>
`, inviteCode, inviteCode)

	_, err := s.resendClient.Emails.Send(&resend.SendEmailRequest{
		From:    s.fromEmail,
		To:      []string{email},
		Subject: "Your Echo Invitation Has Arrived 🚀",
		Html:    html,
	})
	return err
}

// generateInviteCode creates a unique 8-character alphanumeric code
func generateInviteCode() string {
	bytes := make([]byte, 4)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
