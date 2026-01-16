// Package handler implements the WaitlistService Connect RPC handlers.
package handler

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	echov1 "buf.build/gen/go/echo-tracker/echo/protocolbuffers/go/echo/v1"
	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/waitlist/repository"
	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/waitlist/service"
	"github.com/FACorreiaa/smart-finance-tracker/pkg/interceptors"
)

// WaitlistHandler implements the WaitlistService Connect handlers
type WaitlistHandler struct {
	svc *service.WaitlistService
}

// NewWaitlistHandler creates a new waitlist handler
func NewWaitlistHandler(svc *service.WaitlistService) *WaitlistHandler {
	return &WaitlistHandler{svc: svc}
}

// AddToWaitlist handles public waitlist signups from the landing page
func (h *WaitlistHandler) AddToWaitlist(
	ctx context.Context,
	req *connect.Request[echov1.AddToWaitlistRequest],
) (*connect.Response[echov1.AddToWaitlistResponse], error) {
	email := req.Msg.Email
	if email == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("email is required"))
	}

	entry, position, err := h.svc.AddToWaitlist(ctx, email)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	message := "You're on the waitlist!"
	if position > 0 {
		message = "You're #" + itoa(position) + " on the waitlist!"
	}

	// Check if user was already on waitlist
	if entry.Status != repository.StatusPending {
		message = "You're already on the waitlist!"
	}

	return connect.NewResponse(&echov1.AddToWaitlistResponse{
		Success:  true,
		Position: int32(position),
		Message:  message,
	}), nil
}

// ListWaitlist returns paginated waitlist entries (admin only)
func (h *WaitlistHandler) ListWaitlist(
	ctx context.Context,
	req *connect.Request[echov1.ListWaitlistRequest],
) (*connect.Response[echov1.ListWaitlistResponse], error) {
	// Admin check - verify user is authenticated
	_, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	// TODO: Add admin role check

	var statusFilter *repository.WaitlistStatus
	if req.Msg.StatusFilter != nil && *req.Msg.StatusFilter != echov1.WaitlistStatus_WAITLIST_STATUS_UNSPECIFIED {
		status := protoToStatus(*req.Msg.StatusFilter)
		statusFilter = &status
	}

	limit := 50
	offset := 0
	if req.Msg.Page != nil {
		if req.Msg.Page.PageSize > 0 {
			limit = int(req.Msg.Page.PageSize)
		}
		// Parse page token as offset
		if req.Msg.Page.PageToken != "" {
			if parsed := atoi(req.Msg.Page.PageToken); parsed > 0 {
				offset = parsed
			}
		}
	}

	entries, total, err := h.svc.ListWaitlist(ctx, statusFilter, limit, offset)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoEntries := make([]*echov1.WaitlistEntry, 0, len(entries))
	for _, e := range entries {
		protoEntries = append(protoEntries, entryToProto(e))
	}

	var nextPageToken string
	if offset+limit < total {
		nextPageToken = itoa(offset + limit)
	}

	return connect.NewResponse(&echov1.ListWaitlistResponse{
		Entries:    protoEntries,
		TotalCount: int32(total),
		Page: &echov1.PageResponse{
			NextPageToken: nextPageToken,
		},
	}), nil
}

// SendInvite sends an early access invite (admin only)
func (h *WaitlistHandler) SendInvite(
	ctx context.Context,
	req *connect.Request[echov1.SendInviteRequest],
) (*connect.Response[echov1.SendInviteResponse], error) {
	// Admin check
	_, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	waitlistID, err := uuid.Parse(req.Msg.WaitlistId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid waitlist_id"))
	}

	inviteCode, err := h.svc.SendInvite(ctx, waitlistID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&echov1.SendInviteResponse{
		Success:    true,
		InviteCode: inviteCode,
		Message:    "Invitation sent successfully",
	}), nil
}

// GetWaitlistStats returns waitlist metrics (admin only)
func (h *WaitlistHandler) GetWaitlistStats(
	ctx context.Context,
	req *connect.Request[echov1.GetWaitlistStatsRequest],
) (*connect.Response[echov1.GetWaitlistStatsResponse], error) {
	// Admin check
	_, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	stats, err := h.svc.GetStats(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&echov1.GetWaitlistStatsResponse{
		TotalSignups:    int32(stats.TotalSignups),
		PendingCount:    int32(stats.PendingCount),
		InvitedCount:    int32(stats.InvitedCount),
		JoinedCount:     int32(stats.JoinedCount),
		SignupsToday:    int32(stats.SignupsToday),
		SignupsThisWeek: int32(stats.SignupsThisWeek),
	}), nil
}

// Helper functions

func entryToProto(e *repository.WaitlistEntry) *echov1.WaitlistEntry {
	entry := &echov1.WaitlistEntry{
		Id:        e.ID.String(),
		Email:     e.Email,
		Status:    statusToProto(e.Status),
		CreatedAt: timestamppb.New(e.CreatedAt),
	}
	if e.InvitedAt != nil {
		entry.InvitedAt = timestamppb.New(*e.InvitedAt)
	}
	if e.InviteCode != nil {
		entry.InviteCode = e.InviteCode
	}
	return entry
}

func statusToProto(s repository.WaitlistStatus) echov1.WaitlistStatus {
	switch s {
	case repository.StatusPending:
		return echov1.WaitlistStatus_WAITLIST_STATUS_PENDING
	case repository.StatusInvited:
		return echov1.WaitlistStatus_WAITLIST_STATUS_INVITED
	case repository.StatusJoined:
		return echov1.WaitlistStatus_WAITLIST_STATUS_JOINED
	default:
		return echov1.WaitlistStatus_WAITLIST_STATUS_UNSPECIFIED
	}
}

func protoToStatus(s echov1.WaitlistStatus) repository.WaitlistStatus {
	switch s {
	case echov1.WaitlistStatus_WAITLIST_STATUS_PENDING:
		return repository.StatusPending
	case echov1.WaitlistStatus_WAITLIST_STATUS_INVITED:
		return repository.StatusInvited
	case echov1.WaitlistStatus_WAITLIST_STATUS_JOINED:
		return repository.StatusJoined
	default:
		return repository.StatusPending
	}
}

func itoa(i int) string {
	return string(rune('0'+i%10)) + itoa_helper(i/10)
}

func itoa_helper(i int) string {
	if i == 0 {
		return ""
	}
	return itoa_helper(i/10) + string(rune('0'+i%10))
}

func atoi(s string) int {
	result := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}
	return result
}
