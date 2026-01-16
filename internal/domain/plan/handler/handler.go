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
	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/plan/excel"
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
					ItemType:      toRepoItemType(item.ItemType),
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
		Plan: toProtoPlanWithDetails(plan),
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

// UpdatePlanStructure updates the entire structure of a plan
func (h *PlanHandler) UpdatePlanStructure(ctx context.Context, req *connect.Request[echov1.UpdatePlanStructureRequest]) (*connect.Response[echov1.UpdatePlanStructureResponse], error) {
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

	var groups []service.CreateCategoryGroupInput
	for _, g := range req.Msg.CategoryGroups {
		groupInput := service.CreateCategoryGroupInput{
			Name:          g.Name,
			TargetPercent: g.TargetPercent,
			Labels:        g.Labels,
		}
		if g.Id != nil {
			if id, err := uuid.Parse(*g.Id); err == nil {
				groupInput.ID = &id
			}
		}
		if g.Color != "" {
			groupInput.Color = &g.Color
		}

		for _, c := range g.Categories {
			catInput := service.CreateCategoryInput{
				Name:   c.Name,
				Labels: c.Labels,
			}
			if c.Id != nil {
				if id, err := uuid.Parse(*c.Id); err == nil {
					catInput.ID = &id
				}
			}
			if c.Icon != "" {
				catInput.Icon = &c.Icon
			}

			for _, item := range c.Items {
				itemInput := service.CreateItemInput{
					Name:               item.Name,
					BudgetedMinor:      item.BudgetedMinor,
					WidgetType:         toRepoWidgetType(item.WidgetType),
					FieldType:          toRepoFieldType(item.FieldType),
					Labels:             item.Labels,
					ItemType:           toRepoItemType(item.ItemType),
					InitialActualMinor: item.InitialActualMinor,
				}
				if item.Id != nil {
					if id, err := uuid.Parse(*item.Id); err == nil {
						itemInput.ID = &id
					}
				}
				if item.ConfigId != nil {
					itemInput.ConfigID = item.ConfigId
				}
				catInput.Items = append(catInput.Items, itemInput)
			}

			groupInput.Categories = append(groupInput.Categories, catInput)
		}

		groups = append(groups, groupInput)
	}

	plan, err := h.svc.UpdatePlanStructure(ctx, userID, planID, groups)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&echov1.UpdatePlanStructureResponse{
		Plan: toProtoPlan(plan),
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

// AnalyzeExcelTree performs ML-based structural analysis returning a hierarchical tree
func (h *PlanHandler) AnalyzeExcelTree(ctx context.Context, req *connect.Request[echov1.AnalyzeExcelTreeRequest]) (*connect.Response[echov1.AnalyzeExcelTreeResponse], error) {
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

	// Use the structural analyzer with ML support
	result, err := h.svc.AnalyzeExcelTree(
		reader,
		req.Msg.SheetName,
		req.Msg.CategoryColumn,
		req.Msg.ValueColumn,
		int(req.Msg.StartRow),
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Convert to proto format
	nodes := convertAnalysisNodes(result.Nodes)

	resp := &echov1.AnalyzeExcelTreeResponse{
		SheetName:         result.SheetName,
		Nodes:             nodes,
		TotalGroups:       int32(result.TotalGroups),
		TotalItems:        int32(result.TotalItems),
		OverallConfidence: result.OverallConfidence,
	}

	// Include detected mapping if available
	if result.DetectedMapping != nil {
		resp.DetectedMapping = &echov1.DetectedColumnMapping{
			CategoryColumn:   result.DetectedMapping.CategoryColumn,
			ValueColumn:      result.DetectedMapping.ValueColumn,
			HeaderRow:        int32(result.DetectedMapping.HeaderRow),
			PercentageColumn: result.DetectedMapping.PercentageColumn,
			Confidence:       result.DetectedMapping.Confidence,
		}
	}

	return connect.NewResponse(resp), nil
}

// convertAnalysisNodes converts internal AnalysisNode to proto format
func convertAnalysisNodes(nodes []excel.AnalysisNode) []*echov1.AnalysisNode {
	result := make([]*echov1.AnalysisNode, len(nodes))
	for i, n := range nodes {
		protoNode := &echov1.AnalysisNode{
			Id:         n.ID,
			Name:       n.Name,
			ValueMinor: int64(n.Value * 100), // Convert to minor units
			Confidence: n.Confidence,
			ExcelCell:  n.ExcelCell,
			ExcelRow:   int32(n.ExcelRow),
			Formula:    n.Formula,
		}

		// Convert node type
		switch n.Type {
		case excel.NodeTypeGroup:
			protoNode.Type = echov1.AnalysisNodeType_ANALYSIS_NODE_TYPE_GROUP
		case excel.NodeTypeItem:
			protoNode.Type = echov1.AnalysisNodeType_ANALYSIS_NODE_TYPE_ITEM
		case excel.NodeTypeIgnore:
			protoNode.Type = echov1.AnalysisNodeType_ANALYSIS_NODE_TYPE_IGNORE
		}

		// Convert item tag
		switch n.Tag {
		case excel.TagBudget:
			protoNode.Tag = echov1.AnalysisItemTag_ANALYSIS_ITEM_TAG_BUDGET
		case excel.TagRecurring:
			protoNode.Tag = echov1.AnalysisItemTag_ANALYSIS_ITEM_TAG_RECURRING
		case excel.TagSavings:
			protoNode.Tag = echov1.AnalysisItemTag_ANALYSIS_ITEM_TAG_SAVINGS
		case excel.TagIncome:
			protoNode.Tag = echov1.AnalysisItemTag_ANALYSIS_ITEM_TAG_INCOME
		case excel.TagDebt:
			protoNode.Tag = echov1.AnalysisItemTag_ANALYSIS_ITEM_TAG_DEBT
		}

		// Recursively convert children
		if len(n.Children) > 0 {
			protoNode.Children = convertAnalysisNodes(n.Children)
		}

		result[i] = protoNode
	}
	return result
}

// LearnFromExcelCorrection teaches the ML model from user corrections
func (h *PlanHandler) LearnFromExcelCorrection(ctx context.Context, req *connect.Request[echov1.LearnFromExcelCorrectionRequest]) (*connect.Response[echov1.LearnFromExcelCorrectionResponse], error) {
	_, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user not authenticated"))
	}

	// Convert proto tag to internal tag
	var tag string
	switch req.Msg.CorrectTag {
	case echov1.AnalysisItemTag_ANALYSIS_ITEM_TAG_BUDGET:
		tag = "B"
	case echov1.AnalysisItemTag_ANALYSIS_ITEM_TAG_RECURRING:
		tag = "R"
	case echov1.AnalysisItemTag_ANALYSIS_ITEM_TAG_SAVINGS:
		tag = "S"
	case echov1.AnalysisItemTag_ANALYSIS_ITEM_TAG_INCOME:
		tag = "IN"
	case echov1.AnalysisItemTag_ANALYSIS_ITEM_TAG_DEBT:
		tag = "D"
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid tag"))
	}

	// Teach the model
	h.svc.LearnFromCorrection(req.Msg.ItemName, tag)

	return connect.NewResponse(&echov1.LearnFromExcelCorrectionResponse{
		Success: true,
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
			ItemType: toProtoItemType(i.ItemType),
		}
		if i.ConfigID != nil {
			idStr := i.ConfigID.String()
			item.ConfigId = &idStr
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

// ============================================================================
// Budget Period Methods (stub implementations - use BudgetPeriodHandler for full impl)
// ============================================================================

// GetBudgetPeriod gets or creates a budget period for a specific month
func (h *PlanHandler) GetBudgetPeriod(ctx context.Context, req *connect.Request[echov1.GetBudgetPeriodRequest]) (*connect.Response[echov1.GetBudgetPeriodResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("use BudgetPeriodHandler"))
}

// ListBudgetPeriods lists all periods for a plan
func (h *PlanHandler) ListBudgetPeriods(ctx context.Context, req *connect.Request[echov1.ListBudgetPeriodsRequest]) (*connect.Response[echov1.ListBudgetPeriodsResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("use BudgetPeriodHandler"))
}

// UpdateBudgetPeriodItem updates a specific item's values
func (h *PlanHandler) UpdateBudgetPeriodItem(ctx context.Context, req *connect.Request[echov1.UpdateBudgetPeriodItemRequest]) (*connect.Response[echov1.UpdateBudgetPeriodItemResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("use BudgetPeriodHandler"))
}

// CopyBudgetPeriod copies values from one period to another
func (h *PlanHandler) CopyBudgetPeriod(ctx context.Context, req *connect.Request[echov1.CopyBudgetPeriodRequest]) (*connect.Response[echov1.CopyBudgetPeriodResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("use BudgetPeriodHandler"))
}

// ============================================================================
// Item Config Methods
// ============================================================================

// ListItemConfigs returns all item configs for the current user
func (h *PlanHandler) ListItemConfigs(ctx context.Context, req *connect.Request[echov1.ListItemConfigsRequest]) (*connect.Response[echov1.ListItemConfigsResponse], error) {
	userIDStr, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user not authenticated"))
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	configs, err := h.svc.ListItemConfigs(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var protoConfigs []*echov1.ItemConfig
	for _, c := range configs {
		protoConfigs = append(protoConfigs, toProtoItemConfig(c))
	}

	return connect.NewResponse(&echov1.ListItemConfigsResponse{
		Configs: protoConfigs,
	}), nil
}

// CreateItemConfig creates a new custom item type
func (h *PlanHandler) CreateItemConfig(ctx context.Context, req *connect.Request[echov1.CreateItemConfigRequest]) (*connect.Response[echov1.CreateItemConfigResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

// UpdateItemConfig updates an existing item config
func (h *PlanHandler) UpdateItemConfig(ctx context.Context, req *connect.Request[echov1.UpdateItemConfigRequest]) (*connect.Response[echov1.UpdateItemConfigResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

// DeleteItemConfig deletes a custom item config
func (h *PlanHandler) DeleteItemConfig(ctx context.Context, req *connect.Request[echov1.DeleteItemConfigRequest]) (*connect.Response[echov1.DeleteItemConfigResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

// toProtoItemConfig converts repository ItemConfig to proto
func toProtoItemConfig(c *repository.ItemConfig) *echov1.ItemConfig {
	if c == nil {
		return nil
	}
	return &echov1.ItemConfig{
		Id:        c.ID.String(),
		Label:     c.Label,
		ShortCode: c.ShortCode,
		Behavior:  toProtoItemBehavior(c.Behavior),
		TargetTab: toProtoTargetTab(c.TargetTab),
		ColorHex:  c.ColorHex,
		Icon:      c.Icon,
		SortOrder: int32(c.SortOrder),
		IsSystem:  c.IsSystem,
	}
}

func toProtoItemBehavior(b repository.ItemBehavior) echov1.ItemBehavior {
	switch b {
	case repository.ItemBehaviorOutflow:
		return echov1.ItemBehavior_ITEM_BEHAVIOR_OUTFLOW
	case repository.ItemBehaviorInflow:
		return echov1.ItemBehavior_ITEM_BEHAVIOR_INFLOW
	case repository.ItemBehaviorAsset:
		return echov1.ItemBehavior_ITEM_BEHAVIOR_ASSET
	case repository.ItemBehaviorLiability:
		return echov1.ItemBehavior_ITEM_BEHAVIOR_LIABILITY
	default:
		return echov1.ItemBehavior_ITEM_BEHAVIOR_UNSPECIFIED
	}
}

func toProtoTargetTab(t repository.TargetTab) echov1.TargetTab {
	switch t {
	case repository.TargetTabBudgets:
		return echov1.TargetTab_TARGET_TAB_BUDGETS
	case repository.TargetTabRecurring:
		return echov1.TargetTab_TARGET_TAB_RECURRING
	case repository.TargetTabGoals:
		return echov1.TargetTab_TARGET_TAB_GOALS
	case repository.TargetTabIncome:
		return echov1.TargetTab_TARGET_TAB_INCOME
	case repository.TargetTabPortfolio:
		return echov1.TargetTab_TARGET_TAB_PORTFOLIO
	case repository.TargetTabLiabilities:
		return echov1.TargetTab_TARGET_TAB_LIABILITIES
	default:
		return echov1.TargetTab_TARGET_TAB_UNSPECIFIED
	}
}

func toRepoTargetTab(t echov1.TargetTab) repository.TargetTab {
	switch t {
	case echov1.TargetTab_TARGET_TAB_BUDGETS:
		return repository.TargetTabBudgets
	case echov1.TargetTab_TARGET_TAB_RECURRING:
		return repository.TargetTabRecurring
	case echov1.TargetTab_TARGET_TAB_GOALS:
		return repository.TargetTabGoals
	case echov1.TargetTab_TARGET_TAB_INCOME:
		return repository.TargetTabIncome
	case echov1.TargetTab_TARGET_TAB_PORTFOLIO:
		return repository.TargetTabPortfolio
	case echov1.TargetTab_TARGET_TAB_LIABILITIES:
		return repository.TargetTabLiabilities
	default:
		return repository.TargetTabBudgets // Default fallback
	}
}

func toRepoItemType(t echov1.ItemType) repository.ItemType {
	switch t {
	case echov1.ItemType_ITEM_TYPE_BUDGET:
		return repository.ItemTypeBudget
	case echov1.ItemType_ITEM_TYPE_RECURRING:
		return repository.ItemTypeRecurring
	case echov1.ItemType_ITEM_TYPE_GOAL:
		return repository.ItemTypeGoal
	case echov1.ItemType_ITEM_TYPE_INCOME:
		return repository.ItemTypeIncome
	default:
		return repository.ItemTypeBudget // Default
	}
}

func toProtoItemType(t repository.ItemType) echov1.ItemType {
	switch t {
	case repository.ItemTypeBudget:
		return echov1.ItemType_ITEM_TYPE_BUDGET
	case repository.ItemTypeRecurring:
		return echov1.ItemType_ITEM_TYPE_RECURRING
	case repository.ItemTypeGoal:
		return echov1.ItemType_ITEM_TYPE_GOAL
	case repository.ItemTypeIncome:
		return echov1.ItemType_ITEM_TYPE_INCOME
	default:
		return echov1.ItemType_ITEM_TYPE_BUDGET // Default
	}
}

// ============================================================================
// GetPlanItemsByTab - Filtered item queries for tabs
// ============================================================================

// GetPlanItemsByTab returns items filtered by target tab
func (h *PlanHandler) GetPlanItemsByTab(ctx context.Context, req *connect.Request[echov1.GetPlanItemsByTabRequest]) (*connect.Response[echov1.GetPlanItemsByTabResponse], error) {
	_, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user not authenticated"))
	}

	planID, err := uuid.Parse(req.Msg.PlanId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid plan ID"))
	}

	targetTab := toRepoTargetTab(req.Msg.TargetTab)

	result, err := h.svc.GetItemsByTab(ctx, planID, targetTab)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var protoItems []*echov1.PlanItemWithConfig
	for _, item := range result.Items {
		protoItems = append(protoItems, toProtoPlanItemWithConfig(&item))
	}

	return connect.NewResponse(&echov1.GetPlanItemsByTabResponse{
		Items:         protoItems,
		TotalBudgeted: &echov1.Money{AmountMinor: result.TotalBudgeted, CurrencyCode: "EUR"},
		TotalActual:   &echov1.Money{AmountMinor: result.TotalActual, CurrencyCode: "EUR"},
	}), nil
}

func toProtoPlanItemWithConfig(item *repository.PlanItemWithConfig) *echov1.PlanItemWithConfig {
	result := &echov1.PlanItemWithConfig{
		Id:       item.ID.String(),
		Name:     item.Name,
		Budgeted: &echov1.Money{AmountMinor: item.BudgetedMinor, CurrencyCode: "EUR"},
		Actual:   &echov1.Money{AmountMinor: item.ActualMinor, CurrencyCode: "EUR"},
	}

	if item.CategoryName != nil {
		result.CategoryName = *item.CategoryName
	}
	if item.GroupName != nil {
		result.GroupName = *item.GroupName
	}
	if item.ConfigID != nil {
		result.ConfigId = item.ConfigID.String()
	}
	if item.ConfigLabel != nil {
		result.ConfigLabel = *item.ConfigLabel
	}
	if item.ConfigShortCode != nil {
		result.ConfigShortCode = *item.ConfigShortCode
	}
	if item.ConfigColorHex != nil {
		result.ConfigColorHex = *item.ConfigColorHex
	}
	if item.Behavior != nil {
		result.Behavior = toProtoItemBehavior(*item.Behavior)
	}

	return result
}
