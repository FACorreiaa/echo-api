// Package handler implements the Subscriptions Connect RPC handlers.
package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	echov1 "buf.build/gen/go/echo-tracker/echo/protocolbuffers/go/echo/v1"
	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/subscriptions/repository"
	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/subscriptions/service"
	"github.com/FACorreiaa/smart-finance-tracker/pkg/interceptors"
)

// SubscriptionsHandler implements the subscription-related Connect handlers
type SubscriptionsHandler struct {
	svc *service.Service
}

// NewSubscriptionsHandler constructs a new handler
func NewSubscriptionsHandler(svc *service.Service) *SubscriptionsHandler {
	return &SubscriptionsHandler{svc: svc}
}

// ListRecurringSubscriptions retrieves all subscriptions for a user
func (h *SubscriptionsHandler) ListRecurringSubscriptions(
	ctx context.Context,
	req *connect.Request[echov1.ListRecurringSubscriptionsRequest],
) (*connect.Response[echov1.ListRecurringSubscriptionsResponse], error) {
	userID, err := getUserID(ctx)
	if err != nil {
		return nil, err
	}

	var statusFilter *repository.RecurringStatus
	if req.Msg.StatusFilter != nil {
		s := protoToStatus(*req.Msg.StatusFilter)
		statusFilter = &s
	}

	subs, err := h.svc.ListSubscriptions(ctx, userID, statusFilter, req.Msg.IncludeCanceled)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	totalMonthly, activeCount, _ := h.svc.GetTotalMonthlySubscriptionCost(ctx, userID)

	protoSubs := make([]*echov1.RecurringSubscription, 0, len(subs))
	for _, sub := range subs {
		protoSubs = append(protoSubs, subscriptionToProto(sub))
	}

	return connect.NewResponse(&echov1.ListRecurringSubscriptionsResponse{
		Subscriptions:    protoSubs,
		TotalMonthlyCost: toMoney(totalMonthly, "EUR"),
		ActiveCount:      int32(activeCount),
	}), nil
}

// DetectRecurringSubscriptions analyzes transactions to detect recurring patterns
func (h *SubscriptionsHandler) DetectRecurringSubscriptions(
	ctx context.Context,
	req *connect.Request[echov1.DetectRecurringSubscriptionsRequest],
) (*connect.Response[echov1.DetectRecurringSubscriptionsResponse], error) {
	userID, err := getUserID(ctx)
	if err != nil {
		return nil, err
	}

	// Default to 6 months back
	since := time.Now().AddDate(0, -6, 0)
	if req.Msg.Since != nil {
		since = req.Msg.Since.AsTime()
	}

	minOccurrences := 2
	if req.Msg.MinOccurrences != nil {
		minOccurrences = int(*req.Msg.MinOccurrences)
	}

	result, err := h.svc.DetectSubscriptions(ctx, userID, since, minOccurrences)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoSubs := make([]*echov1.RecurringSubscription, 0, len(result.Detected))
	for _, sub := range result.Detected {
		protoSubs = append(protoSubs, subscriptionToProto(sub))
	}

	return connect.NewResponse(&echov1.DetectRecurringSubscriptionsResponse{
		Detected:     protoSubs,
		NewCount:     int32(result.NewCount),
		UpdatedCount: int32(result.UpdatedCount),
	}), nil
}

// UpdateSubscriptionStatus updates the status of a subscription
func (h *SubscriptionsHandler) UpdateSubscriptionStatus(
	ctx context.Context,
	req *connect.Request[echov1.UpdateSubscriptionStatusRequest],
) (*connect.Response[echov1.UpdateSubscriptionStatusResponse], error) {
	_, err := getUserID(ctx)
	if err != nil {
		return nil, err
	}

	subID, err := uuid.Parse(req.Msg.SubscriptionId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid subscription ID"))
	}

	status := protoToStatus(req.Msg.Status)

	sub, err := h.svc.UpdateStatus(ctx, subID, status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("subscription not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&echov1.UpdateSubscriptionStatusResponse{
		Subscription: subscriptionToProto(sub),
	}), nil
}

// GetSubscriptionReviewChecklist returns subscriptions that need review
func (h *SubscriptionsHandler) GetSubscriptionReviewChecklist(
	ctx context.Context,
	req *connect.Request[echov1.GetSubscriptionReviewChecklistRequest],
) (*connect.Response[echov1.GetSubscriptionReviewChecklistResponse], error) {
	userID, err := getUserID(ctx)
	if err != nil {
		return nil, err
	}

	items, potentialSavings, err := h.svc.GetReviewChecklist(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoItems := make([]*echov1.SubscriptionReviewItem, 0, len(items))
	for _, item := range items {
		protoItems = append(protoItems, reviewItemToProto(item))
	}

	summary := "All subscriptions look good!"
	if len(items) > 0 {
		summary = pluralize(len(items), "subscription") + " to review"
	}

	return connect.NewResponse(&echov1.GetSubscriptionReviewChecklistResponse{
		Items:                   protoItems,
		PotentialMonthlySavings: toMoney(potentialSavings, "EUR"),
		Summary:                 summary,
	}), nil
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

func subscriptionToProto(sub *repository.RecurringSubscription) *echov1.RecurringSubscription {
	proto := &echov1.RecurringSubscription{
		Id:              sub.ID.String(),
		UserId:          sub.UserID.String(),
		MerchantName:    sub.MerchantName,
		Amount:          toMoney(sub.AmountMinor, sub.CurrencyCode),
		Cadence:         cadenceToProto(sub.Cadence),
		Status:          statusToProto(sub.Status),
		OccurrenceCount: int32(sub.OccurrenceCount),
		CreatedAt:       timestamppb.New(sub.CreatedAt),
		UpdatedAt:       timestamppb.New(sub.UpdatedAt),
	}

	if sub.FirstSeenAt != nil {
		proto.FirstSeenAt = timestamppb.New(*sub.FirstSeenAt)
	}
	if sub.LastSeenAt != nil {
		proto.LastSeenAt = timestamppb.New(*sub.LastSeenAt)
	}
	if sub.NextExpectedAt != nil {
		proto.NextExpectedAt = timestamppb.New(*sub.NextExpectedAt)
	}
	if sub.CategoryID != nil {
		catID := sub.CategoryID.String()
		proto.CategoryId = &catID
	}

	return proto
}

func reviewItemToProto(item *service.SubscriptionReviewItem) *echov1.SubscriptionReviewItem {
	return &echov1.SubscriptionReviewItem{
		Subscription:      subscriptionToProto(item.Subscription),
		Reason:            reviewReasonToProto(item.Reason),
		ReasonMessage:     item.ReasonMessage,
		RecommendedCancel: item.RecommendedCancel,
	}
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

func cadenceToProto(c repository.RecurringCadence) echov1.RecurringCadence {
	switch c {
	case repository.RecurringCadenceWeekly:
		return echov1.RecurringCadence_RECURRING_CADENCE_WEEKLY
	case repository.RecurringCadenceMonthly:
		return echov1.RecurringCadence_RECURRING_CADENCE_MONTHLY
	case repository.RecurringCadenceQuarterly:
		return echov1.RecurringCadence_RECURRING_CADENCE_QUARTERLY
	case repository.RecurringCadenceAnnual:
		return echov1.RecurringCadence_RECURRING_CADENCE_ANNUAL
	default:
		return echov1.RecurringCadence_RECURRING_CADENCE_UNKNOWN
	}
}

func statusToProto(s repository.RecurringStatus) echov1.RecurringStatus {
	switch s {
	case repository.RecurringStatusActive:
		return echov1.RecurringStatus_RECURRING_STATUS_ACTIVE
	case repository.RecurringStatusPaused:
		return echov1.RecurringStatus_RECURRING_STATUS_PAUSED
	case repository.RecurringStatusCanceled:
		return echov1.RecurringStatus_RECURRING_STATUS_CANCELED
	default:
		return echov1.RecurringStatus_RECURRING_STATUS_UNSPECIFIED
	}
}

func protoToStatus(s echov1.RecurringStatus) repository.RecurringStatus {
	switch s {
	case echov1.RecurringStatus_RECURRING_STATUS_ACTIVE:
		return repository.RecurringStatusActive
	case echov1.RecurringStatus_RECURRING_STATUS_PAUSED:
		return repository.RecurringStatusPaused
	case echov1.RecurringStatus_RECURRING_STATUS_CANCELED:
		return repository.RecurringStatusCanceled
	default:
		return repository.RecurringStatusActive
	}
}

func reviewReasonToProto(r service.ReviewReason) echov1.SubscriptionReviewReason {
	switch r {
	case service.ReviewReasonUnused:
		return echov1.SubscriptionReviewReason_SUBSCRIPTION_REVIEW_REASON_UNUSED
	case service.ReviewReasonPriceIncrease:
		return echov1.SubscriptionReviewReason_SUBSCRIPTION_REVIEW_REASON_PRICE_INCREASE
	case service.ReviewReasonDuplicate:
		return echov1.SubscriptionReviewReason_SUBSCRIPTION_REVIEW_REASON_DUPLICATE
	case service.ReviewReasonHighCost:
		return echov1.SubscriptionReviewReason_SUBSCRIPTION_REVIEW_REASON_HIGH_COST
	case service.ReviewReasonNew:
		return echov1.SubscriptionReviewReason_SUBSCRIPTION_REVIEW_REASON_NEW
	default:
		return echov1.SubscriptionReviewReason_SUBSCRIPTION_REVIEW_REASON_UNSPECIFIED
	}
}

func pluralize(count int, singular string) string {
	if count == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %ss", count, singular)
}

