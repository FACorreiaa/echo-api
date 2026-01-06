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

// GetImportInsights returns quality metrics for an import job.
func (h *InsightsHandler) GetImportInsights(
	ctx context.Context,
	req *connect.Request[echov1.GetImportInsightsRequest],
) (*connect.Response[echov1.GetImportInsightsResponse], error) {
	_, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	importJobID, err := uuid.Parse(req.Msg.ImportJobId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid import job ID"))
	}

	insights, err := h.svc.GetImportInsights(ctx, importJobID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("import insights not found"))
	}

	// Convert domain issues to proto
	protoIssues := make([]*echov1.ImportIssue, 0, len(insights.Issues))
	for _, issue := range insights.Issues {
		protoIssues = append(protoIssues, &echov1.ImportIssue{
			Type:         toProtoImportIssueType(issue.Type),
			AffectedRows: int32(issue.AffectedRows),
			SampleValue:  issue.SampleValue,
			Suggestion:   issue.Suggestion,
		})
	}

	resp := &echov1.GetImportInsightsResponse{
		Insights: &echov1.ImportInsights{
			ImportJobId:        insights.ImportJobID.String(),
			InstitutionName:    insights.InstitutionName,
			TotalRows:          int32(insights.DuplicatesSkipped), // TODO: compute from import job
			RowsImported:       int32(0),                          // TODO: compute from import job
			RowsFailed:         int32(0),                          // TODO: compute from import job
			DuplicatesSkipped:  int32(insights.DuplicatesSkipped),
			CategorizationRate: insights.CategorizationRate,
			DateQualityScore:   insights.DateQualityScore,
			AmountQualityScore: insights.AmountQualityScore,
			TotalIncome:        toMoney(insights.TotalIncome),
			TotalExpenses:      toMoney(insights.TotalExpenses),
			Issues:             protoIssues,
		},
	}

	if insights.EarliestDate != nil {
		resp.Insights.EarliestDate = timestamppb.New(*insights.EarliestDate)
	}
	if insights.LatestDate != nil {
		resp.Insights.LatestDate = timestamppb.New(*insights.LatestDate)
	}

	return connect.NewResponse(resp), nil
}

// GetDataSourceHealth returns health metrics for all connected data sources.
func (h *InsightsHandler) GetDataSourceHealth(
	ctx context.Context,
	req *connect.Request[echov1.GetDataSourceHealthRequest],
) (*connect.Response[echov1.GetDataSourceHealthResponse], error) {
	userIDStr, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("invalid user ID in context"))
	}

	sources, err := h.svc.GetDataSourceHealth(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoSources := make([]*echov1.DataSourceHealth, 0, len(sources))
	totalCount := 0
	var oldest *timestamppb.Timestamp

	for _, s := range sources {
		protoSource := &echov1.DataSourceHealth{
			InstitutionName:    s.InstitutionName,
			SourceType:         toProtoTransactionSource(s.SourceType),
			TransactionCount:   int32(s.TransactionCount),
			CategorizationRate: s.CategorizationRate,
			UncategorizedCount: int32(s.UncategorizedCount),
		}

		if s.FirstTransaction != nil {
			protoSource.FirstTransaction = timestamppb.New(*s.FirstTransaction)
			if oldest == nil || s.FirstTransaction.Before(oldest.AsTime()) {
				oldest = protoSource.FirstTransaction
			}
		}
		if s.LastTransaction != nil {
			protoSource.LastTransaction = timestamppb.New(*s.LastTransaction)
		}
		if s.LastImport != nil {
			protoSource.LastImport = timestamppb.New(*s.LastImport)
		}

		totalCount += s.TransactionCount
		protoSources = append(protoSources, protoSource)
	}

	return connect.NewResponse(&echov1.GetDataSourceHealthResponse{
		Sources:               protoSources,
		TotalTransactionCount: int32(totalCount),
		OldestTransaction:     oldest,
	}), nil
}

func toProtoImportIssueType(t string) echov1.ImportIssueType {
	switch t {
	case "unparseable_date":
		return echov1.ImportIssueType_IMPORT_ISSUE_TYPE_UNPARSEABLE_DATE
	case "invalid_amount":
		return echov1.ImportIssueType_IMPORT_ISSUE_TYPE_INVALID_AMOUNT
	case "missing_description":
		return echov1.ImportIssueType_IMPORT_ISSUE_TYPE_MISSING_DESCRIPTION
	case "duplicate_rows":
		return echov1.ImportIssueType_IMPORT_ISSUE_TYPE_DUPLICATE_ROWS
	case "uncategorized":
		return echov1.ImportIssueType_IMPORT_ISSUE_TYPE_UNCATEGORIZED
	case "future_date":
		return echov1.ImportIssueType_IMPORT_ISSUE_TYPE_FUTURE_DATE
	default:
		return echov1.ImportIssueType_IMPORT_ISSUE_TYPE_UNSPECIFIED
	}
}

func toProtoTransactionSource(s string) echov1.TransactionSource {
	switch s {
	case "manual":
		return echov1.TransactionSource_TRANSACTION_SOURCE_MANUAL
	case "csv":
		return echov1.TransactionSource_TRANSACTION_SOURCE_CSV
	case "aggregator":
		return echov1.TransactionSource_TRANSACTION_SOURCE_AGGREGATOR
	default:
		return echov1.TransactionSource_TRANSACTION_SOURCE_UNSPECIFIED
	}
}

// GetMonthlyInsights returns monthly insights with "3 things changed" and "1 action".
func (h *InsightsHandler) GetMonthlyInsights(
	ctx context.Context,
	req *connect.Request[echov1.GetMonthlyInsightsRequest],
) (*connect.Response[echov1.GetMonthlyInsightsResponse], error) {
	userIDStr, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("invalid user ID in context"))
	}

	monthStart := req.Msg.MonthStart.AsTime()
	mi, err := h.svc.GetMonthlyInsights(ctx, userID, monthStart)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Convert domain model to proto
	protoInsights := &echov1.MonthlyInsights{
		Id:                 mi.ID.String(),
		UserId:             mi.UserID.String(),
		MonthStart:         timestamppb.New(mi.MonthStart),
		TotalSpend:         toMoney(mi.TotalSpend),
		TotalIncome:        toMoney(mi.TotalIncome),
		Net:                toMoney(mi.Net),
		SpendVsLastMonth:   toMoney(mi.SpendVsLastMonth),
		SpendChangePercent: mi.SpendChangePercent,
		Highlights:         mi.Highlights,
		CreatedAt:          timestamppb.New(mi.CreatedAt),
	}

	// Add top categories
	for _, cat := range mi.TopCategories {
		protoCat := &echov1.CategorySpend{
			CategoryId: cat.CategoryID.String(),
			Total:      toMoney(cat.AmountCents),
		}
		protoInsights.TopCategories = append(protoInsights.TopCategories, protoCat)
	}

	// Add top merchants
	for _, m := range mi.TopMerchants {
		protoMerchant := &echov1.MerchantSpend{
			MerchantName: m.MerchantName,
			Total:        toMoney(m.AmountCents),
		}
		protoInsights.TopMerchants = append(protoInsights.TopMerchants, protoMerchant)
	}

	// Add changes ("3 things that changed")
	for _, change := range mi.Changes {
		protoChange := &echov1.InsightChange{
			Type:          changeTypeToProto(change.Type),
			Title:         change.Title,
			Description:   change.Description,
			AmountChange:  toMoney(change.AmountChange),
			PercentChange: change.PercentChange,
			Icon:          change.Icon,
			Sentiment:     changeSentimentToProto(change.Sentiment),
		}
		if change.CategoryID != nil {
			catID := change.CategoryID.String()
			protoChange.CategoryId = &catID
		}
		if change.MerchantName != nil {
			protoChange.MerchantName = change.MerchantName
		}
		protoInsights.Changes = append(protoInsights.Changes, protoChange)
	}

	// Add recommended action ("1 action to take")
	if mi.RecommendedAction != nil {
		protoInsights.RecommendedAction = &echov1.ActionRecommendation{
			Type:            actionTypeToProto(mi.RecommendedAction.Type),
			Title:           mi.RecommendedAction.Title,
			Description:     mi.RecommendedAction.Description,
			CtaText:         mi.RecommendedAction.CTAText,
			CtaAction:       mi.RecommendedAction.CTAAction,
			PotentialImpact: toMoney(mi.RecommendedAction.PotentialImpact),
			Priority:        actionPriorityToProto(mi.RecommendedAction.Priority),
			Icon:            mi.RecommendedAction.Icon,
		}
	}

	return connect.NewResponse(&echov1.GetMonthlyInsightsResponse{
		Insights: protoInsights,
	}), nil
}

// changeTypeToProto converts domain InsightChangeType to proto
func changeTypeToProto(t insights.InsightChangeType) echov1.InsightChangeType {
	switch t {
	case insights.InsightChangeTypeCategoryIncrease:
		return echov1.InsightChangeType_INSIGHT_CHANGE_TYPE_CATEGORY_INCREASE
	case insights.InsightChangeTypeCategoryDecrease:
		return echov1.InsightChangeType_INSIGHT_CHANGE_TYPE_CATEGORY_DECREASE
	case insights.InsightChangeTypeNewMerchant:
		return echov1.InsightChangeType_INSIGHT_CHANGE_TYPE_NEW_MERCHANT
	case insights.InsightChangeTypeMerchantIncrease:
		return echov1.InsightChangeType_INSIGHT_CHANGE_TYPE_MERCHANT_INCREASE
	case insights.InsightChangeTypeSubscriptionDetected:
		return echov1.InsightChangeType_INSIGHT_CHANGE_TYPE_SUBSCRIPTION_DETECTED
	case insights.InsightChangeTypeGoalProgress:
		return echov1.InsightChangeType_INSIGHT_CHANGE_TYPE_GOAL_PROGRESS
	case insights.InsightChangeTypeIncomeChange:
		return echov1.InsightChangeType_INSIGHT_CHANGE_TYPE_INCOME_CHANGE
	case insights.InsightChangeTypeSavingsRate:
		return echov1.InsightChangeType_INSIGHT_CHANGE_TYPE_SAVINGS_RATE
	default:
		return echov1.InsightChangeType_INSIGHT_CHANGE_TYPE_UNSPECIFIED
	}
}

// changeSentimentToProto converts domain InsightChangeSentiment to proto
func changeSentimentToProto(s insights.InsightChangeSentiment) echov1.InsightChangeSentiment {
	switch s {
	case insights.InsightChangeSentimentPositive:
		return echov1.InsightChangeSentiment_INSIGHT_CHANGE_SENTIMENT_POSITIVE
	case insights.InsightChangeSentimentNegative:
		return echov1.InsightChangeSentiment_INSIGHT_CHANGE_SENTIMENT_NEGATIVE
	case insights.InsightChangeSentimentNeutral:
		return echov1.InsightChangeSentiment_INSIGHT_CHANGE_SENTIMENT_NEUTRAL
	default:
		return echov1.InsightChangeSentiment_INSIGHT_CHANGE_SENTIMENT_UNSPECIFIED
	}
}

// actionTypeToProto converts domain ActionType to proto
func actionTypeToProto(t insights.ActionType) echov1.ActionType {
	switch t {
	case insights.ActionTypeReviewSubscriptions:
		return echov1.ActionType_ACTION_TYPE_REVIEW_SUBSCRIPTIONS
	case insights.ActionTypeReduceCategory:
		return echov1.ActionType_ACTION_TYPE_REDUCE_CATEGORY
	case insights.ActionTypeContributeToGoal:
		return echov1.ActionType_ACTION_TYPE_CONTRIBUTE_TO_GOAL
	case insights.ActionTypeCategorizeTransactions:
		return echov1.ActionType_ACTION_TYPE_CATEGORIZE_TRANSACTIONS
	case insights.ActionTypeSetBudget:
		return echov1.ActionType_ACTION_TYPE_SET_BUDGET
	case insights.ActionTypeReviewLargeExpense:
		return echov1.ActionType_ACTION_TYPE_REVIEW_LARGE_EXPENSE
	default:
		return echov1.ActionType_ACTION_TYPE_UNSPECIFIED
	}
}

// actionPriorityToProto converts domain ActionPriority to proto
func actionPriorityToProto(p insights.ActionPriority) echov1.ActionPriority {
	switch p {
	case insights.ActionPriorityLow:
		return echov1.ActionPriority_ACTION_PRIORITY_LOW
	case insights.ActionPriorityMedium:
		return echov1.ActionPriority_ACTION_PRIORITY_MEDIUM
	case insights.ActionPriorityHigh:
		return echov1.ActionPriority_ACTION_PRIORITY_HIGH
	default:
		return echov1.ActionPriority_ACTION_PRIORITY_UNSPECIFIED
	}
}

// GetWrapped returns the wrapped summary for a period.
func (h *InsightsHandler) GetWrapped(
	ctx context.Context,
	req *connect.Request[echov1.GetWrappedRequest],
) (*connect.Response[echov1.GetWrappedResponse], error) {
	userIDStr, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("invalid user ID in context"))
	}

	// Parse period
	period := "month"
	if req.Msg.Period == echov1.WrappedPeriod_WRAPPED_PERIOD_YEAR {
		period = "year"
	}

	periodStart := req.Msg.PeriodStart.AsTime()
	periodEnd := req.Msg.PeriodEnd.AsTime()

	// Get wrapped summary from service
	summary, err := h.svc.GetWrapped(ctx, userID, period, periodStart, periodEnd)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Build proto cards
	protoCards := make([]*echov1.WrappedCard, 0, len(summary.Cards))
	for _, card := range summary.Cards {
		protoCards = append(protoCards, &echov1.WrappedCard{
			Title:    card.Title,
			Subtitle: card.Subtitle,
			Body:     card.Body,
			Accent:   card.Accent,
		})
	}

	protoPeriod := echov1.WrappedPeriod_WRAPPED_PERIOD_MONTH
	if period == "year" {
		protoPeriod = echov1.WrappedPeriod_WRAPPED_PERIOD_YEAR
	}

	return connect.NewResponse(&echov1.GetWrappedResponse{
		Wrapped: &echov1.WrappedSummary{
			Id:          summary.ID.String(),
			UserId:      summary.UserID.String(),
			Period:      protoPeriod,
			PeriodStart: timestamppb.New(summary.PeriodStart),
			PeriodEnd:   timestamppb.New(summary.PeriodEnd),
			Cards:       protoCards,
			CreatedAt:   timestamppb.New(summary.CreatedAt),
		},
	}), nil
}
