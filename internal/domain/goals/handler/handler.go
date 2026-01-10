// Package handler implements the Goals Connect RPC handlers.
package handler

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	echov1 "buf.build/gen/go/echo-tracker/echo/protocolbuffers/go/echo/v1"
	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/goals/repository"
	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/goals/service"
	"github.com/FACorreiaa/smart-finance-tracker/pkg/interceptors"
)

// GoalsHandler implements the goal-related Connect handlers
type GoalsHandler struct {
	svc *service.Service
}

// NewGoalsHandler constructs a new handler
func NewGoalsHandler(svc *service.Service) *GoalsHandler {
	return &GoalsHandler{svc: svc}
}

// CreateGoal creates a new goal
func (h *GoalsHandler) CreateGoal(
	ctx context.Context,
	req *connect.Request[echov1.CreateGoalRequest],
) (*connect.Response[echov1.CreateGoalResponse], error) {
	userID, err := getUserID(ctx)
	if err != nil {
		return nil, err
	}

	goalType := protoToGoalType(req.Msg.Type)
	currency := "EUR"
	if req.Msg.Target != nil && req.Msg.Target.CurrencyCode != "" {
		currency = req.Msg.Target.CurrencyCode
	}

	goal, err := h.svc.CreateGoal(
		ctx,
		userID,
		req.Msg.Name,
		goalType,
		req.Msg.Target.AmountMinor,
		currency,
		req.Msg.StartAt.AsTime(),
		req.Msg.EndAt.AsTime(),
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&echov1.CreateGoalResponse{
		Goal: goalToProto(goal),
	}), nil
}

// GetGoal retrieves a goal by ID
func (h *GoalsHandler) GetGoal(
	ctx context.Context,
	req *connect.Request[echov1.GetGoalRequest],
) (*connect.Response[echov1.GetGoalResponse], error) {
	_, err := getUserID(ctx)
	if err != nil {
		return nil, err
	}

	goalID, err := uuid.Parse(req.Msg.GoalId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid goal ID"))
	}

	goal, err := h.svc.GetGoal(ctx, goalID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("goal not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Get progress to populate computed fields
	progress, err := h.svc.GetGoalProgress(ctx, goalID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&echov1.GetGoalResponse{
		Goal: goalWithProgressToProto(goal, progress),
	}), nil
}

// UpdateGoal updates a goal
func (h *GoalsHandler) UpdateGoal(
	ctx context.Context,
	req *connect.Request[echov1.UpdateGoalRequest],
) (*connect.Response[echov1.UpdateGoalResponse], error) {
	_, err := getUserID(ctx)
	if err != nil {
		return nil, err
	}

	goalID, err := uuid.Parse(req.Msg.GoalId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid goal ID"))
	}

	var name *string
	if req.Msg.Name != nil {
		name = req.Msg.Name
	}

	var targetMinor *int64
	if req.Msg.Target != nil {
		targetMinor = &req.Msg.Target.AmountMinor
	}

	var endAt *time.Time
	if req.Msg.EndAt != nil {
		t := req.Msg.EndAt.AsTime()
		endAt = &t
	}

	var status *repository.GoalStatus
	if req.Msg.Status != nil {
		s := protoToGoalStatus(*req.Msg.Status)
		status = &s
	}

	goal, err := h.svc.UpdateGoal(ctx, goalID, name, targetMinor, endAt, status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("goal not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&echov1.UpdateGoalResponse{
		Goal: goalToProto(goal),
	}), nil
}

// DeleteGoal removes a goal
func (h *GoalsHandler) DeleteGoal(
	ctx context.Context,
	req *connect.Request[echov1.DeleteGoalRequest],
) (*connect.Response[echov1.DeleteGoalResponse], error) {
	_, err := getUserID(ctx)
	if err != nil {
		return nil, err
	}

	goalID, err := uuid.Parse(req.Msg.GoalId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid goal ID"))
	}

	if err := h.svc.DeleteGoal(ctx, goalID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("goal not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&echov1.DeleteGoalResponse{}), nil
}

// ListGoals retrieves all goals for a user
func (h *GoalsHandler) ListGoals(
	ctx context.Context,
	req *connect.Request[echov1.ListGoalsRequest],
) (*connect.Response[echov1.ListGoalsResponse], error) {
	userID, err := getUserID(ctx)
	if err != nil {
		return nil, err
	}

	var statusFilter *repository.GoalStatus
	if req.Msg.StatusFilter != nil {
		s := protoToGoalStatus(*req.Msg.StatusFilter)
		statusFilter = &s
	}

	goals, err := h.svc.ListGoals(ctx, userID, statusFilter)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoGoals := make([]*echov1.Goal, 0, len(goals))
	for _, goal := range goals {
		progress, _ := h.svc.GetGoalProgress(ctx, goal.ID)
		protoGoals = append(protoGoals, goalWithProgressToProto(goal, progress))
	}

	return connect.NewResponse(&echov1.ListGoalsResponse{
		Goals: protoGoals,
	}), nil
}

// GetGoalProgress returns detailed progress for a goal
func (h *GoalsHandler) GetGoalProgress(
	ctx context.Context,
	req *connect.Request[echov1.GetGoalProgressRequest],
) (*connect.Response[echov1.GetGoalProgressResponse], error) {
	_, err := getUserID(ctx)
	if err != nil {
		return nil, err
	}

	goalID, err := uuid.Parse(req.Msg.GoalId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid goal ID"))
	}

	progress, err := h.svc.GetGoalProgress(ctx, goalID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("goal not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &echov1.GetGoalProgressResponse{
		Goal:                  goalWithProgressToProto(progress.Goal, progress),
		NeedsAttention:        progress.NeedsAttention,
		NudgeMessage:          progress.NudgeMessage,
		SuggestedContribution: toMoney(progress.SuggestedContribution, progress.Goal.CurrencyCode),
	}

	// Add milestones
	for _, m := range progress.Milestones {
		protoMilestone := &echov1.GoalMilestone{
			Percent:    int32(m.Percent),
			Reached:    m.Reached,
			ExpectedBy: timestamppb.New(m.ExpectedBy),
		}
		if m.ReachedAt != nil {
			protoMilestone.ReachedAt = timestamppb.New(*m.ReachedAt)
		}
		resp.Milestones = append(resp.Milestones, protoMilestone)
	}

	// Add recent contributions
	for _, c := range progress.RecentContributions {
		protoContrib := &echov1.GoalContribution{
			Id:            c.ID.String(),
			Amount:        toMoney(c.AmountMinor, c.CurrencyCode),
			ContributedAt: timestamppb.New(c.ContributedAt),
		}
		if c.Note != nil {
			protoContrib.Note = c.Note
		}
		if c.TransactionID != nil {
			txID := c.TransactionID.String()
			protoContrib.TransactionId = &txID
		}
		resp.RecentContributions = append(resp.RecentContributions, protoContrib)
	}

	return connect.NewResponse(resp), nil
}

// ContributeToGoal adds a contribution to a goal
func (h *GoalsHandler) ContributeToGoal(
	ctx context.Context,
	req *connect.Request[echov1.ContributeToGoalRequest],
) (*connect.Response[echov1.ContributeToGoalResponse], error) {
	_, err := getUserID(ctx)
	if err != nil {
		return nil, err
	}

	goalID, err := uuid.Parse(req.Msg.GoalId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid goal ID"))
	}

	currency := "EUR"
	if req.Msg.Amount != nil && req.Msg.Amount.CurrencyCode != "" {
		currency = req.Msg.Amount.CurrencyCode
	}

	var note *string
	if req.Msg.Note != nil {
		note = req.Msg.Note
	}

	progress, milestone, err := h.svc.ContributeToGoal(ctx, goalID, req.Msg.Amount.AmountMinor, currency, note)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("goal not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &echov1.ContributeToGoalResponse{
		Goal: goalWithProgressToProto(progress.Goal, progress),
	}

	// Add latest contribution
	if len(progress.RecentContributions) > 0 {
		c := progress.RecentContributions[0]
		resp.Contribution = &echov1.GoalContribution{
			Id:            c.ID.String(),
			Amount:        toMoney(c.AmountMinor, c.CurrencyCode),
			ContributedAt: timestamppb.New(c.ContributedAt),
		}
		if c.Note != nil {
			resp.Contribution.Note = c.Note
		}
	}

	// Check milestone
	if milestone != nil {
		resp.MilestoneReached = true
		pct := int32(milestone.Percent)
		resp.MilestonePercent = &pct
		resp.FeedbackMessage = milestone.Message
	} else {
		resp.FeedbackMessage = "Contribution added successfully!"
	}

	return connect.NewResponse(resp), nil
}

// Helper functions

func getUserID(ctx context.Context) (uuid.UUID, error) {
	userIDStr, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		return uuid.Nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, connect.NewError(connect.CodeInternal, errors.New("invalid user ID"))
	}
	return userID, nil
}

func goalToProto(goal *repository.Goal) *echov1.Goal {
	return &echov1.Goal{
		Id:                 goal.ID.String(),
		UserId:             goal.UserID.String(),
		Name:               goal.Name,
		Type:               goalTypeToProto(goal.Type),
		Status:             goalStatusToProto(goal.Status),
		Target:             toMoney(goal.TargetAmountMinor, goal.CurrencyCode),
		CurrentAmountMinor: goal.CurrentAmountMinor,
		StartAt:            timestamppb.New(goal.StartAt),
		EndAt:              timestamppb.New(goal.EndAt),
		CreatedAt:          timestamppb.New(goal.CreatedAt),
		UpdatedAt:          timestamppb.New(goal.UpdatedAt),
	}
}

func goalWithProgressToProto(goal *repository.Goal, progress *service.GoalProgress) *echov1.Goal {
	protoGoal := goalToProto(goal)
	if progress != nil {
		protoGoal.ProgressPercent = progress.ProgressPercent
		protoGoal.PacePercent = progress.PacePercent
		protoGoal.IsBehindPace = progress.IsBehindPace
		protoGoal.PaceMessage = progress.PaceMessage
		protoGoal.DaysRemaining = int32(progress.DaysRemaining)
		protoGoal.AmountNeededPerDay = toMoney(progress.AmountNeededPerDay, goal.CurrencyCode)
	}
	return protoGoal
}

func toMoney(cents int64, currency string) *echov1.Money {
	if currency == "" {
		currency = "EUR"
	}
	return &echov1.Money{
		AmountMinor:  cents,
		CurrencyCode: currency,
	}
}

func goalTypeToProto(t repository.GoalType) echov1.GoalType {
	switch t {
	case repository.GoalTypeSave:
		return echov1.GoalType_GOAL_TYPE_SAVE
	case repository.GoalTypePayDownDebt:
		return echov1.GoalType_GOAL_TYPE_PAY_DOWN_DEBT
	case repository.GoalTypeSpendCap:
		return echov1.GoalType_GOAL_TYPE_SPEND_CAP
	default:
		return echov1.GoalType_GOAL_TYPE_UNSPECIFIED
	}
}

func protoToGoalType(t echov1.GoalType) repository.GoalType {
	switch t {
	case echov1.GoalType_GOAL_TYPE_SAVE:
		return repository.GoalTypeSave
	case echov1.GoalType_GOAL_TYPE_PAY_DOWN_DEBT:
		return repository.GoalTypePayDownDebt
	case echov1.GoalType_GOAL_TYPE_SPEND_CAP:
		return repository.GoalTypeSpendCap
	default:
		return repository.GoalTypeSave
	}
}

func goalStatusToProto(s repository.GoalStatus) echov1.GoalStatus {
	switch s {
	case repository.GoalStatusActive:
		return echov1.GoalStatus_GOAL_STATUS_ACTIVE
	case repository.GoalStatusPaused:
		return echov1.GoalStatus_GOAL_STATUS_PAUSED
	case repository.GoalStatusCompleted:
		return echov1.GoalStatus_GOAL_STATUS_COMPLETED
	case repository.GoalStatusArchived:
		return echov1.GoalStatus_GOAL_STATUS_ARCHIVED
	default:
		return echov1.GoalStatus_GOAL_STATUS_UNSPECIFIED
	}
}

func protoToGoalStatus(s echov1.GoalStatus) repository.GoalStatus {
	switch s {
	case echov1.GoalStatus_GOAL_STATUS_ACTIVE:
		return repository.GoalStatusActive
	case echov1.GoalStatus_GOAL_STATUS_PAUSED:
		return repository.GoalStatusPaused
	case echov1.GoalStatus_GOAL_STATUS_COMPLETED:
		return repository.GoalStatusCompleted
	case echov1.GoalStatus_GOAL_STATUS_ARCHIVED:
		return repository.GoalStatusArchived
	default:
		return repository.GoalStatusActive
	}
}
