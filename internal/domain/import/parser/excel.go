package parser

import (
	"fmt"
	"io"
	"strings"

	"github.com/xuri/excelize/v2"
)

// ExcelParser parses XLSX files for transaction data
type ExcelParser struct {
	config ParserConfig
}

// NewExcelParser creates a new Excel parser
func NewExcelParser(config ParserConfig) *ExcelParser {
	return &ExcelParser{config: config}
}

// ParseExcel reads and parses transactions from an Excel file
func (p *ExcelParser) ParseExcel(reader io.Reader) (*ParseResult, error) {
	result := &ParseResult{
		Transactions: make([]ParsedTransaction, 0, 1000),
		Errors:       make([]ParseError, 0),
	}

	// Open the Excel file
	f, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file: %w", err)
	}
	defer f.Close()

	// Get the first sheet (or named "Transactions" if exists)
	sheetName := p.findTransactionSheet(f)
	if sheetName == "" {
		return nil, fmt.Errorf("no suitable sheet found")
	}

	// Get all rows
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to read sheet %s: %w", sheetName, err)
	}

	if len(rows) == 0 {
		return result, nil
	}

	// Skip configured lines
	startRow := p.config.SkipLines
	if startRow >= len(rows) {
		return result, nil
	}

	// First row after skip is header
	headers := rows[startRow]
	colMap := p.mapColumns(headers)

	// Process data rows
	for i := startRow + 1; i < len(rows); i++ {
		row := rows[i]
		rowNum := i + 1 // 1-indexed

		result.TotalRows++

		tx, parseErr := p.processExcelRow(row, rowNum, colMap)
		if parseErr != nil {
			result.Errors = append(result.Errors, *parseErr)
			continue
		}

		if tx == nil {
			result.SkippedRows++
			continue
		}

		result.Transactions = append(result.Transactions, *tx)
		result.ParsedRows++
	}

	return result, nil
}

// ParseExcelStream parses Excel file with streaming for large files (uses iterator)
func (p *ExcelParser) ParseExcelStream(reader io.Reader) (*ParseResult, error) {
	result := &ParseResult{
		Transactions: make([]ParsedTransaction, 0, 1000),
		Errors:       make([]ParseError, 0),
	}

	// Open with streaming options
	f, err := excelize.OpenReader(reader, excelize.Options{
		RawCellValue: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file: %w", err)
	}
	defer f.Close()

	sheetName := p.findTransactionSheet(f)
	if sheetName == "" {
		return nil, fmt.Errorf("no suitable sheet found")
	}

	// Use row iterator for memory efficiency
	rows, err := f.Rows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to create row iterator: %w", err)
	}
	defer rows.Close()

	var headers []string
	var colMap columnMap
	rowNum := 0
	skippedHeader := false

	for rows.Next() {
		rowNum++

		// Skip configured lines
		if rowNum <= p.config.SkipLines {
			continue
		}

		row, err := rows.Columns()
		if err != nil {
			result.Errors = append(result.Errors, ParseError{
				Row:     rowNum,
				Message: err.Error(),
			})
			continue
		}

		// First non-skipped row is header
		if !skippedHeader {
			headers = row
			colMap = p.mapColumns(headers)
			skippedHeader = true
			continue
		}

		result.TotalRows++

		tx, parseErr := p.processExcelRow(row, rowNum, colMap)
		if parseErr != nil {
			result.Errors = append(result.Errors, *parseErr)
			continue
		}

		if tx == nil {
			result.SkippedRows++
			continue
		}

		result.Transactions = append(result.Transactions, *tx)
		result.ParsedRows++
	}

	return result, nil
}

type columnMap struct {
	dateCol     int
	descCol     int
	amountCol   int
	debitCol    int
	creditCol   int
	categoryCol int
}

// findTransactionSheet finds the best sheet for transaction data
func (p *ExcelParser) findTransactionSheet(f *excelize.File) string {
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return ""
	}

	// Look for sheets with transaction-related names
	preferredNames := []string{
		"transactions", "movimentos", "extrato",
		"statement", "data", "sheet1",
	}

	for _, preferred := range preferredNames {
		for _, sheet := range sheets {
			if strings.EqualFold(sheet, preferred) {
				return sheet
			}
		}
	}

	// Return first sheet as fallback
	return sheets[0]
}

// mapColumns creates a column index map from headers
func (p *ExcelParser) mapColumns(headers []string) columnMap {
	cm := columnMap{
		dateCol:     -1,
		descCol:     -1,
		amountCol:   -1,
		debitCol:    -1,
		creditCol:   -1,
		categoryCol: -1,
	}

	// Use configured columns if set
	if p.config.DateColumn >= 0 {
		cm.dateCol = p.config.DateColumn
	}
	if p.config.DescColumn >= 0 {
		cm.descCol = p.config.DescColumn
	}
	if p.config.AmountColumn >= 0 {
		cm.amountCol = p.config.AmountColumn
	}
	if p.config.DebitColumn >= 0 {
		cm.debitCol = p.config.DebitColumn
	}
	if p.config.CreditColumn >= 0 {
		cm.creditCol = p.config.CreditColumn
	}
	if p.config.CategoryColumn >= 0 {
		cm.categoryCol = p.config.CategoryColumn
	}

	// Auto-detect from headers
	dateKeywords := []string{"date", "data", "fecha", "datum"}
	descKeywords := []string{"description", "descrição", "descricao", "descripción", "merchant", "payee", "memo"}
	amountKeywords := []string{"amount", "valor", "importe", "value", "montant"}
	debitKeywords := []string{"debit", "débito", "debito", "cargo"}
	creditKeywords := []string{"credit", "crédito", "credito", "abono"}
	categoryKeywords := []string{"category", "categoria", "type", "tipo"}

	for i, header := range headers {
		h := strings.ToLower(strings.TrimSpace(header))

		if cm.dateCol < 0 && containsAny(h, dateKeywords) {
			cm.dateCol = i
		}
		if cm.descCol < 0 && containsAny(h, descKeywords) {
			cm.descCol = i
		}
		if cm.amountCol < 0 && containsAny(h, amountKeywords) {
			cm.amountCol = i
		}
		if cm.debitCol < 0 && containsAny(h, debitKeywords) {
			cm.debitCol = i
		}
		if cm.creditCol < 0 && containsAny(h, creditKeywords) {
			cm.creditCol = i
		}
		if cm.categoryCol < 0 && containsAny(h, categoryKeywords) {
			cm.categoryCol = i
		}
	}

	return cm
}

func containsAny(s string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

// processExcelRow converts an Excel row to a ParsedTransaction
func (p *ExcelParser) processExcelRow(row []string, rowNum int, colMap columnMap) (*ParsedTransaction, *ParseError) {
	getValue := func(idx int) string {
		if idx < 0 || idx >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[idx])
	}

	// Get date
	dateStr := getValue(colMap.dateCol)
	if dateStr == "" {
		return nil, nil // Skip empty rows
	}

	// Create temporary parser for shared parsing logic
	csvParser := NewParser(p.config)

	date, err := csvParser.parseDate(dateStr)
	if err != nil {
		return nil, &ParseError{
			Row:     rowNum,
			Column:  "date",
			Message: fmt.Sprintf("invalid date: %s", err.Error()),
			RawData: dateStr,
		}
	}

	// Get description
	desc := getValue(colMap.descCol)
	if desc == "" {
		return nil, &ParseError{
			Row:     rowNum,
			Column:  "description",
			Message: "missing description",
		}
	}

	// Get amount
	var amountCents int64
	var currency string

	if colMap.amountCol >= 0 {
		amountStr := getValue(colMap.amountCol)
		if amountStr == "" {
			return nil, &ParseError{
				Row:     rowNum,
				Column:  "amount",
				Message: "missing amount",
			}
		}
		amountCents, currency, err = csvParser.parseAmount(amountStr)
		if err != nil {
			return nil, &ParseError{
				Row:     rowNum,
				Column:  "amount",
				Message: fmt.Sprintf("invalid amount: %s", err.Error()),
				RawData: amountStr,
			}
		}
	} else if colMap.debitCol >= 0 || colMap.creditCol >= 0 {
		debitStr := getValue(colMap.debitCol)
		creditStr := getValue(colMap.creditCol)
		amountCents, currency = csvParser.parseDebitCredit(debitStr, creditStr)
	} else {
		return nil, &ParseError{
			Row:     rowNum,
			Column:  "amount",
			Message: "no amount column found",
		}
	}

	// Get category
	category := getValue(colMap.categoryCol)

	return &ParsedTransaction{
		Date:         date,
		Description:  cleanDescription(desc),
		AmountCents:  amountCents,
		Category:     category,
		RawRow:       rowNum,
		CurrencyHint: currency,
	}, nil
}

// DetectExcelFormat analyzes an Excel file and returns detected configuration
func DetectExcelFormat(reader io.Reader) (*ExcelFormatInfo, error) {
	f, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file: %w", err)
	}
	defer f.Close()

	info := &ExcelFormatInfo{
		Sheets: f.GetSheetList(),
	}

	if len(info.Sheets) == 0 {
		return info, nil
	}

	// Analyze first sheet
	sheetName := info.Sheets[0]
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return info, nil
	}

	if len(rows) > 0 {
		info.Headers = rows[0]
		info.RowCount = len(rows) - 1 // Exclude header

		// Get sample rows
		maxSamples := 5
		if len(rows) < maxSamples+1 {
			maxSamples = len(rows) - 1
		}
		info.SampleRows = make([][]string, maxSamples)
		for i := 0; i < maxSamples; i++ {
			info.SampleRows[i] = rows[i+1]
		}
	}

	return info, nil
}

// ExcelFormatInfo contains detected Excel file format information
type ExcelFormatInfo struct {
	Sheets     []string
	Headers    []string
	RowCount   int
	SampleRows [][]string
}
