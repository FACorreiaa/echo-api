package handler

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	echov1 "buf.build/gen/go/echo-tracker/echo/protocolbuffers/go/echo/v1"
	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/balance"
	"github.com/FACorreiaa/smart-finance-tracker/pkg/interceptors"
)

// BalanceHandler implements the BalanceService RPC handlers
type BalanceHandler struct {
	svc *balance.Service
}

// NewBalanceHandler creates a new balance handler
func NewBalanceHandler(svc *balance.Service) *BalanceHandler {
	return &BalanceHandler{svc: svc}
}

// GetBalance returns the user's current balance
func (h *BalanceHandler) GetBalance(
	ctx context.Context,
	req *connect.Request[echov1.GetBalanceRequest],
) (*connect.Response[echov1.GetBalanceResponse], error) {
	// Get user ID from context
	userIDStr, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Parse optional account ID
	var accountID *uuid.UUID
	if req.Msg.AccountId != nil && *req.Msg.AccountId != "" {
		parsed, err := uuid.Parse(*req.Msg.AccountId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		accountID = &parsed
	}

	// Get balance
	result, err := h.svc.GetBalance(ctx, userID, accountID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Build response
	resp := &echov1.GetBalanceResponse{
		TotalNetWorth: &echov1.Money{
			AmountMinor:  result.TotalNetWorthCents,
			CurrencyCode: result.CurrencyCode,
		},
		SafeToSpend: &echov1.Money{
			AmountMinor:  result.SafeToSpendCents,
			CurrencyCode: result.CurrencyCode,
		},
		TotalInvestments: &echov1.Money{
			AmountMinor:  result.TotalInvestmentCents,
			CurrencyCode: result.CurrencyCode,
		},
		UpcomingBills: &echov1.Money{
			AmountMinor:  result.UpcomingBillsCents,
			CurrencyCode: result.CurrencyCode,
		},
		IsEstimated: result.IsEstimated,
		Balances:    make([]*echov1.AccountBalance, 0, len(result.Accounts)),
	}

	// Add account balances
	for _, a := range result.Accounts {
		ab := &echov1.AccountBalance{
			AccountId:   a.AccountID.String(),
			AccountName: a.AccountName,
			AccountType: echov1.AccountType(a.AccountType),
			CashBalance: &echov1.Money{
				AmountMinor:  a.CashBalanceCents,
				CurrencyCode: a.CurrencyCode,
			},
			InvestmentBalance: &echov1.Money{
				AmountMinor:  a.InvestmentCents,
				CurrencyCode: a.CurrencyCode,
			},
			Change_24H: &echov1.Money{
				AmountMinor:  a.Change24hCents,
				CurrencyCode: a.CurrencyCode,
			},
		}
		if !a.LastActivity.IsZero() {
			ab.LastActivity = timestamppb.New(a.LastActivity)
		}
		resp.Balances = append(resp.Balances, ab)
	}

	return connect.NewResponse(resp), nil
}

// GetBalanceHistory returns daily balance snapshots for charts
func (h *BalanceHandler) GetBalanceHistory(
	ctx context.Context,
	req *connect.Request[echov1.GetBalanceHistoryRequest],
) (*connect.Response[echov1.GetBalanceHistoryResponse], error) {
	// Get user ID from context
	userIDStr, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Parse optional account ID
	var accountID *uuid.UUID
	if req.Msg.AccountId != nil && *req.Msg.AccountId != "" {
		parsed, err := uuid.Parse(*req.Msg.AccountId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		accountID = &parsed
	}

	days := int(req.Msg.Days)
	if days <= 0 {
		days = 30 // Default
	}

	// Get history
	result, err := h.svc.GetBalanceHistory(ctx, userID, days, accountID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Build response
	resp := &echov1.GetBalanceHistoryResponse{
		History: make([]*echov1.DailyBalance, 0, len(result.History)),
		HighestBalance: &echov1.Money{
			AmountMinor:  result.HighestCents,
			CurrencyCode: result.CurrencyCode,
		},
		LowestBalance: &echov1.Money{
			AmountMinor:  result.LowestCents,
			CurrencyCode: result.CurrencyCode,
		},
		AverageBalance: &echov1.Money{
			AmountMinor:  result.AverageCents,
			CurrencyCode: result.CurrencyCode,
		},
	}

	for _, d := range result.History {
		resp.History = append(resp.History, &echov1.DailyBalance{
			Date: timestamppb.New(d.Date),
			Balance: &echov1.Money{
				AmountMinor:  d.BalanceCents,
				CurrencyCode: d.CurrencyCode,
			},
			Change: &echov1.Money{
				AmountMinor:  d.ChangeCents,
				CurrencyCode: d.CurrencyCode,
			},
		})
	}

	return connect.NewResponse(resp), nil
}
