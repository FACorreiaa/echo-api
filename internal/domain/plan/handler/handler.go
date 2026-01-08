// Package handler provides gRPC handlers for user plans.
package handler

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"buf.build/gen/go/echo-tracker/echo/connectrpc/go/echo/v1/echov1connect"
	echov1 "buf.build/gen/go/echo-tracker/echo/protocolbuffers/go/echo/v1"
	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/plan/repository"
	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/plan/service"
	"github.com/FACorreiaa/smart-finance-tracker/pkg/interceptors"
	"github.com/FACorreiaa/smart-finance-tracker/pkg/storage"
)

// PlanHandler implements the PlanService RPC handlers
type PlanHandler struct {
	svc     *service.PlanService
	storage storage.Storage
}

// Ensure PlanHandler implements PlanServiceHandler
var _ echov1connect.PlanServiceHandler = (*PlanHandler)(nil)

// NewPlanHandler creates a new plan handler
func NewPlanHandler(svc *service.PlanService, storage storage.Storage) *PlanHandler {
	return &PlanHandler{svc: svc, storage: storage}
}

// CreatePlan creates a new financial plan
func (h *PlanHandler) CreatePlan(ctx context.Context, req *connect.Request[echov1.CreatePlanRequest]) (*connect.Response[echov1.CreatePlanResponse], error) {
	userIDStr, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user not authenticated"))
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	input := &service.CreatePlanInput{
		Name:         req.Msg.Name,
		CurrencyCode: req.Msg.CurrencyCode,
	}
	if req.Msg.Description != "" {
		input.Description = &req.Msg.Description
	}

	// Convert proto category groups to service input
	for _, g := range req.Msg.CategoryGroups {
		groupInput := service.CreateCategoryGroupInput{
			Name:          g.Name,
			TargetPercent: g.TargetPercent,
			Labels:        g.Labels,
		}
		if g.Color != "" {
			groupInput.Color = &g.Color
		}

		for _, c := range g.Categories {
			catInput := service.CreateCategoryInput{
				Name:   c.Name,
				Labels: c.Labels,
			}
			if c.Icon != "" {
				catInput.Icon = &c.Icon
			}

			for _, item := range c.Items {
				itemInput := service.CreateItemInput{
					Name:          item.Name,
					BudgetedMinor: item.BudgetedMinor,
					WidgetType:    toRepoWidgetType(item.WidgetType),
					FieldType:     toRepoFieldType(item.FieldType),
					Labels:        item.Labels,
				}
				catInput.Items = append(catInput.Items, itemInput)
			}

			groupInput.Categories = append(groupInput.Categories, catInput)
		}

		input.CategoryGroups = append(input.CategoryGroups, groupInput)
	}

	plan, err := h.svc.CreatePlan(ctx, userID, input)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&echov1.CreatePlanResponse{
		Plan: toProtoPlan(plan),
	}), nil
}

// GetPlan retrieves a plan by ID
func (h *PlanHandler) GetPlan(ctx context.Context, req *connect.Request[echov1.GetPlanRequest]) (*connect.Response[echov1.GetPlanResponse], error) {
	userIDStr, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user not authenticated"))
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	planID, err := uuid.Parse(req.Msg.PlanId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	details, err := h.svc.GetPlanWithDetails(ctx, userID, planID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if details == nil {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}

	return connect.NewResponse(&echov1.GetPlanResponse{
		Plan: toProtoPlanWithDetails(details),
	}), nil
}

// ListPlans lists all plans for the current user
func (h *PlanHandler) ListPlans(ctx context.Context, req *connect.Request[echov1.ListPlansRequest]) (*connect.Response[echov1.ListPlansResponse], error) {
	userIDStr, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user not authenticated"))
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	var status *repository.PlanStatus
	if req.Msg.StatusFilter != echov1.PlanStatus_PLAN_STATUS_UNSPECIFIED {
		s := toRepoPlanStatus(req.Msg.StatusFilter)
		status = &s
	}

	limit := int(req.Msg.Limit)
	offset := int(req.Msg.Offset)

	plans, total, err := h.svc.ListPlans(ctx, userID, status, limit, offset)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var protoPlans []*echov1.UserPlan
	for _, p := range plans {
		protoPlans = append(protoPlans, toProtoPlan(p))
	}

	return connect.NewResponse(&echov1.ListPlansResponse{
		Plans:      protoPlans,
		TotalCount: int32(total),
	}), nil
}

// UpdatePlan updates an existing plan
func (h *PlanHandler) UpdatePlan(ctx context.Context, req *connect.Request[echov1.UpdatePlanRequest]) (*connect.Response[echov1.UpdatePlanResponse], error) {
	userIDStr, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user not authenticated"))
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	planID, err := uuid.Parse(req.Msg.PlanId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	var name, desc *string
	if req.Msg.Name != nil {
		name = req.Msg.Name
	}
	if req.Msg.Description != nil {
		desc = req.Msg.Description
	}

	plan, err := h.svc.UpdatePlan(ctx, userID, planID, name, desc)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Update individual items if provided
	for _, item := range req.Msg.Items {
		itemID, err := uuid.Parse(item.ItemId)
		if err != nil {
			continue
		}
		if err := h.svc.UpdatePlanItem(ctx, userID, planID, itemID, item.BudgetedMinor); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&echov1.UpdatePlanResponse{
		Plan: toProtoPlan(plan),
	}), nil
}

// DeletePlan soft-deletes a plan
func (h *PlanHandler) DeletePlan(ctx context.Context, req *connect.Request[echov1.DeletePlanRequest]) (*connect.Response[echov1.DeletePlanResponse], error) {
	userIDStr, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user not authenticated"))
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	planID, err := uuid.Parse(req.Msg.PlanId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if err := h.svc.DeletePlan(ctx, userID, planID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&echov1.DeletePlanResponse{}), nil
}

// SetActivePlan marks a plan as the active/live plan
func (h *PlanHandler) SetActivePlan(ctx context.Context, req *connect.Request[echov1.SetActivePlanRequest]) (*connect.Response[echov1.SetActivePlanResponse], error) {
	userIDStr, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user not authenticated"))
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	planID, err := uuid.Parse(req.Msg.PlanId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	plan, err := h.svc.SetActivePlan(ctx, userID, planID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&echov1.SetActivePlanResponse{
		Plan: toProtoPlan(plan),
	}), nil
}

// DuplicatePlan creates a copy of an existing plan
func (h *PlanHandler) DuplicatePlan(ctx context.Context, req *connect.Request[echov1.DuplicatePlanRequest]) (*connect.Response[echov1.DuplicatePlanResponse], error) {
	userIDStr, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user not authenticated"))
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	planID, err := uuid.Parse(req.Msg.PlanId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	plan, err := h.svc.DuplicatePlan(ctx, userID, planID, req.Msg.NewName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&echov1.DuplicatePlanResponse{
		Plan: toProtoPlan(plan),
	}), nil
}

// ImportPlanFromExcel imports a plan from an uploaded Excel file
func (h *PlanHandler) ImportPlanFromExcel(ctx context.Context, req *connect.Request[echov1.ImportPlanFromExcelRequest]) (*connect.Response[echov1.ImportPlanFromExcelResponse], error) {
	userIDStr, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user not authenticated"))
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	fileID, err := uuid.Parse(req.Msg.FileId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid file ID"))
	}

	// Get file from storage
	reader, err := h.storage.GetReader(ctx, userID, fileID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	defer reader.Close()

	// Config from request
	config := &service.ExcelImportConfig{
		CategoryColumn: "A",
		ValueColumn:    "B",
		HeaderRow:      1,
	}
	if req.Msg.Mapping != nil {
		if req.Msg.Mapping.CategoryColumn != "" {
			config.CategoryColumn = req.Msg.Mapping.CategoryColumn
		}
		if req.Msg.Mapping.ValueColumn != "" {
			config.ValueColumn = req.Msg.Mapping.ValueColumn
		}
		if req.Msg.Mapping.HeaderRow > 0 {
			config.HeaderRow = int(req.Msg.Mapping.HeaderRow)
		}
	}

	// Generate plan name
	planName := "Imported Plan"
	if req.Msg.SheetName != "" {
		planName = req.Msg.SheetName
	}

	// Import
	importResult, err := h.svc.ImportFromExcel(ctx, userID, reader, req.Msg.SheetName, config, planName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Get plan with full category structure for response
	planWithDetails, err := h.svc.GetPlanWithDetails(ctx, userID, importResult.Plan.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get plan details: %w", err))
	}

	return connect.NewResponse(&echov1.ImportPlanFromExcelResponse{
		Plan:               toProtoPlanWithDetails(planWithDetails),
		CategoriesImported: int32(importResult.CategoriesImported),
		ItemsImported:      int32(importResult.ItemsImported),
	}), nil
}

// AnalyzeExcelForPlan analyzes an Excel file to determine structure
func (h *PlanHandler) AnalyzeExcelForPlan(ctx context.Context, req *connect.Request[echov1.AnalyzeExcelForPlanRequest]) (*connect.Response[echov1.AnalyzeExcelForPlanResponse], error) {
	userIDStr, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user not authenticated"))
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	fileID, err := uuid.Parse(req.Msg.FileId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid file ID"))
	}

	// Get file from storage
	reader, err := h.storage.GetReader(ctx, userID, fileID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	defer reader.Close()

	// Analyze
	result, err := h.svc.AnalyzeExcel(reader)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Convert to proto
	sheets := make([]*echov1.ExcelSheetAnalysis, len(result.Sheets))
	for i, s := range result.Sheets {
		analysis := &echov1.ExcelSheetAnalysis{
			Name:               s.Name,
			IsLivingPlan:       s.IsLivingPlan,
			RowCount:           int32(s.RowCount),
			FormulaCount:       int32(s.FormulaCount),
			DetectedCategories: s.DetectedCategories,
			MonthColumns:       s.MonthColumns,
		}

		// Include detected column mapping if available
		if s.DetectedMapping != nil {
			analysis.DetectedMapping = &echov1.DetectedColumnMapping{
				CategoryColumn:   s.DetectedMapping.CategoryColumn,
				ValueColumn:      s.DetectedMapping.ValueColumn,
				HeaderRow:        int32(s.DetectedMapping.HeaderRow),
				PercentageColumn: s.DetectedMapping.PercentageColumn,
				Confidence:       s.DetectedMapping.Confidence,
			}
		}

		// Include preview rows for UI display
		if len(s.PreviewRows) > 0 {
			analysis.PreviewRows = make([]*echov1.ExcelPreviewRow, len(s.PreviewRows))
			for j, row := range s.PreviewRows {
				analysis.PreviewRows[j] = &echov1.ExcelPreviewRow{
					Cells: row,
				}
			}
		}

		sheets[i] = analysis
	}

	return connect.NewResponse(&echov1.AnalyzeExcelForPlanResponse{
		Sheets:         sheets,
		SuggestedSheet: result.SuggestedSheet,
	}), nil
}

// ComputePlanActuals syncs actual spending from transactions to plan items
func (h *PlanHandler) ComputePlanActuals(ctx context.Context, req *connect.Request[echov1.ComputePlanActualsRequest]) (*connect.Response[echov1.ComputePlanActualsResponse], error) {
	userIDStr, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user not authenticated"))
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	planID, err := uuid.Parse(req.Msg.GetPlanId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid plan_id"))
	}

	// Build input from request
	input := &service.ComputePlanActualsInput{
		Persist: req.Msg.GetPersist(),
	}

	// Parse start/end dates if provided
	if req.Msg.GetStartDate() != nil {
		input.StartDate = req.Msg.GetStartDate().AsTime()
	}
	if req.Msg.GetEndDate() != nil {
		input.EndDate = req.Msg.GetEndDate().AsTime()
	}

	// Call service
	result, err := h.svc.ComputePlanActuals(ctx, userID, planID, input)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Convert unmatched items
	unmatchedItems := make([]*echov1.UnmatchedItem, len(result.UnmatchedItems))
	for i, item := range result.UnmatchedItems {
		unmatchedItems[i] = &echov1.UnmatchedItem{
			ItemId:   item.ItemID.String(),
			ItemName: item.ItemName,
			Reason:   item.Reason,
		}
	}

	return connect.NewResponse(&echov1.ComputePlanActualsResponse{
		Plan:                toProtoPlanWithDetails(result.Plan),
		ItemsUpdated:        int32(result.ItemsUpdated),
		TransactionsMatched: int32(result.TransactionsMatched),
		UnmatchedItems:      unmatchedItems,
	}), nil
}

// ============================================================================
// Item Config Management
// ============================================================================

// ListItemConfigs returns all item configs for the current user
func (h *PlanHandler) ListItemConfigs(ctx context.Context, req *connect.Request[echov1.ListItemConfigsRequest]) (*connect.Response[echov1.ListItemConfigsResponse], error) {
	userID, err := interceptors.UserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user not authenticated"))
	}

	configs, err := h.svc.ListItemConfigs(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list item configs: %w", err))
	}

	protoConfigs := make([]*echov1.ItemConfig, len(configs))
	for i, c := range configs {
		protoConfigs[i] = toProtoItemConfig(c)
	}

	return connect.NewResponse(&echov1.ListItemConfigsResponse{
		Configs: protoConfigs,
	}), nil
}

// CreateItemConfig creates a new custom item type
func (h *PlanHandler) CreateItemConfig(ctx context.Context, req *connect.Request[echov1.CreateItemConfigRequest]) (*connect.Response[echov1.CreateItemConfigResponse], error) {
	userID, err := interceptors.UserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user not authenticated"))
	}

	config := &repository.ItemConfig{
		UserID:    userID,
		Label:     req.Msg.Label,
		ShortCode: req.Msg.ShortCode,
		Behavior:  toRepoItemBehavior(req.Msg.Behavior),
		TargetTab: toRepoTargetTab(req.Msg.TargetTab),
		ColorHex:  req.Msg.ColorHex,
		Icon:      req.Msg.Icon,
		IsSystem:  false,
	}

	if err := h.svc.CreateItemConfig(ctx, config); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create item config: %w", err))
	}

	return connect.NewResponse(&echov1.CreateItemConfigResponse{
		Config: toProtoItemConfig(config),
	}), nil
}

// UpdateItemConfig updates an existing item config
func (h *PlanHandler) UpdateItemConfig(ctx context.Context, req *connect.Request[echov1.UpdateItemConfigRequest]) (*connect.Response[echov1.UpdateItemConfigResponse], error) {
	configID, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid config ID"))
	}

	// Get existing config first
	config, err := h.svc.GetItemConfigByID(ctx, configID)
	if err != nil || config == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("item config not found"))
	}

	// Apply updates
	if req.Msg.Label != nil {
		config.Label = *req.Msg.Label
	}
	if req.Msg.ShortCode != nil {
		config.ShortCode = *req.Msg.ShortCode
	}
	if req.Msg.Behavior != nil {
		config.Behavior = toRepoItemBehavior(*req.Msg.Behavior)
	}
	if req.Msg.TargetTab != nil {
		config.TargetTab = toRepoTargetTab(*req.Msg.TargetTab)
	}
	if req.Msg.ColorHex != nil {
		config.ColorHex = *req.Msg.ColorHex
	}
	if req.Msg.Icon != nil {
		config.Icon = *req.Msg.Icon
	}
	if req.Msg.SortOrder != nil {
		config.SortOrder = int(*req.Msg.SortOrder)
	}

	if err := h.svc.UpdateItemConfig(ctx, config); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update item config: %w", err))
	}

	return connect.NewResponse(&echov1.UpdateItemConfigResponse{
		Config: toProtoItemConfig(config),
	}), nil
}

// DeleteItemConfig deletes a custom item config
func (h *PlanHandler) DeleteItemConfig(ctx context.Context, req *connect.Request[echov1.DeleteItemConfigRequest]) (*connect.Response[echov1.DeleteItemConfigResponse], error) {
	configID, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid config ID"))
	}

	if err := h.svc.DeleteItemConfig(ctx, configID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete item config: %w", err))
	}

	return connect.NewResponse(&echov1.DeleteItemConfigResponse{
		Success: true,
	}), nil
}

// ============================================================================
// Conversion helpers
// ============================================================================

func toProtoPlan(p *repository.UserPlan) *echov1.UserPlan {
	if p == nil {
		return nil
	}
	plan := &echov1.UserPlan{
		Id:            p.ID.String(),
		Name:          p.Name,
		Status:        toProtoPlanStatus(p.Status),
		SourceType:    toProtoSourceType(p.SourceType),
		TotalIncome:   &echov1.Money{AmountMinor: p.TotalIncomeMinor, CurrencyCode: p.CurrencyCode},
		TotalExpenses: &echov1.Money{AmountMinor: p.TotalExpensesMinor, CurrencyCode: p.CurrencyCode},
		Surplus:       &echov1.Money{AmountMinor: p.TotalIncomeMinor - p.TotalExpensesMinor, CurrencyCode: p.CurrencyCode},
	}
	if p.Description != nil {
		plan.Description = *p.Description
	}
	return plan
}

func toProtoPlanWithDetails(d *service.PlanWithDetails) *echov1.UserPlan {
	if d == nil || d.Plan == nil {
		return nil
	}
	plan := toProtoPlan(d.Plan)

	// Build category groups with nested structure
	groupMap := make(map[uuid.UUID]*echov1.PlanCategoryGroup)
	for _, g := range d.Groups {
		groupMap[g.ID] = &echov1.PlanCategoryGroup{
			Id:            g.ID.String(),
			Name:          g.Name,
			TargetPercent: g.TargetPercent,
		}
		if g.Color != nil {
			groupMap[g.ID].Color = *g.Color
		}
	}

	categoryMap := make(map[uuid.UUID]*echov1.PlanCategory)
	for _, c := range d.Categories {
		cat := &echov1.PlanCategory{
			Id:   c.ID.String(),
			Name: c.Name,
		}
		if c.Icon != nil {
			cat.Icon = *c.Icon
		}
		categoryMap[c.ID] = cat

		if c.GroupID != nil {
			if group, ok := groupMap[*c.GroupID]; ok {
				group.Categories = append(group.Categories, cat)
			}
		}
	}

	for _, i := range d.Items {
		item := &echov1.PlanItem{
			Id:       i.ID.String(),
			Name:     i.Name,
			Budgeted: &echov1.Money{AmountMinor: i.BudgetedMinor, CurrencyCode: d.Plan.CurrencyCode},
			Actual:   &echov1.Money{AmountMinor: i.ActualMinor, CurrencyCode: d.Plan.CurrencyCode},
		}
		if i.CategoryID != nil {
			if cat, ok := categoryMap[*i.CategoryID]; ok {
				cat.Items = append(cat.Items, item)
			}
		}
	}

	for _, g := range groupMap {
		plan.CategoryGroups = append(plan.CategoryGroups, g)
	}

	return plan
}

func toProtoPlanStatus(s repository.PlanStatus) echov1.PlanStatus {
	switch s {
	case repository.PlanStatusDraft:
		return echov1.PlanStatus_PLAN_STATUS_DRAFT
	case repository.PlanStatusActive:
		return echov1.PlanStatus_PLAN_STATUS_ACTIVE
	case repository.PlanStatusArchived:
		return echov1.PlanStatus_PLAN_STATUS_ARCHIVED
	default:
		return echov1.PlanStatus_PLAN_STATUS_UNSPECIFIED
	}
}

func toRepoPlanStatus(s echov1.PlanStatus) repository.PlanStatus {
	switch s {
	case echov1.PlanStatus_PLAN_STATUS_DRAFT:
		return repository.PlanStatusDraft
	case echov1.PlanStatus_PLAN_STATUS_ACTIVE:
		return repository.PlanStatusActive
	case echov1.PlanStatus_PLAN_STATUS_ARCHIVED:
		return repository.PlanStatusArchived
	default:
		return repository.PlanStatusDraft
	}
}

func toProtoSourceType(s repository.PlanSourceType) echov1.PlanSourceType {
	switch s {
	case repository.PlanSourceManual:
		return echov1.PlanSourceType_PLAN_SOURCE_TYPE_MANUAL
	case repository.PlanSourceExcel:
		return echov1.PlanSourceType_PLAN_SOURCE_TYPE_EXCEL
	case repository.PlanSourceTemplate:
		return echov1.PlanSourceType_PLAN_SOURCE_TYPE_TEMPLATE
	default:
		return echov1.PlanSourceType_PLAN_SOURCE_TYPE_UNSPECIFIED
	}
}

func toRepoWidgetType(w echov1.WidgetType) repository.WidgetType {
	switch w {
	case echov1.WidgetType_WIDGET_TYPE_INPUT:
		return repository.WidgetTypeInput
	case echov1.WidgetType_WIDGET_TYPE_SLIDER:
		return repository.WidgetTypeSlider
	case echov1.WidgetType_WIDGET_TYPE_TOGGLE:
		return repository.WidgetTypeToggle
	case echov1.WidgetType_WIDGET_TYPE_READONLY:
		return repository.WidgetTypeReadonly
	default:
		return repository.WidgetTypeInput
	}
}

func toRepoFieldType(f echov1.FieldType) repository.FieldType {
	switch f {
	case echov1.FieldType_FIELD_TYPE_CURRENCY:
		return repository.FieldTypeCurrency
	case echov1.FieldType_FIELD_TYPE_PERCENTAGE:
		return repository.FieldTypePercentage
	case echov1.FieldType_FIELD_TYPE_NUMBER:
		return repository.FieldTypeNumber
	case echov1.FieldType_FIELD_TYPE_TEXT:
		return repository.FieldTypeText
	default:
		return repository.FieldTypeCurrency
	}
}
