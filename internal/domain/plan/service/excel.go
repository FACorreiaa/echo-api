// Package service provides business logic for user plans.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/plan/excel"
	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/plan/repository"
	"github.com/google/uuid"
)

// AnalyzeExcel analyzes an Excel file and returns sheet information
func (s *PlanService) AnalyzeExcel(r io.Reader) (*ExcelAnalysisResult, error) {
	parser, err := excel.NewParserFromReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Excel: %w", err)
	}
	defer parser.Close()

	analyses, suggested, err := parser.AnalyzeAllSheets()
	if err != nil {
		return nil, fmt.Errorf("failed to analyze sheets: %w", err)
	}

	result := &ExcelAnalysisResult{
		SuggestedSheet: suggested,
		Sheets:         make([]SheetInfo, len(analyses)),
	}

	for i, a := range analyses {
		sheetInfo := SheetInfo{
			Name:               a.Name,
			IsLivingPlan:       a.Type == excel.SheetTypeLivingPlan,
			RowCount:           a.RowCount,
			FormulaCount:       a.FormulaCount,
			DetectedCategories: a.DetectedCategories,
			MonthColumns:       a.MonthColumns,
		}

		// Include detected column mapping if available
		if a.DetectedMapping != nil {
			sheetInfo.DetectedMapping = &ColumnMappingInfo{
				CategoryColumn:   a.DetectedMapping.CategoryColumn,
				ValueColumn:      a.DetectedMapping.ValueColumn,
				HeaderRow:        a.DetectedMapping.HeaderRow,
				PercentageColumn: a.DetectedMapping.PercentageColumn,
				Confidence:       a.DetectedMapping.Confidence,
			}
		}

		result.Sheets[i] = sheetInfo
	}

	return result, nil
}

// ImportFromExcel imports a plan from an Excel file
func (s *PlanService) ImportFromExcel(ctx context.Context, userID uuid.UUID, r io.Reader, sheetName string, config *ExcelImportConfig, planName string) (*ExcelImportResult, error) {
	parser, err := excel.NewParserFromReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Excel: %w", err)
	}
	defer parser.Close()

	// Set defaults
	if config.CategoryColumn == "" {
		config.CategoryColumn = "A"
	}
	if config.ValueColumn == "" {
		config.ValueColumn = "B"
	}
	if config.HeaderRow == 0 {
		config.HeaderRow = 1
	}

	// Extract categories from Excel
	categories, err := parser.ExtractCategories(sheetName, config.CategoryColumn, config.ValueColumn, config.HeaderRow)
	if err != nil {
		return nil, fmt.Errorf("failed to extract categories: %w", err)
	}

	// Build plan structure
	planConfig, _ := json.Marshal(map[string]any{
		"chart_type":       "horizontal_bar",
		"show_percentages": true,
		"source":           "excel",
		"sheet_name":       sheetName,
	})

	plan := &repository.UserPlan{
		UserID:         userID,
		Name:           planName,
		Status:         repository.PlanStatusDraft,
		SourceType:     repository.PlanSourceExcel,
		ExcelSheetName: &sheetName,
		CurrencyCode:   "EUR",
		Config:         planConfig,
	}

	var groups []*repository.PlanCategoryGroup
	var planCategories []*repository.PlanCategory
	var items []*repository.PlanItem

	// Create a single default group for Excel imports
	defaultGroup := &repository.PlanCategoryGroup{
		ID:     uuid.New(),
		Name:   "Imported Categories",
		Labels: []byte(`{"en": "Imported Categories", "pt": "Categorias Importadas"}`),
	}
	groups = append(groups, defaultGroup)

	categoriesImported := 0
	itemsImported := 0

	for catIdx, cat := range categories {
		catID := uuid.New()
		planCat := &repository.PlanCategory{
			ID:        catID,
			GroupID:   &defaultGroup.ID,
			Name:      cat.Name,
			SortOrder: catIdx,
			Labels:    marshalLabels(map[string]string{"pt": cat.Name}),
		}
		planCategories = append(planCategories, planCat)
		categoriesImported++

		for itemIdx, item := range cat.Items {
			// Convert value to minor units (cents)
			budgetedMinor := int64(item.Value * 100)

			var excelCell *string
			var formula *string
			if item.ValueCell != "" {
				excelCell = &item.ValueCell
			}
			if item.Formula != "" {
				formula = &item.Formula
			}

			planItem := &repository.PlanItem{
				ID:            uuid.New(),
				CategoryID:    &catID,
				Name:          item.Name,
				BudgetedMinor: budgetedMinor,
				ExcelCell:     excelCell,
				Formula:       formula,
				WidgetType:    repository.WidgetTypeInput,
				FieldType:     repository.FieldTypeCurrency,
				SortOrder:     itemIdx,
				Labels:        marshalLabels(map[string]string{"pt": item.Name}),
			}
			items = append(items, planItem)
			itemsImported++
		}
	}

	// Save to database
	if err := s.repo.CreatePlanWithStructure(ctx, plan, groups, planCategories, items); err != nil {
		return nil, fmt.Errorf("failed to save plan: %w", err)
	}

	// Reload plan to get computed totals
	savedPlan, err := s.repo.GetPlanByID(ctx, plan.ID)
	if err != nil {
		return nil, err
	}

	return &ExcelImportResult{
		Plan:               savedPlan,
		CategoriesImported: categoriesImported,
		ItemsImported:      itemsImported,
	}, nil
}
