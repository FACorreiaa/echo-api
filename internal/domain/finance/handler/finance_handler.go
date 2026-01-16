// Package handler implements the FinanceService Connect RPC handlers.
package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	"buf.build/gen/go/echo-tracker/echo/connectrpc/go/echo/v1/echov1connect"
	echov1 "buf.build/gen/go/echo-tracker/echo/protocolbuffers/go/echo/v1"
	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/categorization"
	goalsrepo "github.com/FACorreiaa/smart-finance-tracker/internal/domain/goals/repository"
	goalsservice "github.com/FACorreiaa/smart-finance-tracker/internal/domain/goals/service"
	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/import/repository"
	importservice "github.com/FACorreiaa/smart-finance-tracker/internal/domain/import/service"
	planservice "github.com/FACorreiaa/smart-finance-tracker/internal/domain/plan/service"
	subscriptionsrepo "github.com/FACorreiaa/smart-finance-tracker/internal/domain/subscriptions/repository"
	subscriptionsservice "github.com/FACorreiaa/smart-finance-tracker/internal/domain/subscriptions/service"
	"github.com/FACorreiaa/smart-finance-tracker/pkg/interceptors"
)

// FinanceHandler implements the FinanceService Connect handlers.
type FinanceHandler struct {
	echov1connect.UnimplementedFinanceServiceHandler
	importSvc        *importservice.ImportService
	importRepo       repository.ImportRepository
	catService       *categorization.Service
	goalsSvc         *goalsservice.Service
	subscriptionsSvc *subscriptionsservice.Service
	planSvc          *planservice.PlanService
}

// NewFinanceHandler constructs a new handler.
func NewFinanceHandler(importSvc *importservice.ImportService, repo repository.ImportRepository, catSvc *categorization.Service) *FinanceHandler {
	return &FinanceHandler{
		importSvc:  importSvc,
		importRepo: repo,
		catService: catSvc,
	}
}

// WithGoalsService sets the goals service on the handler
func (h *FinanceHandler) WithGoalsService(svc *goalsservice.Service) *FinanceHandler {
	h.goalsSvc = svc
	return h
}

// WithSubscriptionsService sets the subscriptions service on the handler
func (h *FinanceHandler) WithSubscriptionsService(svc *subscriptionsservice.Service) *FinanceHandler {
	h.subscriptionsSvc = svc
	return h
}

// WithPlanService sets the plan service on the handler
func (h *FinanceHandler) WithPlanService(svc *planservice.PlanService) *FinanceHandler {
	h.planSvc = svc
	return h
}

// ImportTransactionsCsv handles CSV transaction import with column mapping.
func (h *FinanceHandler) ImportTransactionsCsv(
	ctx context.Context,
	req *connect.Request[echov1.ImportTransactionsCsvRequest],
) (*connect.Response[echov1.ImportTransactionsCsvResponse], error) {
	// Get user ID from auth context
	userIDStr, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("invalid user ID in context"))
	}

	// Parse optional account ID
	var accountID *uuid.UUID
	if req.Msg.AccountId != nil && *req.Msg.AccountId != "" {
		parsed, err := uuid.Parse(*req.Msg.AccountId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid account_id"))
		}
		accountID = &parsed
	}

	// Validate CSV bytes
	if len(req.Msg.CsvBytes) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("csv_bytes is required"))
	}

	// Convert proto CsvMapping to service ColumnMapping
	mapping := h.protoMappingToService(req.Msg.Mapping, req.Msg.DateFormat)

	// Perform import
	result, err := h.importSvc.ImportWithOptions(ctx, userID, accountID, req.Msg.CsvBytes, mapping, importservice.ImportOptions{
		HeaderRows:      int(req.Msg.HeaderRows),
		Timezone:        req.Msg.Timezone,
		InstitutionName: req.Msg.InstitutionName,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if result.RowsImported == 0 && len(result.Errors) > 0 {
		errMsg := formatImportErrors(result.Errors)
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New(errMsg))
	}

	// Calculate duplicates: parsed rows minus rows that were actually inserted
	// RowsTotal = successfully parsed rows, RowsImported = actually inserted (deduplicated)
	duplicates := result.RowsTotal - result.RowsImported - result.RowsFailed
	if duplicates < 0 {
		duplicates = 0
	}

	return connect.NewResponse(&echov1.ImportTransactionsCsvResponse{
		ImportedCount:  int32(result.RowsImported),
		DuplicateCount: int32(duplicates),
		ImportJobId:    result.JobID.String(),
	}), nil
}

// protoMappingToService converts a proto CsvMapping to the service's ColumnMapping.
func (h *FinanceHandler) protoMappingToService(protoMapping *echov1.CsvMapping, dateFormat string) importservice.ColumnMapping {
	// Default mapping if none provided
	if protoMapping == nil {
		return importservice.ColumnMapping{
			DateCol:          -1,
			DescCol:          -1,
			AmountCol:        -1,
			CategoryCol:      -1,
			DebitCol:         -1,
			CreditCol:        -1,
			IsDoubleEntry:    false,
			IsEuropeanFormat: true, // Default to European for Portuguese/EU banks
			DateFormat:       dateFormat,
		}
	}

	// Parse column indices from column names (headers) or indices
	// The proto uses string for flexibility (could be header name or index)
	dateCol := parseColumnIndex(protoMapping.DateColumn)
	descCol := parseColumnIndex(protoMapping.DescriptionColumn)
	amountCol := parseColumnIndex(protoMapping.AmountColumn)
	debitCol := parseColumnIndex(protoMapping.DebitColumn)
	creditCol := parseColumnIndex(protoMapping.CreditColumn)

	// Determine if double entry based on whether debit/credit columns are set
	isDoubleEntry := debitCol >= 0 && creditCol >= 0

	// Parse delimiter from proto (single character string to rune)
	var delimiter rune
	if protoMapping.Delimiter != "" {
		delimiter = rune(protoMapping.Delimiter[0])
	}

	return importservice.ColumnMapping{
		DateCol:          dateCol,
		DescCol:          descCol,
		AmountCol:        amountCol,
		CategoryCol:      -1, // Could add to proto if needed
		DebitCol:         debitCol,
		CreditCol:        creditCol,
		IsDoubleEntry:    isDoubleEntry,
		IsEuropeanFormat: getIsEuropeanFormat(protoMapping),
		DateFormat:       dateFormat,
		Delimiter:        delimiter,
		SkipLines:        int(protoMapping.SkipLines),
	}
}

// parseColumnIndex converts a string column identifier to an int index.
// If it's a number string, parses as int. Otherwise returns -1.
func parseColumnIndex(col string) int {
	if col == "" {
		return -1
	}
	idx, err := strconv.Atoi(col)
	if err != nil {
		return -1
	}
	return idx
}

// getIsEuropeanFormat extracts the is_european_format field from the proto.
// Returns true (European format) as default for backwards compatibility.
func getIsEuropeanFormat(protoMapping *echov1.CsvMapping) bool {
	if protoMapping == nil {
		return true
	}
	return protoMapping.GetIsEuropeanFormat()
}

const maxImportErrorsInResponse = 10

func formatImportErrors(errors []string) string {
	if len(errors) == 0 {
		return "import failed: no valid rows"
	}

	limit := len(errors)
	if limit > maxImportErrorsInResponse {
		limit = maxImportErrorsInResponse
	}

	message := fmt.Sprintf("import failed: %d error(s). ", len(errors))
	message += strings.Join(errors[:limit], "; ")
	if limit < len(errors) {
		message += fmt.Sprintf(" (and %d more)", len(errors)-limit)
	}

	return message
}

// ListTransactions returns a paginated list of transactions with optional filters.
func (h *FinanceHandler) ListTransactions(
	ctx context.Context,
	req *connect.Request[echov1.ListTransactionsRequest],
) (*connect.Response[echov1.ListTransactionsResponse], error) {
	// Get user ID from auth context
	userIDStr, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("invalid user ID in context"))
	}

	// Build filter from request
	filter := repository.ListTransactionsFilter{
		Limit:  50, // Default
		Offset: 0,
	}

	// Parse pagination
	if req.Msg.Page != nil {
		filter.Limit = int(req.Msg.Page.PageSize)
		// Token-based pagination: decode offset from page token if provided
		if req.Msg.Page.PageToken != "" {
			// Parse page token as offset
			offset, err := strconv.Atoi(req.Msg.Page.PageToken)
			if err == nil && offset > 0 {
				filter.Offset = offset
			}
		}
	}

	// Parse optional filters
	if req.Msg.AccountId != nil && *req.Msg.AccountId != "" {
		parsed, err := uuid.Parse(*req.Msg.AccountId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid account_id"))
		}
		filter.AccountID = &parsed
	}

	if req.Msg.CategoryId != nil && *req.Msg.CategoryId != "" {
		parsed, err := uuid.Parse(*req.Msg.CategoryId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid category_id"))
		}
		filter.CategoryID = &parsed
	}

	if req.Msg.ImportJobId != nil && *req.Msg.ImportJobId != "" {
		parsed, err := uuid.Parse(*req.Msg.ImportJobId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid import_job_id"))
		}
		filter.ImportJobID = &parsed
	}

	if req.Msg.TimeRange != nil {
		if req.Msg.TimeRange.StartTime != nil {
			t := req.Msg.TimeRange.StartTime.AsTime()
			filter.StartDate = &t
		}
		if req.Msg.TimeRange.EndTime != nil {
			t := req.Msg.TimeRange.EndTime.AsTime()
			filter.EndDate = &t
		}
	}

	// Query transactions
	transactions, totalCount, err := h.importRepo.ListTransactions(ctx, userID, filter)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list transactions: %w", err))
	}

	// Convert to proto
	protoTxs := make([]*echov1.Transaction, 0, len(transactions))
	for _, tx := range transactions {
		protoTx := transactionToProto(tx)
		protoTxs = append(protoTxs, protoTx)
	}

	// Build next page token
	var nextPageToken string
	if int64(filter.Offset+filter.Limit) < totalCount {
		nextPageToken = strconv.Itoa(filter.Offset + filter.Limit)
	}

	return connect.NewResponse(&echov1.ListTransactionsResponse{
		Transactions: protoTxs,
		Page: &echov1.PageResponse{
			NextPageToken: nextPageToken,
		},
	}), nil
}

// transactionToProto converts a repository Transaction to proto Transaction
func transactionToProto(tx *repository.Transaction) *echov1.Transaction {
	result := &echov1.Transaction{
		Id:          tx.ID.String(),
		UserId:      tx.UserID.String(),
		Description: tx.Description,
		PostedAt:    timestamppb.New(tx.Date),
		CreatedAt:   timestamppb.New(tx.CreatedAt),
		UpdatedAt:   timestamppb.New(tx.UpdatedAt),
		Source:      echov1.TransactionSource(echov1.TransactionSource_value["TRANSACTION_SOURCE_"+strings.ToUpper(tx.Source)]),
	}

	if tx.AccountID != nil {
		s := tx.AccountID.String()
		result.AccountId = &s
	}
	if tx.CategoryID != nil {
		s := tx.CategoryID.String()
		result.CategoryId = &s
	}
	if tx.ExternalID != nil {
		result.ExternalId = *tx.ExternalID
	}
	if tx.Notes != nil {
		result.Notes = *tx.Notes
	}
	if tx.MerchantName != nil {
		result.MerchantName = *tx.MerchantName
	}
	if tx.OriginalDescription != nil {
		result.OriginalDescription = *tx.OriginalDescription
	}
	if tx.InstitutionName != nil {
		result.InstitutionName = *tx.InstitutionName
	}

	// Convert amount
	result.Amount = &echov1.Money{
		AmountMinor:  tx.AmountCents,
		CurrencyCode: tx.CurrencyCode,
	}

	return result
}

// DeleteImportBatch deletes all transactions from a specific import batch.
func (h *FinanceHandler) DeleteImportBatch(
	ctx context.Context,
	req *connect.Request[echov1.DeleteImportBatchRequest],
) (*connect.Response[echov1.DeleteImportBatchResponse], error) {
	// Get user ID from auth context
	userIDStr, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("invalid user ID in context"))
	}

	importJobID, err := uuid.Parse(req.Msg.ImportJobId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid import_job_id"))
	}

	deletedCount, err := h.importRepo.DeleteByImportJobID(ctx, userID, importJobID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete import batch: %w", err))
	}

	return connect.NewResponse(&echov1.DeleteImportBatchResponse{
		DeletedCount: int32(deletedCount),
	}), nil
}

// CreateCategoryRule creates a new categorization rule for "Remember this" learning.
func (h *FinanceHandler) CreateCategoryRule(
	ctx context.Context,
	req *connect.Request[echov1.CreateCategoryRuleRequest],
) (*connect.Response[echov1.CreateCategoryRuleResponse], error) {
	userIDStr, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("invalid user ID in context"))
	}

	var categoryID *uuid.UUID
	if req.Msg.CategoryId != nil && *req.Msg.CategoryId != "" {
		parsed, err := uuid.Parse(*req.Msg.CategoryId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid category_id"))
		}
		categoryID = &parsed
	}

	rule, updated, err := h.catService.CreateRule(
		ctx, userID,
		req.Msg.MatchPattern,
		req.Msg.CleanName,
		categoryID,
		req.Msg.IsRecurring,
		req.Msg.ApplyToExisting,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create rule: %w", err))
	}

	var catIDStr *string
	if rule.AssignedCategoryID != nil {
		s := rule.AssignedCategoryID.String()
		catIDStr = &s
	}

	return connect.NewResponse(&echov1.CreateCategoryRuleResponse{
		Rule: &echov1.CategoryRule{
			Id:           rule.ID.String(),
			UserId:       rule.UserID.String(),
			MatchPattern: rule.MatchPattern,
			CleanName:    *rule.CleanName,
			CategoryId:   catIDStr,
			IsRecurring:  rule.IsRecurring,
			Priority:     int32(rule.Priority),
		},
		TransactionsUpdated: updated,
	}), nil
}

// ListCategoryRules lists all categorization rules for the user.
func (h *FinanceHandler) ListCategoryRules(
	ctx context.Context,
	req *connect.Request[echov1.ListCategoryRulesRequest],
) (*connect.Response[echov1.ListCategoryRulesResponse], error) {
	userIDStr, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("invalid user ID in context"))
	}

	rules, err := h.catService.GetUserRules(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list rules: %w", err))
	}

	protoRules := make([]*echov1.CategoryRule, 0, len(rules))
	for _, r := range rules {
		var catIDStr *string
		if r.AssignedCategoryID != nil {
			s := r.AssignedCategoryID.String()
			catIDStr = &s
		}
		var cleanName string
		if r.CleanName != nil {
			cleanName = *r.CleanName
		}
		protoRules = append(protoRules, &echov1.CategoryRule{
			Id:           r.ID.String(),
			UserId:       r.UserID.String(),
			MatchPattern: r.MatchPattern,
			CleanName:    cleanName,
			CategoryId:   catIDStr,
			IsRecurring:  r.IsRecurring,
			Priority:     int32(r.Priority),
		})
	}

	return connect.NewResponse(&echov1.ListCategoryRulesResponse{
		Rules: protoRules,
	}), nil
}

// CreateManualTransaction handles Quick Capture natural language transaction input.
// Parses input like "Coffee 1$" and creates a transaction with auto-categorization.
func (h *FinanceHandler) CreateManualTransaction(
	ctx context.Context,
	req *connect.Request[echov1.CreateManualTransactionRequest],
) (*connect.Response[echov1.CreateManualTransactionResponse], error) {
	userIDStr, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("invalid user ID in context"))
	}

	// Parse natural language input
	parsed := parseNaturalLanguage(req.Msg.RawText)

	// Allow overrides from request
	description := parsed.Description
	if req.Msg.Description != nil && *req.Msg.Description != "" {
		description = *req.Msg.Description
	}

	amountMinor := parsed.AmountMinor
	if req.Msg.AmountMinor != nil {
		amountMinor = *req.Msg.AmountMinor
	}

	txDate := parsed.Date
	if req.Msg.Date != nil {
		txDate = req.Msg.Date.AsTime()
	}

	var accountID *uuid.UUID
	if req.Msg.AccountId != nil && *req.Msg.AccountId != "" {
		id, err := uuid.Parse(*req.Msg.AccountId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid account_id"))
		}
		accountID = &id
	}

	// Auto-categorize using the high-performance categorization service
	var categoryID *uuid.UUID
	var suggestedCategoryID *string
	if req.Msg.CategoryId != nil && *req.Msg.CategoryId != "" {
		// User provided category override
		id, err := uuid.Parse(*req.Msg.CategoryId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid category_id"))
		}
		categoryID = &id
	} else if h.catService != nil {
		// Try auto-categorization using fast Aho-Corasick engine with fuzzy fallback
		catResult, _ := h.catService.CategorizeWithFallback(ctx, userID, description, 75)
		if catResult != nil && catResult.CategoryID != nil {
			categoryID = catResult.CategoryID
			s := catResult.CategoryID.String()
			suggestedCategoryID = &s
		}
	}

	// Create the transaction via import repository
	tx := &repository.Transaction{
		ID:                  uuid.New(),
		UserID:              userID,
		AccountID:           accountID,
		CategoryID:          categoryID,
		Description:         description,
		OriginalDescription: &req.Msg.RawText,
		AmountCents:         amountMinor,
		CurrencyCode:        parsed.Currency,
		Date:                txDate,
		Source:              "manual",
	}

	err = h.importRepo.InsertTransaction(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create transaction: %w", err))
	}

	// Double-Entry: Update Active Plan Actuals
	// Pass description as category name hint for matching budget items
	if h.planSvc != nil {
		categoryName := description // Use description as category hint
		if err := h.planSvc.ProcessTransaction(ctx, userID, tx.AmountCents, tx.CategoryID, categoryName); err != nil {
			// Log error but don't fail the request (budget tracking is secondary to data integrity)
			fmt.Printf("failed to process transaction for plan: %v\n", err)
		}
	}

	// TODO: Calculate budget impact feedback
	// For now, return empty - can be enhanced later
	var budgetImpact *string

	return connect.NewResponse(&echov1.CreateManualTransactionResponse{
		Transaction:         transactionToProto(tx),
		ParsedDescription:   parsed.Description,
		ParsedAmountMinor:   parsed.AmountMinor,
		SuggestedCategoryId: suggestedCategoryID,
		BudgetImpact:        budgetImpact,
	}), nil
}

// parseNaturalLanguage extracts transaction details from natural language input.
// By default, amounts are treated as EXPENSES (negative).
// Use "+" prefix for income transactions (e.g., "+Salary 100$").
func parseNaturalLanguage(rawText string) parsedTransaction {
	result := parsedTransaction{
		Date:     timestamppb.Now().AsTime(),
		Currency: "EUR",
	}

	rawText = strings.TrimSpace(rawText)
	if rawText == "" {
		return result
	}

	// Check for income prefix "+"
	isIncome := false
	if strings.HasPrefix(rawText, "+") {
		isIncome = true
		rawText = strings.TrimPrefix(rawText, "+")
		rawText = strings.TrimSpace(rawText)
	}

	// Simple regex-based parsing for amounts
	// Matches: $1, 1$, €5, 5€, $10.50, 10,50€, etc.
	amountPattern := regexp.MustCompile(`(?:(\$|€|EUR|USD)\s*)?(\d+(?:[.,]\d{1,2})?)\s*(\$|€|EUR|USD)?`)
	matches := amountPattern.FindAllStringSubmatchIndex(rawText, -1)

	if len(matches) == 0 {
		// No amount found, entire text is description
		result.Description = rawText
		return result
	}

	// Use the last match (most likely the amount)
	match := matches[len(matches)-1]
	fullMatchStart := match[0]
	fullMatchEnd := match[1]

	// Extract amount string
	amountStart := match[4]
	amountEnd := match[5]
	amountStr := rawText[amountStart:amountEnd]

	// Detect currency from prefix or suffix
	if match[2] != -1 && match[3] != -1 {
		prefix := rawText[match[2]:match[3]]
		result.Currency = normalizeCurrency(prefix)
	} else if match[6] != -1 && match[7] != -1 {
		suffix := rawText[match[6]:match[7]]
		result.Currency = normalizeCurrency(suffix)
	}

	// Parse amount - handle European format (comma as decimal)
	amountStr = strings.Replace(amountStr, ",", ".", 1)
	if amount, err := strconv.ParseFloat(amountStr, 64); err == nil {
		amountMinor := int64(amount * 100)
		// Default to NEGATIVE (expense) unless explicitly marked as income with "+"
		if !isIncome {
			amountMinor = -amountMinor
		}
		result.AmountMinor = amountMinor
	}

	// Extract description (text without the amount part)
	description := rawText[:fullMatchStart] + rawText[fullMatchEnd:]
	description = strings.Join(strings.Fields(description), " ")
	if len(description) > 0 {
		description = strings.ToUpper(description[:1]) + description[1:]
	}
	result.Description = description

	return result
}

type parsedTransaction struct {
	Description string
	AmountMinor int64
	Currency    string
	Date        time.Time
}

func normalizeCurrency(symbol string) string {
	switch strings.ToUpper(symbol) {
	case "$", "USD":
		return "USD"
	case "€", "EUR":
		return "EUR"
	default:
		return "EUR"
	}
}

// ============================================================================
// High-Performance Categorization (Internal Integration)
// ============================================================================
// The following methods are available on the categorization service but require
// proto definitions to be exposed as API endpoints:
//
// - CategorizeFast: O(n) Aho-Corasick pattern matching
// - CategorizeBatchFast: Bulk categorization (5M+ tx/sec)
// - CategorizeWithFallback: Fast exact + fuzzy fallback
// - SuggestMerchantMatches: Fuzzy autocomplete suggestions
// - SearchMerchants: Full-text Bleve search
//
// These are integrated internally via CategorizeWithFallback in CreateManualTransaction
// and CategorizeBatchFast in the import service for maximum performance.
//
// To expose as API endpoints, add the following proto definitions:
// - CategorizeFastRequest/Response
// - CategorizeBatchFastRequest/Response
// - CategorizeWithFallbackRequest/Response
// - SuggestMerchantMatchesRequest/Response
// - SearchMerchantsRequest/Response

// ============================================================================
// Goals Management
// ============================================================================

// CreateGoal creates a new savings/spending goal
func (h *FinanceHandler) CreateGoal(
	ctx context.Context,
	req *connect.Request[echov1.CreateGoalRequest],
) (*connect.Response[echov1.CreateGoalResponse], error) {
	if h.goalsSvc == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("goals service not configured"))
	}

	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	goalType := protoToGoalType(req.Msg.Type)
	currency := "EUR"
	if req.Msg.Target != nil && req.Msg.Target.CurrencyCode != "" {
		currency = req.Msg.Target.CurrencyCode
	}

	goal, err := h.goalsSvc.CreateGoal(
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
func (h *FinanceHandler) GetGoal(
	ctx context.Context,
	req *connect.Request[echov1.GetGoalRequest],
) (*connect.Response[echov1.GetGoalResponse], error) {
	if h.goalsSvc == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("goals service not configured"))
	}

	_, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	goalID, err := uuid.Parse(req.Msg.GoalId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid goal ID"))
	}

	goal, err := h.goalsSvc.GetGoal(ctx, goalID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("goal not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	progress, err := h.goalsSvc.GetGoalProgress(ctx, goalID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&echov1.GetGoalResponse{
		Goal: goalWithProgressToProto(goal, progress),
	}), nil
}

// UpdateGoal updates a goal
func (h *FinanceHandler) UpdateGoal(
	ctx context.Context,
	req *connect.Request[echov1.UpdateGoalRequest],
) (*connect.Response[echov1.UpdateGoalResponse], error) {
	if h.goalsSvc == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("goals service not configured"))
	}

	_, err := getUserIDFromContext(ctx)
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

	var status *goalsrepo.GoalStatus
	if req.Msg.Status != nil {
		s := protoToGoalStatus(*req.Msg.Status)
		status = &s
	}

	goal, err := h.goalsSvc.UpdateGoal(ctx, goalID, name, targetMinor, endAt, status)
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
func (h *FinanceHandler) DeleteGoal(
	ctx context.Context,
	req *connect.Request[echov1.DeleteGoalRequest],
) (*connect.Response[echov1.DeleteGoalResponse], error) {
	if h.goalsSvc == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("goals service not configured"))
	}

	_, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	goalID, err := uuid.Parse(req.Msg.GoalId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid goal ID"))
	}

	if err := h.goalsSvc.DeleteGoal(ctx, goalID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("goal not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&echov1.DeleteGoalResponse{}), nil
}

// ListGoals retrieves all goals for a user
func (h *FinanceHandler) ListGoals(
	ctx context.Context,
	req *connect.Request[echov1.ListGoalsRequest],
) (*connect.Response[echov1.ListGoalsResponse], error) {
	if h.goalsSvc == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("goals service not configured"))
	}

	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var statusFilter *goalsrepo.GoalStatus
	if req.Msg.StatusFilter != nil {
		s := protoToGoalStatus(*req.Msg.StatusFilter)
		statusFilter = &s
	}

	goals, err := h.goalsSvc.ListGoals(ctx, userID, statusFilter)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoGoals := make([]*echov1.Goal, 0, len(goals))
	for _, goal := range goals {
		progress, _ := h.goalsSvc.GetGoalProgress(ctx, goal.ID)
		protoGoals = append(protoGoals, goalWithProgressToProto(goal, progress))
	}

	return connect.NewResponse(&echov1.ListGoalsResponse{
		Goals: protoGoals,
	}), nil
}

// GetGoalProgress returns detailed progress for a goal
func (h *FinanceHandler) GetGoalProgress(
	ctx context.Context,
	req *connect.Request[echov1.GetGoalProgressRequest],
) (*connect.Response[echov1.GetGoalProgressResponse], error) {
	if h.goalsSvc == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("goals service not configured"))
	}

	_, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	goalID, err := uuid.Parse(req.Msg.GoalId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid goal ID"))
	}

	progress, err := h.goalsSvc.GetGoalProgress(ctx, goalID)
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
func (h *FinanceHandler) ContributeToGoal(
	ctx context.Context,
	req *connect.Request[echov1.ContributeToGoalRequest],
) (*connect.Response[echov1.ContributeToGoalResponse], error) {
	if h.goalsSvc == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("goals service not configured"))
	}

	_, err := getUserIDFromContext(ctx)
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

	progress, milestone, err := h.goalsSvc.ContributeToGoal(ctx, goalID, req.Msg.Amount.AmountMinor, currency, note)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("goal not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &echov1.ContributeToGoalResponse{
		Goal: goalWithProgressToProto(progress.Goal, progress),
	}

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

// ============================================================================
// Subscriptions Management
// ============================================================================

// ListRecurringSubscriptions retrieves all subscriptions for a user
func (h *FinanceHandler) ListRecurringSubscriptions(
	ctx context.Context,
	req *connect.Request[echov1.ListRecurringSubscriptionsRequest],
) (*connect.Response[echov1.ListRecurringSubscriptionsResponse], error) {
	if h.subscriptionsSvc == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("subscriptions service not configured"))
	}

	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var statusFilter *subscriptionsrepo.RecurringStatus
	if req.Msg.StatusFilter != nil {
		s := protoToSubscriptionStatus(*req.Msg.StatusFilter)
		statusFilter = &s
	}

	subs, err := h.subscriptionsSvc.ListSubscriptions(ctx, userID, statusFilter, req.Msg.IncludeCanceled)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	totalMonthly, activeCount, _ := h.subscriptionsSvc.GetTotalMonthlySubscriptionCost(ctx, userID)

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
func (h *FinanceHandler) DetectRecurringSubscriptions(
	ctx context.Context,
	req *connect.Request[echov1.DetectRecurringSubscriptionsRequest],
) (*connect.Response[echov1.DetectRecurringSubscriptionsResponse], error) {
	if h.subscriptionsSvc == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("subscriptions service not configured"))
	}

	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	since := time.Now().AddDate(0, -6, 0)
	if req.Msg.Since != nil {
		since = req.Msg.Since.AsTime()
	}

	minOccurrences := 2
	if req.Msg.MinOccurrences != nil {
		minOccurrences = int(*req.Msg.MinOccurrences)
	}

	result, err := h.subscriptionsSvc.DetectSubscriptions(ctx, userID, since, minOccurrences)
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
func (h *FinanceHandler) UpdateSubscriptionStatus(
	ctx context.Context,
	req *connect.Request[echov1.UpdateSubscriptionStatusRequest],
) (*connect.Response[echov1.UpdateSubscriptionStatusResponse], error) {
	if h.subscriptionsSvc == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("subscriptions service not configured"))
	}

	_, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	subID, err := uuid.Parse(req.Msg.SubscriptionId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid subscription ID"))
	}

	status := protoToSubscriptionStatus(req.Msg.Status)

	sub, err := h.subscriptionsSvc.UpdateStatus(ctx, subID, status)
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
func (h *FinanceHandler) GetSubscriptionReviewChecklist(
	ctx context.Context,
	req *connect.Request[echov1.GetSubscriptionReviewChecklistRequest],
) (*connect.Response[echov1.GetSubscriptionReviewChecklistResponse], error) {
	if h.subscriptionsSvc == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("subscriptions service not configured"))
	}

	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	items, potentialSavings, err := h.subscriptionsSvc.GetReviewChecklist(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoItems := make([]*echov1.SubscriptionReviewItem, 0, len(items))
	for _, item := range items {
		protoItems = append(protoItems, reviewItemToProto(item))
	}

	summary := "All subscriptions look good!"
	if len(items) > 0 {
		summary = fmt.Sprintf("%d subscription(s) to review", len(items))
	}

	return connect.NewResponse(&echov1.GetSubscriptionReviewChecklistResponse{
		Items:                   protoItems,
		PotentialMonthlySavings: toMoney(potentialSavings, "EUR"),
		Summary:                 summary,
	}), nil
}

// ============================================================================
// Helper Functions for Goals and Subscriptions
// ============================================================================

func getUserIDFromContext(ctx context.Context) (uuid.UUID, error) {
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

func toMoney(cents int64, currency string) *echov1.Money {
	if currency == "" {
		currency = "EUR"
	}
	return &echov1.Money{
		AmountMinor:  cents,
		CurrencyCode: currency,
	}
}

func goalToProto(goal *goalsrepo.Goal) *echov1.Goal {
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

func goalWithProgressToProto(goal *goalsrepo.Goal, progress *goalsservice.GoalProgress) *echov1.Goal {
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

func goalTypeToProto(t goalsrepo.GoalType) echov1.GoalType {
	switch t {
	case goalsrepo.GoalTypeSave:
		return echov1.GoalType_GOAL_TYPE_SAVE
	case goalsrepo.GoalTypePayDownDebt:
		return echov1.GoalType_GOAL_TYPE_PAY_DOWN_DEBT
	case goalsrepo.GoalTypeSpendCap:
		return echov1.GoalType_GOAL_TYPE_SPEND_CAP
	default:
		return echov1.GoalType_GOAL_TYPE_UNSPECIFIED
	}
}

func protoToGoalType(t echov1.GoalType) goalsrepo.GoalType {
	switch t {
	case echov1.GoalType_GOAL_TYPE_SAVE:
		return goalsrepo.GoalTypeSave
	case echov1.GoalType_GOAL_TYPE_PAY_DOWN_DEBT:
		return goalsrepo.GoalTypePayDownDebt
	case echov1.GoalType_GOAL_TYPE_SPEND_CAP:
		return goalsrepo.GoalTypeSpendCap
	default:
		return goalsrepo.GoalTypeSave
	}
}

func goalStatusToProto(s goalsrepo.GoalStatus) echov1.GoalStatus {
	switch s {
	case goalsrepo.GoalStatusActive:
		return echov1.GoalStatus_GOAL_STATUS_ACTIVE
	case goalsrepo.GoalStatusPaused:
		return echov1.GoalStatus_GOAL_STATUS_PAUSED
	case goalsrepo.GoalStatusCompleted:
		return echov1.GoalStatus_GOAL_STATUS_COMPLETED
	case goalsrepo.GoalStatusArchived:
		return echov1.GoalStatus_GOAL_STATUS_ARCHIVED
	default:
		return echov1.GoalStatus_GOAL_STATUS_UNSPECIFIED
	}
}

func protoToGoalStatus(s echov1.GoalStatus) goalsrepo.GoalStatus {
	switch s {
	case echov1.GoalStatus_GOAL_STATUS_ACTIVE:
		return goalsrepo.GoalStatusActive
	case echov1.GoalStatus_GOAL_STATUS_PAUSED:
		return goalsrepo.GoalStatusPaused
	case echov1.GoalStatus_GOAL_STATUS_COMPLETED:
		return goalsrepo.GoalStatusCompleted
	case echov1.GoalStatus_GOAL_STATUS_ARCHIVED:
		return goalsrepo.GoalStatusArchived
	default:
		return goalsrepo.GoalStatusActive
	}
}

func subscriptionToProto(sub *subscriptionsrepo.RecurringSubscription) *echov1.RecurringSubscription {
	proto := &echov1.RecurringSubscription{
		Id:              sub.ID.String(),
		UserId:          sub.UserID.String(),
		MerchantName:    sub.MerchantName,
		Amount:          toMoney(sub.AmountMinor, sub.CurrencyCode),
		Cadence:         cadenceToProto(sub.Cadence),
		Status:          subscriptionStatusToProto(sub.Status),
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

func cadenceToProto(c subscriptionsrepo.RecurringCadence) echov1.RecurringCadence {
	switch c {
	case subscriptionsrepo.RecurringCadenceWeekly:
		return echov1.RecurringCadence_RECURRING_CADENCE_WEEKLY
	case subscriptionsrepo.RecurringCadenceMonthly:
		return echov1.RecurringCadence_RECURRING_CADENCE_MONTHLY
	case subscriptionsrepo.RecurringCadenceQuarterly:
		return echov1.RecurringCadence_RECURRING_CADENCE_QUARTERLY
	case subscriptionsrepo.RecurringCadenceAnnual:
		return echov1.RecurringCadence_RECURRING_CADENCE_ANNUAL
	default:
		return echov1.RecurringCadence_RECURRING_CADENCE_UNKNOWN
	}
}

func subscriptionStatusToProto(s subscriptionsrepo.RecurringStatus) echov1.RecurringStatus {
	switch s {
	case subscriptionsrepo.RecurringStatusActive:
		return echov1.RecurringStatus_RECURRING_STATUS_ACTIVE
	case subscriptionsrepo.RecurringStatusPaused:
		return echov1.RecurringStatus_RECURRING_STATUS_PAUSED
	case subscriptionsrepo.RecurringStatusCanceled:
		return echov1.RecurringStatus_RECURRING_STATUS_CANCELED
	default:
		return echov1.RecurringStatus_RECURRING_STATUS_UNSPECIFIED
	}
}

func protoToSubscriptionStatus(s echov1.RecurringStatus) subscriptionsrepo.RecurringStatus {
	switch s {
	case echov1.RecurringStatus_RECURRING_STATUS_ACTIVE:
		return subscriptionsrepo.RecurringStatusActive
	case echov1.RecurringStatus_RECURRING_STATUS_PAUSED:
		return subscriptionsrepo.RecurringStatusPaused
	case echov1.RecurringStatus_RECURRING_STATUS_CANCELED:
		return subscriptionsrepo.RecurringStatusCanceled
	default:
		return subscriptionsrepo.RecurringStatusActive
	}
}

func reviewItemToProto(item *subscriptionsservice.SubscriptionReviewItem) *echov1.SubscriptionReviewItem {
	return &echov1.SubscriptionReviewItem{
		Subscription:      subscriptionToProto(item.Subscription),
		Reason:            reviewReasonToProto(item.Reason),
		ReasonMessage:     item.ReasonMessage,
		RecommendedCancel: item.RecommendedCancel,
	}
}

func reviewReasonToProto(r subscriptionsservice.ReviewReason) echov1.SubscriptionReviewReason {
	switch r {
	case subscriptionsservice.ReviewReasonUnused:
		return echov1.SubscriptionReviewReason_SUBSCRIPTION_REVIEW_REASON_UNUSED
	case subscriptionsservice.ReviewReasonPriceIncrease:
		return echov1.SubscriptionReviewReason_SUBSCRIPTION_REVIEW_REASON_PRICE_INCREASE
	case subscriptionsservice.ReviewReasonDuplicate:
		return echov1.SubscriptionReviewReason_SUBSCRIPTION_REVIEW_REASON_DUPLICATE
	case subscriptionsservice.ReviewReasonHighCost:
		return echov1.SubscriptionReviewReason_SUBSCRIPTION_REVIEW_REASON_HIGH_COST
	case subscriptionsservice.ReviewReasonNew:
		return echov1.SubscriptionReviewReason_SUBSCRIPTION_REVIEW_REASON_NEW
	default:
		return echov1.SubscriptionReviewReason_SUBSCRIPTION_REVIEW_REASON_UNSPECIFIED
	}
}
