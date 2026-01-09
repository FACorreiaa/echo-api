// Package handler provides gRPC handlers for budget periods
package handler

import (
	"context"
	"errors"

	echov1 "buf.build/gen/go/echo-tracker/echo/protocolbuffers/go/echo/v1"
	"connectrpc.com/connect"
	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/plan/repository"
	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/plan/service"
	"github.com/FACorreiaa/smart-finance-tracker/pkg/interceptors"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// BudgetPeriodHandler handles budget period RPCs
type BudgetPeriodHandler struct {
	svc *service.BudgetPeriodService
}

// NewBudgetPeriodHandler creates a new handler
func NewBudgetPeriodHandler(svc *service.BudgetPeriodService) *BudgetPeriodHandler {
	return &BudgetPeriodHandler{svc: svc}
}

// GetBudgetPeriod gets or creates a budget period for a specific month
func (h *BudgetPeriodHandler) GetBudgetPeriod(ctx context.Context, req *connect.Request[echov1.GetBudgetPeriodRequest]) (*connect.Response[echov1.GetBudgetPeriodResponse], error) {
	_, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user not authenticated"))
	}

	planID, err := uuid.Parse(req.Msg.PlanId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid plan ID"))
	}

	period, wasCreated, err := h.svc.GetOrCreatePeriod(ctx, planID, int(req.Msg.Year), int(req.Msg.Month))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&echov1.GetBudgetPeriodResponse{
		Period:     toProtoBudgetPeriod(period),
		WasCreated: wasCreated,
	}), nil
}

// ListBudgetPeriods lists all periods for a plan
func (h *BudgetPeriodHandler) ListBudgetPeriods(ctx context.Context, req *connect.Request[echov1.ListBudgetPeriodsRequest]) (*connect.Response[echov1.ListBudgetPeriodsResponse], error) {
	_, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user not authenticated"))
	}

	planID, err := uuid.Parse(req.Msg.PlanId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid plan ID"))
	}

	periods, err := h.svc.ListPeriods(ctx, planID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var protoPeriodsWithItems []*echov1.BudgetPeriod
	for _, p := range periods {
		// Get items for each period
		periodWithItems, err := h.svc.GetPeriodByID(ctx, p.ID)
		if err != nil {
			continue
		}
		protoPeriodsWithItems = append(protoPeriodsWithItems, toProtoBudgetPeriod(periodWithItems))
	}

	return connect.NewResponse(&echov1.ListBudgetPeriodsResponse{
		Periods: protoPeriodsWithItems,
	}), nil
}

// UpdateBudgetPeriodItem updates a specific item's values
func (h *BudgetPeriodHandler) UpdateBudgetPeriodItem(ctx context.Context, req *connect.Request[echov1.UpdateBudgetPeriodItemRequest]) (*connect.Response[echov1.UpdateBudgetPeriodItemResponse], error) {
	_, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user not authenticated"))
	}

	periodItemID, err := uuid.Parse(req.Msg.PeriodItemId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid period item ID"))
	}

	var budgeted, actual *int64
	var notes *string

	if req.Msg.BudgetedMinor != nil {
		b := *req.Msg.BudgetedMinor
		budgeted = &b
	}
	if req.Msg.ActualMinor != nil {
		a := *req.Msg.ActualMinor
		actual = &a
	}
	if req.Msg.Notes != nil {
		n := *req.Msg.Notes
		notes = &n
	}

	item, err := h.svc.UpdatePeriodItem(ctx, periodItemID, budgeted, actual, notes)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&echov1.UpdateBudgetPeriodItemResponse{
		Item: toProtoBudgetPeriodItem(item),
	}), nil
}

// CopyBudgetPeriod copies values from one period to another
func (h *BudgetPeriodHandler) CopyBudgetPeriod(ctx context.Context, req *connect.Request[echov1.CopyBudgetPeriodRequest]) (*connect.Response[echov1.CopyBudgetPeriodResponse], error) {
	_, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user not authenticated"))
	}

	sourcePeriodID, err := uuid.Parse(req.Msg.SourcePeriodId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid source period ID"))
	}

	targetPlanID, err := uuid.Parse(req.Msg.TargetPlanId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid target plan ID"))
	}

	period, err := h.svc.CopyPeriodItems(ctx, sourcePeriodID, targetPlanID, int(req.Msg.TargetYear), int(req.Msg.TargetMonth))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&echov1.CopyBudgetPeriodResponse{
		Period: toProtoBudgetPeriod(period),
	}), nil
}

// ============================================================================
// Conversion helpers
// ============================================================================

func toProtoBudgetPeriod(p *repository.BudgetPeriodWithItems) *echov1.BudgetPeriod {
	if p == nil || p.Period == nil {
		return nil
	}

	proto := &echov1.BudgetPeriod{
		Id:        p.Period.ID.String(),
		PlanId:    p.Period.PlanID.String(),
		Year:      int32(p.Period.Year),
		Month:     int32(p.Period.Month),
		IsLocked:  p.Period.IsLocked,
		CreatedAt: timestamppb.New(p.Period.CreatedAt),
		UpdatedAt: timestamppb.New(p.Period.UpdatedAt),
	}

	if p.Period.Notes != nil {
		proto.Notes = *p.Period.Notes
	}

	for _, item := range p.Items {
		proto.Items = append(proto.Items, toProtoBudgetPeriodItem(item))
	}

	return proto
}

func toProtoBudgetPeriodItem(item *repository.BudgetPeriodItem) *echov1.BudgetPeriodItem {
	if item == nil {
		return nil
	}

	proto := &echov1.BudgetPeriodItem{
		Id:            item.ID.String(),
		PeriodId:      item.PeriodID.String(),
		ItemId:        item.ItemID.String(),
		ItemName:      item.ItemName,
		CategoryName:  item.CategoryName,
		BudgetedMinor: item.BudgetedMinor,
		ActualMinor:   item.ActualMinor,
	}

	if item.Notes != nil {
		proto.Notes = *item.Notes
	}

	return proto
}
