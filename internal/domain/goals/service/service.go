// Package service provides business logic for goals management.
package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/goals/repository"
)

// GoalProgress contains calculated progress information
type GoalProgress struct {
	Goal                  *repository.Goal
	ProgressPercent       float64 // 0-100, current/target
	PacePercent           float64 // 100 = on track, <100 = behind
	IsBehindPace          bool
	PaceMessage           string
	DaysRemaining         int
	AmountNeededPerDay    int64 // Cents needed per day to reach goal on time
	Milestones            []Milestone
	RecentContributions   []*repository.GoalContribution
	NeedsAttention        bool
	NudgeMessage          string
	SuggestedContribution int64 // Recommended next contribution
}

// Milestone represents a progress checkpoint
type Milestone struct {
	Percent    int // 25, 50, 75, 100
	Reached    bool
	ReachedAt  *time.Time
	ExpectedBy time.Time
}

// Service provides goal management business logic
type Service struct {
	repo repository.GoalRepository
}

// NewService creates a new goals service
func NewService(repo repository.GoalRepository) *Service {
	return &Service{repo: repo}
}

// CreateGoal creates a new goal
func (s *Service) CreateGoal(ctx context.Context, userID uuid.UUID, name string, goalType repository.GoalType, targetMinor int64, currency string, startAt, endAt time.Time) (*repository.Goal, error) {
	if endAt.Before(startAt) {
		return nil, fmt.Errorf("end date must be after start date")
	}
	if targetMinor <= 0 {
		return nil, fmt.Errorf("target amount must be positive")
	}

	goal := &repository.Goal{
		ID:                 uuid.New(),
		UserID:             userID,
		Name:               name,
		Type:               goalType,
		Status:             repository.GoalStatusActive,
		TargetAmountMinor:  targetMinor,
		CurrencyCode:       currency,
		CurrentAmountMinor: 0,
		StartAt:            startAt,
		EndAt:              endAt,
	}

	if err := s.repo.Create(ctx, goal); err != nil {
		return nil, err
	}
	return goal, nil
}

// GetGoal retrieves a goal by ID
func (s *Service) GetGoal(ctx context.Context, goalID uuid.UUID) (*repository.Goal, error) {
	return s.repo.GetByID(ctx, goalID)
}

// UpdateGoal updates a goal
func (s *Service) UpdateGoal(ctx context.Context, goalID uuid.UUID, name *string, targetMinor *int64, endAt *time.Time, status *repository.GoalStatus) (*repository.Goal, error) {
	goal, err := s.repo.GetByID(ctx, goalID)
	if err != nil {
		return nil, err
	}

	if name != nil {
		goal.Name = *name
	}
	if targetMinor != nil {
		goal.TargetAmountMinor = *targetMinor
	}
	if endAt != nil {
		goal.EndAt = *endAt
	}
	if status != nil {
		goal.Status = *status
	}

	if err := s.repo.Update(ctx, goal); err != nil {
		return nil, err
	}
	return goal, nil
}

// DeleteGoal removes a goal
func (s *Service) DeleteGoal(ctx context.Context, goalID uuid.UUID) error {
	return s.repo.Delete(ctx, goalID)
}

// ListGoals retrieves all goals for a user
func (s *Service) ListGoals(ctx context.Context, userID uuid.UUID, statusFilter *repository.GoalStatus) ([]*repository.Goal, error) {
	return s.repo.ListByUserID(ctx, userID, statusFilter)
}

// GetGoalProgress calculates detailed progress for a goal
func (s *Service) GetGoalProgress(ctx context.Context, goalID uuid.UUID) (*GoalProgress, error) {
	goal, err := s.repo.GetByID(ctx, goalID)
	if err != nil {
		return nil, err
	}

	contributions, err := s.repo.ListContributions(ctx, goalID, 10)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	progress := &GoalProgress{
		Goal:                goal,
		RecentContributions: contributions,
	}

	// Calculate progress percent
	if goal.TargetAmountMinor > 0 {
		progress.ProgressPercent = float64(goal.CurrentAmountMinor) / float64(goal.TargetAmountMinor) * 100
		if progress.ProgressPercent > 100 {
			progress.ProgressPercent = 100
		}
	}

	// Calculate days remaining
	if now.Before(goal.EndAt) {
		progress.DaysRemaining = int(goal.EndAt.Sub(now).Hours() / 24)
	}

	// Calculate pace
	totalDays := goal.EndAt.Sub(goal.StartAt).Hours() / 24
	elapsedDays := now.Sub(goal.StartAt).Hours() / 24
	if elapsedDays < 0 {
		elapsedDays = 0
	}
	if elapsedDays > totalDays {
		elapsedDays = totalDays
	}

	expectedProgress := 0.0
	if totalDays > 0 {
		expectedProgress = (elapsedDays / totalDays) * 100
	}

	if expectedProgress > 0 {
		progress.PacePercent = (progress.ProgressPercent / expectedProgress) * 100
	} else {
		progress.PacePercent = 100 // Before start date, consider on pace
	}

	progress.IsBehindPace = progress.PacePercent < 100

	// Calculate amount needed per day
	remainingAmount := goal.TargetAmountMinor - goal.CurrentAmountMinor
	if remainingAmount < 0 {
		remainingAmount = 0
	}
	if progress.DaysRemaining > 0 {
		progress.AmountNeededPerDay = remainingAmount / int64(progress.DaysRemaining)
	}

	// Generate pace message
	progress.PaceMessage = s.generatePaceMessage(progress)

	// Calculate milestones
	progress.Milestones = s.calculateMilestones(goal, now)

	// Generate nudge if needed
	progress.NeedsAttention, progress.NudgeMessage, progress.SuggestedContribution = s.generateNudge(progress, now)

	return progress, nil
}

func (s *Service) generatePaceMessage(progress *GoalProgress) string {
	if progress.Goal.Status == repository.GoalStatusCompleted {
		return "Goal completed!"
	}

	if progress.ProgressPercent >= 100 {
		return "Goal reached!"
	}

	if progress.DaysRemaining <= 0 {
		if progress.ProgressPercent >= 100 {
			return "Goal completed!"
		}
		return fmt.Sprintf("Deadline passed (%.0f%% complete)", progress.ProgressPercent)
	}

	if progress.PacePercent >= 100 {
		aheadPercent := progress.PacePercent - 100
		if aheadPercent > 10 {
			return fmt.Sprintf("Ahead of schedule by %.0f%%", aheadPercent)
		}
		return "On track"
	}

	behindPercent := 100 - progress.PacePercent
	if behindPercent > 25 {
		weeksRemaining := progress.DaysRemaining / 7
		return fmt.Sprintf("Behind by %.0f%% (%d weeks left)", behindPercent, weeksRemaining)
	}
	return fmt.Sprintf("Slightly behind (%.0f%% of expected)", progress.PacePercent)
}

func (s *Service) calculateMilestones(goal *repository.Goal, now time.Time) []Milestone {
	milestones := make([]Milestone, 4)
	percents := []int{25, 50, 75, 100}
	totalDays := goal.EndAt.Sub(goal.StartAt).Hours() / 24

	for i, pct := range percents {
		milestoneAmount := (goal.TargetAmountMinor * int64(pct)) / 100
		expectedDays := (totalDays * float64(pct)) / 100
		expectedBy := goal.StartAt.Add(time.Duration(expectedDays*24) * time.Hour)

		milestones[i] = Milestone{
			Percent:    pct,
			Reached:    goal.CurrentAmountMinor >= milestoneAmount,
			ExpectedBy: expectedBy,
		}

		if milestones[i].Reached {
			// In a real implementation, we'd track when each milestone was reached
			t := now
			milestones[i].ReachedAt = &t
		}
	}
	return milestones
}

func (s *Service) generateNudge(progress *GoalProgress, now time.Time) (needsAttention bool, message string, suggestedAmount int64) {
	if progress.Goal.Status != repository.GoalStatusActive {
		return false, "", 0
	}

	if progress.ProgressPercent >= 100 {
		return false, "", 0
	}

	// Weekly contribution suggestion
	weeksRemaining := float64(progress.DaysRemaining) / 7
	if weeksRemaining < 1 {
		weeksRemaining = 1
	}

	remainingAmount := progress.Goal.TargetAmountMinor - progress.Goal.CurrentAmountMinor
	suggestedAmount = int64(math.Ceil(float64(remainingAmount) / weeksRemaining))

	// Determine if needs attention
	if progress.PacePercent < 75 {
		needsAttention = true
		message = fmt.Sprintf("Add €%.2f this week to get back on track", float64(suggestedAmount)/100)
	} else if progress.PacePercent < 90 {
		needsAttention = true
		message = fmt.Sprintf("Small boost needed: €%.2f this week", float64(suggestedAmount)/100)
	} else if progress.DaysRemaining <= 7 {
		needsAttention = true
		message = fmt.Sprintf("Final push: €%.2f to reach your goal", float64(remainingAmount)/100)
	}

	return needsAttention, message, suggestedAmount
}

// ContributeToGoal adds a contribution and returns milestone info
func (s *Service) ContributeToGoal(ctx context.Context, goalID uuid.UUID, amountMinor int64, currency string, note *string) (*GoalProgress, *MilestoneReached, error) {
	goal, err := s.repo.GetByID(ctx, goalID)
	if err != nil {
		return nil, nil, err
	}

	// Check for milestone before contribution
	previousAmount := goal.CurrentAmountMinor
	percentages := []int{25, 50, 75, 100}

	contribution := &repository.GoalContribution{
		ID:           uuid.New(),
		GoalID:       goalID,
		AmountMinor:  amountMinor,
		CurrencyCode: currency,
		Note:         note,
	}

	if err := s.repo.AddContribution(ctx, contribution); err != nil {
		return nil, nil, err
	}

	// Get updated progress
	progress, err := s.GetGoalProgress(ctx, goalID)
	if err != nil {
		return nil, nil, err
	}

	// Check if milestone was reached
	var milestoneReached *MilestoneReached
	newAmount := previousAmount + amountMinor
	for _, pct := range percentages {
		threshold := (goal.TargetAmountMinor * int64(pct)) / 100
		if previousAmount < threshold && newAmount >= threshold {
			milestoneReached = &MilestoneReached{
				Percent: pct,
				Message: s.getMilestoneMessage(pct),
			}
			break
		}
	}

	// Check if goal was completed
	if newAmount >= goal.TargetAmountMinor && goal.Status == repository.GoalStatusActive {
		status := repository.GoalStatusCompleted
		_, _ = s.UpdateGoal(ctx, goalID, nil, nil, nil, &status)
		progress.Goal.Status = repository.GoalStatusCompleted
	}

	return progress, milestoneReached, nil
}

// MilestoneReached contains info about a reached milestone
type MilestoneReached struct {
	Percent int
	Message string
}

func (s *Service) getMilestoneMessage(percent int) string {
	switch percent {
	case 25:
		return "Great start! You're 25% of the way there!"
	case 50:
		return "Halfway there! Keep up the momentum!"
	case 75:
		return "Amazing progress! Just 25% left to go!"
	case 100:
		return "Congratulations! You've reached your goal!"
	default:
		return fmt.Sprintf("You've reached %d%% of your goal!", percent)
	}
}

// GetGoalsBehindPace returns all goals that are behind schedule for nudge alerts
func (s *Service) GetGoalsBehindPace(ctx context.Context, userID uuid.UUID, behindThreshold float64) ([]*GoalProgress, error) {
	status := repository.GoalStatusActive
	goals, err := s.repo.ListByUserID(ctx, userID, &status)
	if err != nil {
		return nil, err
	}

	var behindGoals []*GoalProgress
	for _, goal := range goals {
		progress, err := s.GetGoalProgress(ctx, goal.ID)
		if err != nil {
			continue
		}
		if progress.IsBehindPace && progress.PacePercent < behindThreshold {
			behindGoals = append(behindGoals, progress)
		}
	}
	return behindGoals, nil
}
