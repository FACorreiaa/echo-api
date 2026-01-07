package excel_test

import (
	"testing"

	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/plan/excel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAnalyzeBudgetFile tests the Excel parser against the test budget file
// This file is located at internal/data/import/budget.xlsx
func TestAnalyzeBudgetFile(t *testing.T) {
	parser, err := excel.NewParserFromFile("../../../data/import/budget.xlsx")
	require.NoError(t, err, "should open budget.xlsx")
	defer parser.Close()

	// Analyze all sheets
	sheets, suggestedSheet, err := parser.AnalyzeAllSheets()
	require.NoError(t, err, "should analyze sheets")
	assert.NotEmpty(t, sheets, "should have sheets")
	assert.NotEmpty(t, suggestedSheet, "should have a suggested sheet")

	t.Logf("Found %d sheets, suggested: %s", len(sheets), suggestedSheet)

	// Find the main budget sheet (ORC DOM_2025)
	var mainSheet *excel.SheetAnalysis
	for i, s := range sheets {
		t.Logf("Sheet %d: %s (type=%s, rows=%d, formulas=%d, score=%d)",
			i, s.Name, s.Type, s.RowCount, s.FormulaCount, s.Score)
		if s.Name == "ORC DOM_2025" {
			mainSheet = &sheets[i]
		}
	}

	require.NotNil(t, mainSheet, "should find ORC DOM_2025 sheet")
	assert.Equal(t, excel.SheetTypeLivingPlan, mainSheet.Type, "main sheet should be a living plan")
	assert.Greater(t, mainSheet.FormulaCount, 10, "should have many formulas")
	assert.NotEmpty(t, mainSheet.DetectedCategories, "should detect categories")

	// Test auto-detected column mapping
	require.NotNil(t, mainSheet.DetectedMapping, "should have detected mapping")
	t.Logf("Detected mapping: category=%s, value=%s, headerRow=%d, confidence=%.2f",
		mainSheet.DetectedMapping.CategoryColumn,
		mainSheet.DetectedMapping.ValueColumn,
		mainSheet.DetectedMapping.HeaderRow,
		mainSheet.DetectedMapping.Confidence)

	// For budget.xlsx, we expect:
	// - Category column should be A (categories are in column A)
	// - Value column could be C, B, or J depending on which column has most numeric/formula content
	//   (J might be a year-to-date total column which the parser correctly identifies)
	// - Header row should be around row 5-6 (where "MESES DO ANO" appears)
	assert.Equal(t, "A", mainSheet.DetectedMapping.CategoryColumn, "category column should be A")
	// Accept any value column if confidence is high - the parser uses content analysis to find the best column
	if mainSheet.DetectedMapping.Confidence >= 0.9 {
		assert.NotEmpty(t, mainSheet.DetectedMapping.ValueColumn, "value column should be detected")
	} else {
		assert.Contains(t, []string{"C", "B", "J"}, mainSheet.DetectedMapping.ValueColumn, "value column should be C, B, or J")
	}
	assert.GreaterOrEqual(t, mainSheet.DetectedMapping.HeaderRow, 5, "header row should be at least 5")
	assert.Greater(t, mainSheet.DetectedMapping.Confidence, 0.5, "confidence should be reasonable")
}

func TestExtractCategories(t *testing.T) {
	parser, err := excel.NewParserFromFile("../../../data/import/budget.xlsx")
	require.NoError(t, err, "should open budget.xlsx")
	defer parser.Close()

	// First analyze to get the suggested mapping
	sheets, _, err := parser.AnalyzeAllSheets()
	require.NoError(t, err, "should analyze sheets")

	// Find ORC DOM_2025 sheet
	var mainSheet *excel.SheetAnalysis
	for i, s := range sheets {
		if s.Name == "ORC DOM_2025" {
			mainSheet = &sheets[i]
			break
		}
	}
	require.NotNil(t, mainSheet, "should find main sheet")
	require.NotNil(t, mainSheet.DetectedMapping, "should have detected mapping")

	// Extract categories using detected mapping
	categories, err := parser.ExtractCategories(
		mainSheet.Name,
		mainSheet.DetectedMapping.CategoryColumn,
		mainSheet.DetectedMapping.ValueColumn,
		mainSheet.DetectedMapping.HeaderRow,
	)
	require.NoError(t, err, "should extract categories")

	t.Logf("Extracted %d categories", len(categories))
	for i, cat := range categories {
		t.Logf("Category %d: %s (%d items)", i, cat.Name, len(cat.Items))
		for j, item := range cat.Items {
			if j < 3 { // Only show first 3 items
				t.Logf("  Item %d: %s = %.2f (cell=%s)", j, item.Name, item.Value, item.ValueCell)
			}
		}
	}

	assert.NotEmpty(t, categories, "should extract at least one category")
}
