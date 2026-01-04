// Package excel provides Excel file analysis and parsing for plan imports.
package excel

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/xuri/excelize/v2"
)

// SheetType indicates whether a sheet contains raw data or formulas/logic
type SheetType string

const (
	SheetTypeDataDump   SheetType = "data_dump"   // Simple rows of data (bank statements)
	SheetTypeLivingPlan SheetType = "living_plan" // Contains formulas and structure (budgets)
)

// SheetAnalysis contains the results of analyzing an Excel sheet
type SheetAnalysis struct {
	Name               string    `json:"name"`
	Type               SheetType `json:"type"`
	RowCount           int       `json:"row_count"`
	ColCount           int       `json:"col_count"`
	FormulaCount       int       `json:"formula_count"`
	DetectedCategories []string  `json:"detected_categories,omitempty"`
	MonthColumns       []string  `json:"month_columns,omitempty"`
	Score              int       `json:"score"` // Higher = more likely to be the main budget sheet
}

// CategoryExtraction represents an extracted category from the sheet
type CategoryExtraction struct {
	Name       string
	Row        int
	Items      []ItemExtraction
	TotalCell  string // e.g., "B29" for category total
	TotalValue float64
}

// ItemExtraction represents an extracted budget line item
type ItemExtraction struct {
	Name      string
	Row       int
	ValueCell string
	Value     float64
	Formula   string
	IsFormula bool
}

// Parser handles Excel file analysis and data extraction
type Parser struct {
	file *excelize.File
}

// NewParserFromReader creates a parser from an io.Reader
func NewParserFromReader(r io.Reader) (*Parser, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file: %w", err)
	}
	return &Parser{file: f}, nil
}

// NewParserFromFile creates a parser from a file path
func NewParserFromFile(path string) (*Parser, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file: %w", err)
	}
	return &Parser{file: f}, nil
}

// Close closes the Excel file
func (p *Parser) Close() error {
	if p.file != nil {
		return p.file.Close()
	}
	return nil
}

// AnalyzeAllSheets analyzes all sheets and returns analysis for each
func (p *Parser) AnalyzeAllSheets() ([]SheetAnalysis, string, error) {
	sheets := p.file.GetSheetList()
	results := make([]SheetAnalysis, 0, len(sheets))

	var bestSheet string
	bestScore := 0

	for _, sheet := range sheets {
		analysis, err := p.AnalyzeSheet(sheet)
		if err != nil {
			continue // Skip sheets that fail to analyze
		}
		results = append(results, *analysis)

		if analysis.Score > bestScore {
			bestScore = analysis.Score
			bestSheet = sheet
		}
	}

	return results, bestSheet, nil
}

// AnalyzeSheet analyzes a single sheet for structure and content
func (p *Parser) AnalyzeSheet(sheetName string) (*SheetAnalysis, error) {
	rows, err := p.file.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to get rows: %w", err)
	}

	analysis := &SheetAnalysis{
		Name:     sheetName,
		RowCount: len(rows),
	}

	// Count columns (max across all rows)
	for _, row := range rows {
		if len(row) > analysis.ColCount {
			analysis.ColCount = len(row)
		}
	}

	// Count formulas and detect categories
	formulaCount := 0
	categories := make([]string, 0)
	monthColumns := make([]string, 0)

	for rowIdx := 1; rowIdx <= analysis.RowCount; rowIdx++ {
		for colIdx := 1; colIdx <= analysis.ColCount; colIdx++ {
			cell, _ := excelize.CoordinatesToCellName(colIdx, rowIdx)
			formula, _ := p.file.GetCellFormula(sheetName, cell)
			if formula != "" {
				formulaCount++
			}

			// Check first row for month headers
			if rowIdx == 1 || rowIdx == 2 {
				value, _ := p.file.GetCellValue(sheetName, cell)
				if isMonthHeader(value) {
					monthColumns = append(monthColumns, value)
				}
			}

			// Check first column for category names
			if colIdx == 1 {
				value, _ := p.file.GetCellValue(sheetName, cell)
				if isCategoryLike(value) {
					categories = append(categories, value)
				}
			}
		}
	}

	analysis.FormulaCount = formulaCount
	analysis.DetectedCategories = categories
	analysis.MonthColumns = monthColumns

	// Determine sheet type and score
	if formulaCount > 10 {
		analysis.Type = SheetTypeLivingPlan
		analysis.Score = 100 + formulaCount + len(categories)*10
	} else {
		analysis.Type = SheetTypeDataDump
		analysis.Score = 50 + len(categories)*5
	}

	// Boost score if sheet name suggests it's a budget
	nameLower := strings.ToLower(sheetName)
	if strings.Contains(nameLower, "budget") ||
		strings.Contains(nameLower, "orc") ||
		strings.Contains(nameLower, "plan") ||
		strings.Contains(nameLower, "despesas") {
		analysis.Score += 50
	}

	return analysis, nil
}

// ExtractCategories extracts categories and items from a budget sheet
func (p *Parser) ExtractCategories(sheetName string, categoryCol, valueCol string, startRow int) ([]CategoryExtraction, error) {
	rows, err := p.file.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to get rows: %w", err)
	}

	categories := make([]CategoryExtraction, 0)
	var currentCategory *CategoryExtraction

	// Column letters to indices
	catColIdx := colLetterToIdx(categoryCol)
	valColIdx := colLetterToIdx(valueCol)

	for rowIdx := startRow; rowIdx <= len(rows); rowIdx++ {
		row := rows[rowIdx-1] // 0-indexed

		// Get cell values
		var catValue, valValue string
		if catColIdx <= len(row) {
			catValue = strings.TrimSpace(row[catColIdx-1])
		}
		if valColIdx <= len(row) {
			valValue = strings.TrimSpace(row[valColIdx-1])
		}

		if catValue == "" {
			continue
		}

		// Skip rows that have no value (likely section headers or separators)
		hasValue := valValue != "" || valValue == "0"
		_ = hasValue // Used for potential future filtering

		// Get cell references
		catCell, _ := excelize.CoordinatesToCellName(catColIdx, rowIdx)
		valCell, _ := excelize.CoordinatesToCellName(valColIdx, rowIdx)

		// Check if this looks like a category header (bold, all caps, or followed by items)
		style, _ := p.file.GetCellStyle(sheetName, catCell)
		isBold := style > 0 // Simplified: assuming styled cells are headers

		isUpperCase := catValue == strings.ToUpper(catValue) && len(catValue) > 2
		isCatHeader := isBold || isUpperCase || isCategoryKeyword(catValue)

		if isCatHeader && currentCategory != nil {
			// Save current category and start new one
			categories = append(categories, *currentCategory)
			currentCategory = nil
		}

		if isCatHeader {
			currentCategory = &CategoryExtraction{
				Name:  catValue,
				Row:   rowIdx,
				Items: make([]ItemExtraction, 0),
			}
		} else if currentCategory != nil {
			// This is an item under the current category
			formula, _ := p.file.GetCellFormula(sheetName, valCell)
			value, _ := p.file.GetCellValue(sheetName, valCell)

			item := ItemExtraction{
				Name:      catValue,
				Row:       rowIdx,
				ValueCell: valCell,
				Formula:   formula,
				IsFormula: formula != "",
			}

			// Parse value as float
			if v, err := parseNumericValue(value); err == nil {
				item.Value = v
			}

			currentCategory.Items = append(currentCategory.Items, item)
		}
	}

	// Don't forget the last category
	if currentCategory != nil {
		categories = append(categories, *currentCategory)
	}

	return categories, nil
}

// GetSheetList returns all sheet names
func (p *Parser) GetSheetList() []string {
	return p.file.GetSheetList()
}

// ============================================================================
// Helper functions
// ============================================================================

// isMonthHeader checks if a value looks like a month header
func isMonthHeader(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	months := []string{
		"jan", "feb", "mar", "apr", "may", "jun",
		"jul", "aug", "sep", "oct", "nov", "dec",
		"janeiro", "fevereiro", "março", "abril", "maio", "junho",
		"julho", "agosto", "setembro", "outubro", "novembro", "dezembro",
	}
	for _, m := range months {
		if strings.HasPrefix(value, m) {
			return true
		}
	}
	return false
}

// isCategoryLike checks if a value looks like a category name
func isCategoryLike(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) < 3 || len(value) > 50 {
		return false
	}
	// Skip numeric values
	if _, err := parseNumericValue(value); err == nil {
		return false
	}
	return true
}

// isCategoryKeyword checks if the value is a known category keyword
func isCategoryKeyword(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	keywords := []string{
		"housing", "transport", "food", "health", "entertainment",
		"habitação", "transporte", "alimentação", "saúde", "lazer",
		"rendimentos", "income", "expenses", "despesas", "savings",
		"total", "subtotal", "dívidas", "debts",
	}
	for _, kw := range keywords {
		if strings.Contains(value, kw) {
			return true
		}
	}
	return false
}

// colLetterToIdx converts a column letter (A, B, C...) to a 1-based index
func colLetterToIdx(col string) int {
	col = strings.ToUpper(col)
	result := 0
	for i := 0; i < len(col); i++ {
		result = result*26 + int(col[i]-'A'+1)
	}
	return result
}

// parseNumericValue attempts to parse a string as a numeric value
func parseNumericValue(s string) (float64, error) {
	// Remove common formatting
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "€", "")
	s = strings.ReplaceAll(s, "$", "")
	s = strings.ReplaceAll(s, ",", ".")

	// Handle European format (1.234,56 -> 1234.56)
	re := regexp.MustCompile(`(\d+)\.(\d{3})`)
	for re.MatchString(s) {
		s = re.ReplaceAllString(s, "$1$2")
	}

	var v float64
	_, err := fmt.Sscanf(s, "%f", &v)
	return v, err
}
