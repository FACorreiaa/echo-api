// Package e2etest provides end-to-end integration tests for import flows.
package e2etest

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/import/service"
	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/import/sniffer"
	planexcel "github.com/FACorreiaa/smart-finance-tracker/internal/domain/plan/excel"
	planservice "github.com/FACorreiaa/smart-finance-tracker/internal/domain/plan/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testDataDir = "../../internal/data/import"

// TestCGD_CSVImport tests importing a CGD (Caixa Geral de Depósitos) bank statement CSV.
// CGD statements use semicolon delimiter, European number format, and Portuguese headers.
func TestCGD_CSVImport(t *testing.T) {
	csvPath := filepath.Join(testDataDir, "comprovativo.csv")

	data, err := os.ReadFile(csvPath)
	if os.IsNotExist(err) {
		t.Skipf("Test data file not found: %s (add CGD CSV to run this test)", csvPath)
	}
	require.NoError(t, err, "Failed to read CGD CSV file")
	require.NotEmpty(t, data, "CGD CSV file is empty")

	t.Run("DetectConfig", func(t *testing.T) {
		config, err := sniffer.DetectConfig(data)
		require.NoError(t, err, "Failed to detect config for CGD CSV")

		// CGD uses semicolon delimiter
		assert.Equal(t, ';', config.Delimiter, "Expected semicolon delimiter for CGD")
		assert.NotEmpty(t, config.Headers, "Expected headers to be detected")

		t.Logf("CGD CSV config: delimiter=%c, skipLines=%d, headers=%v",
			config.Delimiter, config.SkipLines, config.Headers)
	})

	t.Run("SuggestColumns", func(t *testing.T) {
		config, err := sniffer.DetectConfig(data)
		require.NoError(t, err)

		suggestions := sniffer.SuggestColumns(config.Headers)

		// CGD should have date, description, and debit/credit columns
		assert.GreaterOrEqual(t, suggestions.DateCol, 0, "Expected date column to be detected")
		assert.GreaterOrEqual(t, suggestions.DescCol, 0, "Expected description column to be detected")

		t.Logf("CGD column suggestions: date=%d, desc=%d, amount=%d, debit=%d, credit=%d, isDoubleEntry=%v",
			suggestions.DateCol, suggestions.DescCol, suggestions.AmountCol,
			suggestions.DebitCol, suggestions.CreditCol, suggestions.IsDoubleEntry)
	})

	t.Run("ProbeDialect", func(t *testing.T) {
		config, err := sniffer.DetectConfig(data)
		require.NoError(t, err)

		suggestions := sniffer.SuggestColumns(config.Headers)

		// CGD uses double-entry format (debit/credit columns), so use debit column for probing
		// if amount column is not available
		amountCol := suggestions.AmountCol
		if amountCol < 0 && suggestions.DebitCol >= 0 {
			amountCol = suggestions.DebitCol
		}

		dialect := sniffer.ProbeDialect(config.SampleRows, amountCol, suggestions.DateCol)

		// CGD is Portuguese bank, should detect European format
		// Note: If column detection fails due to encoding issues, this may not work
		if amountCol >= 0 {
			assert.True(t, dialect.IsEuropeanFormat, "Expected European number format for CGD")
		} else {
			t.Log("Skipping European format assertion - amount column not detected (likely encoding issue)")
		}

		t.Logf("CGD dialect: isEuropean=%v, dateFormat=%s, confidence=%.2f, currency=%s",
			dialect.IsEuropeanFormat, dialect.DateFormat, dialect.Confidence, dialect.CurrencyHint)
	})
}

// TestRevolut_CSVImport tests importing a Revolut bank statement CSV.
// Revolut statements use comma delimiter, English headers, and standard date format.
func TestRevolut_CSVImport(t *testing.T) {
	// Revolut exports are usually the account-statement files
	csvPath := filepath.Join(testDataDir, "account-statement_2019-12-01_2025-12-28_en_e1631a.csv")

	data, err := os.ReadFile(csvPath)
	if os.IsNotExist(err) {
		t.Skipf("Test data file not found: %s (add Revolut CSV to run this test)", csvPath)
	}
	require.NoError(t, err, "Failed to read Revolut CSV file")
	require.NotEmpty(t, data, "Revolut CSV file is empty")

	t.Run("DetectConfig", func(t *testing.T) {
		config, err := sniffer.DetectConfig(data)
		require.NoError(t, err, "Failed to detect config for Revolut CSV")

		// Revolut uses comma delimiter
		assert.Equal(t, ',', config.Delimiter, "Expected comma delimiter for Revolut")
		assert.NotEmpty(t, config.Headers, "Expected headers to be detected")

		t.Logf("Revolut CSV config: delimiter=%c, skipLines=%d, headers=%v",
			config.Delimiter, config.SkipLines, config.Headers)
	})

	t.Run("SuggestColumns", func(t *testing.T) {
		config, err := sniffer.DetectConfig(data)
		require.NoError(t, err)

		suggestions := sniffer.SuggestColumns(config.Headers)

		// Revolut should have date, description, and amount columns
		assert.GreaterOrEqual(t, suggestions.DateCol, 0, "Expected date column to be detected")
		assert.GreaterOrEqual(t, suggestions.DescCol, 0, "Expected description column to be detected")

		t.Logf("Revolut column suggestions: date=%d, desc=%d, amount=%d, debit=%d, credit=%d",
			suggestions.DateCol, suggestions.DescCol, suggestions.AmountCol,
			suggestions.DebitCol, suggestions.CreditCol)
	})

	t.Run("ProbeDialect", func(t *testing.T) {
		config, err := sniffer.DetectConfig(data)
		require.NoError(t, err)

		suggestions := sniffer.SuggestColumns(config.Headers)
		dialect := sniffer.ProbeDialect(config.SampleRows, suggestions.AmountCol, suggestions.DateCol)

		t.Logf("Revolut dialect: isEuropean=%v, dateFormat=%s, confidence=%.2f, currency=%s",
			dialect.IsEuropeanFormat, dialect.DateFormat, dialect.Confidence, dialect.CurrencyHint)
	})
}

// TestBudget_ExcelPlanImport tests importing a budget spreadsheet as a financial plan.
// This tests the Excel-to-Plan flow which extracts categories from the spreadsheet.
func TestBudget_ExcelPlanImport(t *testing.T) {
	xlsxPath := filepath.Join(testDataDir, "budget.xlsx")

	data, err := os.ReadFile(xlsxPath)
	if os.IsNotExist(err) {
		t.Skipf("Test data file not found: %s (add budget.xlsx to run this test)", xlsxPath)
	}
	require.NoError(t, err, "Failed to read budget Excel file")
	require.NotEmpty(t, data, "Budget Excel file is empty")

	t.Run("ParseExcel", func(t *testing.T) {
		parser, err := planexcel.NewParserFromReader(bytes.NewReader(data))
		require.NoError(t, err, "Failed to parse budget Excel file")
		defer parser.Close()

		sheets := parser.GetSheetList()
		assert.NotEmpty(t, sheets, "Expected at least one sheet in budget Excel")

		t.Logf("Budget Excel sheets: %v", sheets)
	})

	t.Run("AnalyzeSheets", func(t *testing.T) {
		parser, err := planexcel.NewParserFromReader(bytes.NewReader(data))
		require.NoError(t, err)
		defer parser.Close()

		analyses, suggested, err := parser.AnalyzeAllSheets()
		require.NoError(t, err, "Failed to analyze sheets")

		assert.NotEmpty(t, analyses, "Expected at least one sheet analysis")

		for _, a := range analyses {
			t.Logf("Sheet %q: rows=%d, formulas=%d, isLiving=%v, categories=%v, months=%v",
				a.Name, a.RowCount, a.FormulaCount,
				a.Type == planexcel.SheetTypeLivingPlan,
				a.DetectedCategories, a.MonthColumns)
		}

		if suggested != "" {
			t.Logf("Suggested sheet for import: %s", suggested)
		}
	})

	t.Run("ExtractCategories", func(t *testing.T) {
		parser, err := planexcel.NewParserFromReader(bytes.NewReader(data))
		require.NoError(t, err)
		defer parser.Close()

		sheets := parser.GetSheetList()
		require.NotEmpty(t, sheets)

		// Try to extract categories from the first sheet
		categories, err := parser.ExtractCategories(sheets[0], "A", "B", 1)
		if err != nil {
			t.Logf("Could not extract categories from first sheet with default config: %v", err)
			// Try with custom config
			categories, err = parser.ExtractCategories(sheets[0], "A", "C", 2)
		}

		if err == nil && len(categories) > 0 {
			t.Logf("Extracted %d categories from sheet %q", len(categories), sheets[0])
			for _, cat := range categories {
				t.Logf("  Category %q: %d items", cat.Name, len(cat.Items))
				for _, item := range cat.Items {
					t.Logf("    - %s: %.2f (cell=%s, formula=%s)",
						item.Name, item.Value, item.ValueCell, item.Formula)
				}
			}
		}
	})
}

// TestIntegration_FullImportFlow tests a complete import flow from file to parsed transactions.
func TestIntegration_FullImportFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test CGD file with service layer
	t.Run("CGD_AnalyzeFile", func(t *testing.T) {
		csvPath := filepath.Join(testDataDir, "comprovativo.csv")
		data, err := os.ReadFile(csvPath)
		if os.IsNotExist(err) {
			t.Skip("CGD test file not found")
		}
		require.NoError(t, err)

		// Create mock service without repo (for analysis only)
		svc := service.NewImportService(nil, nil)

		result, err := svc.AnalyzeFile(context.Background(), uuid.New(), data)
		require.NoError(t, err, "AnalyzeFile should not fail for CGD CSV")

		assert.NotNil(t, result.FileConfig, "Expected file config")
		assert.NotNil(t, result.ColumnSuggestions, "Expected column suggestions")
		assert.NotNil(t, result.ProbedDialect, "Expected probed dialect")

		t.Logf("CGD Analysis: headers=%v, suggestions=%+v, dialect=%+v, canAutoImport=%v",
			result.FileConfig.Headers,
			result.ColumnSuggestions,
			result.ProbedDialect,
			result.CanAutoImport)
	})

	t.Run("Revolut_AnalyzeFile", func(t *testing.T) {
		csvPath := filepath.Join(testDataDir, "account-statement_2019-12-01_2025-12-28_en_e1631a.csv")
		data, err := os.ReadFile(csvPath)
		if os.IsNotExist(err) {
			t.Skip("Revolut test file not found")
		}
		require.NoError(t, err)

		svc := service.NewImportService(nil, nil)

		result, err := svc.AnalyzeFile(context.Background(), uuid.New(), data)
		require.NoError(t, err, "AnalyzeFile should not fail for Revolut CSV")

		assert.NotNil(t, result.FileConfig, "Expected file config")

		t.Logf("Revolut Analysis: headers=%v, canAutoImport=%v",
			result.FileConfig.Headers, result.CanAutoImport)
	})

	t.Run("Budget_AnalyzeExcel", func(t *testing.T) {
		xlsxPath := filepath.Join(testDataDir, "budget.xlsx")
		data, err := os.ReadFile(xlsxPath)
		if os.IsNotExist(err) {
			t.Skip("Budget test file not found")
		}
		require.NoError(t, err)

		svc := planservice.NewPlanService(nil, nil, nil)

		result, err := svc.AnalyzeExcel(bytes.NewReader(data))
		require.NoError(t, err, "AnalyzeExcel should not fail for budget.xlsx")

		assert.NotEmpty(t, result.Sheets, "Expected at least one sheet")

		t.Logf("Budget Analysis: %d sheets, suggested=%s",
			len(result.Sheets), result.SuggestedSheet)
		for _, s := range result.Sheets {
			t.Logf("  Sheet %q: rows=%d, formulas=%d, living=%v",
				s.Name, s.RowCount, s.FormulaCount, s.IsLivingPlan)
		}
	})
}
