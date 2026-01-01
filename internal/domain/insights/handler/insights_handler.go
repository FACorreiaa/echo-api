// Package handler implements the InsightsService Connect RPC handlers.
package handler

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	"buf.build/gen/go/echo-tracker/echo/connectrpc/go/echo/v1/echov1connect"
	echov1 "buf.build/gen/go/echo-tracker/echo/protocolbuffers/go/echo/v1"
	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/insights"
	"github.com/FACorreiaa/smart-finance-tracker/pkg/interceptors"
)

// InsightsHandler implements the InsightsService Connect handlers.
type InsightsHandler struct {
	echov1connect.UnimplementedInsightsServiceHandler
	svc *insights.Service
}

// NewInsightsHandler constructs a new handler.
func NewInsightsHandler(svc *insights.Service) *InsightsHandler {
	return &InsightsHandler{svc: svc}
}

// GetSpendingPulse returns spending pace comparison vs last month.
func (h *InsightsHandler) GetSpendingPulse(
	ctx context.Context,
	req *connect.Request[echov1.GetSpendingPulseRequest],
) (*connect.Response[echov1.GetSpendingPulseResponse], error) {
	userIDStr, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("invalid user ID in context"))
	}

	// Use provided as_of date or default to now
	asOf := time.Now()
	if req.Msg.AsOf != nil {
		asOf = req.Msg.AsOf.AsTime()
	}

	pulse, err := h.svc.GetSpendingPulse(ctx, userID, asOf)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Build proto response
	resp := &echov1.GetSpendingPulseResponse{
		Pulse: &echov1.SpendingPulse{
			CurrentMonthSpend: toMoney(pulse.CurrentMonthSpend),
			LastMonthSpend:    toMoney(pulse.LastMonthSpend),
			SpendDelta:        toMoney(pulse.SpendDelta),
			PacePercent:       pulse.PacePercent,
			IsOverPace:        pulse.IsOverPace,
			PaceMessage:       pulse.PaceMessage,
			DayOfMonth:        int32(pulse.DayOfMonth),
			TransactionCount:  int32(pulse.TransactionCount),
			AsOfDate:          timestamppb.New(pulse.AsOfDate),
		},
		ShouldNotify: h.svc.ShouldNotify(pulse),
	}

	// Add top categories
	for _, cat := range pulse.TopCategories {
		protoCat := &echov1.TopCategorySpend{
			CategoryName:     cat.CategoryName,
			Amount:           toMoney(cat.AmountCents),
			TransactionCount: int32(cat.TxCount),
		}
		if cat.CategoryID != nil {
			catIDStr := cat.CategoryID.String()
			protoCat.CategoryId = &catIDStr
		}
		resp.Pulse.TopCategories = append(resp.Pulse.TopCategories, protoCat)
	}

	// Add surprise expenses
	for _, exp := range pulse.SurpriseExpenses {
		protoExp := &echov1.SurpriseExpense{
			TransactionId: exp.TransactionID.String(),
			Description:   exp.Description,
			MerchantName:  exp.MerchantName,
			Amount:        toMoney(exp.AmountCents),
			PostedAt:      timestamppb.New(exp.PostedAt),
			CategoryName:  exp.CategoryName,
		}
		resp.Pulse.SurpriseExpenses = append(resp.Pulse.SurpriseExpenses, protoExp)
	}

	// Auto-trigger pace alert if user is overspending (fire and forget)
	if resp.ShouldNotify {
		go func() {
			_ = h.svc.TriggerPaceAlert(context.Background(), userID, pulse)
		}()
	}

	return connect.NewResponse(resp), nil
}

// GetDashboardBlocks returns bento-grid blocks for the dashboard.
func (h *InsightsHandler) GetDashboardBlocks(
	ctx context.Context,
	req *connect.Request[echov1.GetDashboardBlocksRequest],
) (*connect.Response[echov1.GetDashboardBlocksResponse], error) {
	userIDStr, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("invalid user ID in context"))
	}

	blocks, err := h.svc.GetDashboardBlocks(ctx, userID, time.Now())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoBlocks := make([]*echov1.DashboardBlock, 0, len(blocks))
	for _, b := range blocks {
		protoBlock := &echov1.DashboardBlock{
			Type:     b.Type,
			Title:    b.Title,
			Subtitle: b.Subtitle,
			Value:    b.Value,
			Icon:     b.Icon,
			Color:    b.Color,
		}
		if b.Action != "" {
			protoBlock.Action = &b.Action
		}
		protoBlocks = append(protoBlocks, protoBlock)
	}

	return connect.NewResponse(&echov1.GetDashboardBlocksResponse{
		Blocks: protoBlocks,
	}), nil
}

// toMoney converts cents to proto Money message
func toMoney(cents int64) *echov1.Money {
	return &echov1.Money{
		AmountMinor:  cents,
		CurrencyCode: "EUR", // Default, should come from user settings
	}
}

// ListAlerts returns alerts for the authenticated user.
func (h *InsightsHandler) ListAlerts(
	ctx context.Context,
	req *connect.Request[echov1.ListAlertsRequest],
) (*connect.Response[echov1.ListAlertsResponse], error) {
	userIDStr, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("invalid user ID in context"))
	}

	limit := int(req.Msg.Limit)
	if limit <= 0 {
		limit = 20
	}

	alerts, err := h.svc.GetUnreadAlerts(ctx, userID, limit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoAlerts := make([]*echov1.Alert, 0, len(alerts))
	for _, a := range alerts {
		protoAlert := &echov1.Alert{
			Id:          a.ID.String(),
			UserId:      a.UserID.String(),
			AlertType:   toProtoAlertType(a.AlertType),
			Severity:    toProtoSeverity(a.Severity),
			Title:       a.Title,
			Message:     a.Message,
			IsRead:      a.IsRead,
			IsDismissed: a.IsDismissed,
			AlertDate:   timestamppb.New(a.AlertDate),
			CreatedAt:   timestamppb.New(a.CreatedAt),
		}
		protoAlerts = append(protoAlerts, protoAlert)
	}

	return connect.NewResponse(&echov1.ListAlertsResponse{
		Alerts: protoAlerts,
	}), nil
}

// MarkAlertRead marks an alert as read.
func (h *InsightsHandler) MarkAlertRead(
	ctx context.Context,
	req *connect.Request[echov1.MarkAlertReadRequest],
) (*connect.Response[echov1.MarkAlertReadResponse], error) {
	_, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	alertID, err := uuid.Parse(req.Msg.AlertId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid alert ID"))
	}

	if err := h.svc.MarkAlertRead(ctx, alertID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&echov1.MarkAlertReadResponse{}), nil
}

// DismissAlert marks an alert as dismissed.
func (h *InsightsHandler) DismissAlert(
	ctx context.Context,
	req *connect.Request[echov1.DismissAlertRequest],
) (*connect.Response[echov1.DismissAlertResponse], error) {
	_, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	alertID, err := uuid.Parse(req.Msg.AlertId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid alert ID"))
	}

	if err := h.svc.MarkAlertDismissed(ctx, alertID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&echov1.DismissAlertResponse{}), nil
}

// toProtoAlertType converts domain AlertType to proto AlertType
func toProtoAlertType(t insights.AlertType) echov1.AlertType {
	switch t {
	case insights.AlertTypePaceWarning:
		return echov1.AlertType_ALERT_TYPE_PACE_WARNING
	case insights.AlertTypeSurpriseExpense:
		return echov1.AlertType_ALERT_TYPE_SURPRISE_EXPENSE
	case insights.AlertTypeGoalProgress:
		return echov1.AlertType_ALERT_TYPE_GOAL_PROGRESS
	case insights.AlertTypeSubscriptionDue:
		return echov1.AlertType_ALERT_TYPE_SUBSCRIPTION_DUE
	default:
		return echov1.AlertType_ALERT_TYPE_UNSPECIFIED
	}
}

// toProtoSeverity converts domain AlertSeverity to proto AlertSeverity
func toProtoSeverity(s insights.AlertSeverity) echov1.AlertSeverity {
	switch s {
	case insights.AlertSeverityInfo:
		return echov1.AlertSeverity_ALERT_SEVERITY_INFO
	case insights.AlertSeverityWarning:
		return echov1.AlertSeverity_ALERT_SEVERITY_WARNING
	case insights.AlertSeverityCritical:
		return echov1.AlertSeverity_ALERT_SEVERITY_CRITICAL
	default:
		return echov1.AlertSeverity_ALERT_SEVERITY_UNSPECIFIED
	}
}
