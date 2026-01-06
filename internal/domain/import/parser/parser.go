// Package parser provides high-performance CSV and Excel parsing for transaction imports.
// It uses gocsv for struct-based unmarshaling and supports streaming for large files.
package parser

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gocarina/gocsv"
)

// TransactionRow represents a raw CSV row that can be unmarshaled directly.
// The tags support flexible column naming (gocsv matches by header name).
type TransactionRow struct {
	// Date columns - various common names
	Date      string `csv:"date"`
	DataMov   string `csv:"data mov."`
	DataMovim string `csv:"data movim."`
	Fecha     string `csv:"fecha"`
	Datum     string `csv:"datum"`

	// Description columns
	Description string `csv:"description"`
	Descricao   string `csv:"descrição"`
	Descricao2  string `csv:"descricao"`
	Descripcion string `csv:"descripción"`
	Merchant    string `csv:"merchant"`
	Payee       string `csv:"payee"`
	Details     string `csv:"details"`
	Memo        string `csv:"memo"`

	// Amount columns (single)
	Amount  string `csv:"amount"`
	Valor   string `csv:"valor"`
	Importe string `csv:"importe"`
	Value   string `csv:"value"`
	Montant string `csv:"montant"`

	// Debit/Credit columns (double-entry)
	Debit   string `csv:"debit"`
	Debito  string `csv:"débito"`
	Debito2 string `csv:"debito"`
	Cargo   string `csv:"cargo"`

	Credit   string `csv:"credit"`
	Credito  string `csv:"crédito"`
	Credito2 string `csv:"credito"`
	Abono    string `csv:"abono"`

	// Category columns
	Category  string `csv:"category"`
	Categoria string `csv:"categoria"`
	Type      string `csv:"type"`
	Tipo      string `csv:"tipo"`

	// Balance (for reference, not imported)
	Balance string `csv:"balance"`
	Saldo   string `csv:"saldo"`
}

// ParsedTransaction is the normalized output after parsing a row
type ParsedTransaction struct {
	Date         time.Time
	Description  string
	AmountCents  int64  // Positive = income, Negative = expense
	Category     string // Raw category from file
	RawRow       int    // Original row number for error reporting
	CurrencyHint string // Detected currency if present
}

// ParseError represents a parsing error for a specific row
type ParseError struct {
	Row     int
	Column  string
	Message string
	RawData string
}

func (e ParseError) Error() string {
	return fmt.Sprintf("row %d, column %s: %s", e.Row, e.Column, e.Message)
}

// ParseResult contains the results of parsing a CSV file
type ParseResult struct {
	Transactions []ParsedTransaction
	Errors       []ParseError
	TotalRows    int
	ParsedRows   int
	SkippedRows  int
}

// ParserConfig configures the CSV parser behavior
type ParserConfig struct {
	Delimiter        rune   // CSV delimiter (default: auto-detect)
	SkipLines        int    // Lines to skip before headers
	DateFormat       string // Expected date format (default: flexible)
	IsEuropeanFormat bool   // Amount format: true = 1.234,56, false = 1,234.56
	DateColumn       int    // Override column index for date (-1 = auto)
	DescColumn       int    // Override column index for description (-1 = auto)
	AmountColumn     int    // Override column index for amount (-1 = auto)
	DebitColumn      int    // Override column index for debit (-1 = auto)
	CreditColumn     int    // Override column index for credit (-1 = auto)
	CategoryColumn   int    // Override column index for category (-1 = auto)
}

// DefaultConfig returns a parser config with sensible defaults
func DefaultConfig() ParserConfig {
	return ParserConfig{
		Delimiter:        0, // Auto-detect
		SkipLines:        0,
		DateFormat:       "", // Flexible
		IsEuropeanFormat: false,
		DateColumn:       -1,
		DescColumn:       -1,
		AmountColumn:     -1,
		DebitColumn:      -1,
		CreditColumn:     -1,
		CategoryColumn:   -1,
	}
}

// Parser is a high-performance CSV parser for transaction data
type Parser struct {
	config ParserConfig
}

// NewParser creates a new parser with the given configuration
func NewParser(config ParserConfig) *Parser {
	return &Parser{config: config}
}

// Parse reads and parses all transactions from a CSV reader
func (p *Parser) Parse(reader io.Reader) (*ParseResult, error) {
	result := &ParseResult{
		Transactions: make([]ParsedTransaction, 0, 1000),
		Errors:       make([]ParseError, 0),
	}

	// Skip lines if configured
	if p.config.SkipLines > 0 {
		reader = skipLines(reader, p.config.SkipLines)
	}

	// Configure gocsv
	if p.config.Delimiter != 0 {
		gocsv.SetCSVReader(func(in io.Reader) gocsv.CSVReader {
			r := csv.NewReader(in)
			r.Comma = p.config.Delimiter
			r.LazyQuotes = true
			r.TrimLeadingSpace = true
			return r
		})
	}

	// Parse into generic rows first for flexibility
	var rows []TransactionRow
	if err := gocsv.Unmarshal(reader, &rows); err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	result.TotalRows = len(rows)

	// Process each row
	for i, row := range rows {
		rowNum := i + p.config.SkipLines + 2 // +2 for 1-indexed and header

		tx, err := p.processRow(row, rowNum)
		if err != nil {
			result.Errors = append(result.Errors, *err)
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

// ParseWithColumns parses using explicit column indices (for when headers don't match)
func (p *Parser) ParseWithColumns(reader io.Reader, headers []string) (*ParseResult, error) {
	result := &ParseResult{
		Transactions: make([]ParsedTransaction, 0, 1000),
		Errors:       make([]ParseError, 0),
	}

	// Skip lines if configured
	if p.config.SkipLines > 0 {
		reader = skipLines(reader, p.config.SkipLines)
	}

	// Create CSV reader
	csvReader := csv.NewReader(reader)
	if p.config.Delimiter != 0 {
		csvReader.Comma = p.config.Delimiter
	}
	csvReader.LazyQuotes = true
	csvReader.TrimLeadingSpace = true
	csvReader.FieldsPerRecord = -1 // Variable field count

	// Skip header row
	_, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	rowNum := p.config.SkipLines + 2

	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			result.Errors = append(result.Errors, ParseError{
				Row:     rowNum,
				Message: err.Error(),
			})
			rowNum++
			continue
		}

		result.TotalRows++

		tx, parseErr := p.processRecord(record, rowNum)
		if parseErr != nil {
			result.Errors = append(result.Errors, *parseErr)
			rowNum++
			continue
		}

		if tx == nil {
			result.SkippedRows++
			rowNum++
			continue
		}

		result.Transactions = append(result.Transactions, *tx)
		result.ParsedRows++
		rowNum++
	}

	return result, nil
}

// processRow converts a TransactionRow to a ParsedTransaction
func (p *Parser) processRow(row TransactionRow, rowNum int) (*ParsedTransaction, *ParseError) {
	// Find date
	dateStr := coalesce(row.Date, row.DataMov, row.DataMovim, row.Fecha, row.Datum)
	if dateStr == "" {
		return nil, nil // Skip rows without date
	}

	date, err := p.parseDate(dateStr)
	if err != nil {
		return nil, &ParseError{
			Row:     rowNum,
			Column:  "date",
			Message: fmt.Sprintf("invalid date: %s", err.Error()),
			RawData: dateStr,
		}
	}

	// Find description
	desc := coalesce(row.Description, row.Descricao, row.Descricao2, row.Descripcion,
		row.Merchant, row.Payee, row.Details, row.Memo)
	if desc == "" {
		return nil, &ParseError{
			Row:     rowNum,
			Column:  "description",
			Message: "missing description",
		}
	}

	// Find amount
	var amountCents int64
	var currency string

	// Try single amount column first
	amountStr := coalesce(row.Amount, row.Valor, row.Importe, row.Value, row.Montant)
	if amountStr != "" {
		amountCents, currency, err = p.parseAmount(amountStr)
		if err != nil {
			return nil, &ParseError{
				Row:     rowNum,
				Column:  "amount",
				Message: fmt.Sprintf("invalid amount: %s", err.Error()),
				RawData: amountStr,
			}
		}
	} else {
		// Try debit/credit columns
		debitStr := coalesce(row.Debit, row.Debito, row.Debito2, row.Cargo)
		creditStr := coalesce(row.Credit, row.Credito, row.Credito2, row.Abono)

		if debitStr == "" && creditStr == "" {
			return nil, &ParseError{
				Row:     rowNum,
				Column:  "amount",
				Message: "no amount found",
			}
		}

		amountCents, currency = p.parseDebitCredit(debitStr, creditStr)
	}

	// Find category
	category := coalesce(row.Category, row.Categoria, row.Type, row.Tipo)

	return &ParsedTransaction{
		Date:         date,
		Description:  cleanDescription(desc),
		AmountCents:  amountCents,
		Category:     category,
		RawRow:       rowNum,
		CurrencyHint: currency,
	}, nil
}

// processRecord converts a raw CSV record to a ParsedTransaction using column indices
func (p *Parser) processRecord(record []string, rowNum int) (*ParsedTransaction, *ParseError) {
	getValue := func(idx int) string {
		if idx < 0 || idx >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[idx])
	}

	// Get date
	dateStr := getValue(p.config.DateColumn)
	if dateStr == "" {
		return nil, nil // Skip rows without date
	}

	date, err := p.parseDate(dateStr)
	if err != nil {
		return nil, &ParseError{
			Row:     rowNum,
			Column:  "date",
			Message: fmt.Sprintf("invalid date: %s", err.Error()),
			RawData: dateStr,
		}
	}

	// Get description
	desc := getValue(p.config.DescColumn)
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

	if p.config.AmountColumn >= 0 {
		amountStr := getValue(p.config.AmountColumn)
		if amountStr == "" {
			return nil, &ParseError{
				Row:     rowNum,
				Column:  "amount",
				Message: "missing amount",
			}
		}
		amountCents, currency, err = p.parseAmount(amountStr)
		if err != nil {
			return nil, &ParseError{
				Row:     rowNum,
				Column:  "amount",
				Message: fmt.Sprintf("invalid amount: %s", err.Error()),
				RawData: amountStr,
			}
		}
	} else if p.config.DebitColumn >= 0 || p.config.CreditColumn >= 0 {
		debitStr := getValue(p.config.DebitColumn)
		creditStr := getValue(p.config.CreditColumn)
		amountCents, currency = p.parseDebitCredit(debitStr, creditStr)
	} else {
		return nil, &ParseError{
			Row:     rowNum,
			Column:  "amount",
			Message: "no amount column configured",
		}
	}

	// Get category
	category := getValue(p.config.CategoryColumn)

	return &ParsedTransaction{
		Date:         date,
		Description:  cleanDescription(desc),
		AmountCents:  amountCents,
		Category:     category,
		RawRow:       rowNum,
		CurrencyHint: currency,
	}, nil
}

// parseDate parses a date string using flexible format detection
func (p *Parser) parseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}

	// If specific format configured, try it first
	if p.config.DateFormat != "" {
		if t, err := time.Parse(p.config.DateFormat, s); err == nil {
			return t, nil
		}
	}

	// Try common formats
	formats := []string{
		"2006-01-02",           // ISO 8601
		"02/01/2006",           // DD/MM/YYYY (European)
		"01/02/2006",           // MM/DD/YYYY (American)
		"02-01-2006",           // DD-MM-YYYY
		"01-02-2006",           // MM-DD-YYYY
		"2006/01/02",           // YYYY/MM/DD
		"02.01.2006",           // DD.MM.YYYY (German)
		"2006-01-02T15:04:05Z", // ISO 8601 with time
		"2006-01-02 15:04:05",  // ISO with space
		"02/01/2006 15:04",     // European with time
		"01/02/2006 15:04",     // American with time
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unrecognized format: %s", s)
}

// parseAmount parses an amount string and returns cents and currency hint
func (p *Parser) parseAmount(s string) (int64, string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, "", fmt.Errorf("empty amount")
	}

	// Detect and extract currency
	currency := ""
	for _, sym := range []string{"$", "€", "£", "R$", "USD", "EUR", "GBP", "BRL"} {
		if strings.Contains(s, sym) {
			currency = sym
			s = strings.ReplaceAll(s, sym, "")
			break
		}
	}

	s = strings.TrimSpace(s)

	// Handle negative amounts
	negative := false
	if strings.HasPrefix(s, "-") || strings.HasPrefix(s, "(") {
		negative = true
		s = strings.TrimPrefix(s, "-")
		s = strings.Trim(s, "()")
	}

	// Parse based on format
	var amount float64
	var err error

	if p.config.IsEuropeanFormat {
		// European: 1.234,56
		s = strings.ReplaceAll(s, ".", "")  // Remove thousands separator
		s = strings.ReplaceAll(s, ",", ".") // Decimal separator to dot
	} else {
		// American: 1,234.56
		s = strings.ReplaceAll(s, ",", "") // Remove thousands separator
	}

	amount, err = strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, currency, fmt.Errorf("invalid number: %s", s)
	}

	if negative {
		amount = -amount
	}

	// Convert to cents
	cents := int64(amount * 100)

	return cents, currency, nil
}

// parseDebitCredit handles double-entry bookkeeping columns
func (p *Parser) parseDebitCredit(debitStr, creditStr string) (int64, string) {
	var currency string
	var amount int64

	// Try debit first (negative = money out)
	if debitStr != "" {
		cents, cur, err := p.parseAmount(debitStr)
		if err == nil && cents != 0 {
			currency = cur
			if cents > 0 {
				cents = -cents // Debit is negative (money out)
			}
			amount = cents
		}
	}

	// Try credit (positive = money in)
	if creditStr != "" && amount == 0 {
		cents, cur, err := p.parseAmount(creditStr)
		if err == nil && cents != 0 {
			currency = cur
			if cents < 0 {
				cents = -cents // Credit is positive (money in)
			}
			amount = cents
		}
	}

	return amount, currency
}

// coalesce returns the first non-empty string
func coalesce(values ...string) string {
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

// cleanDescription normalizes a transaction description
func cleanDescription(s string) string {
	s = strings.TrimSpace(s)
	// Remove multiple spaces
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return s
}

// skipLines returns a reader that skips the first n lines
func skipLines(r io.Reader, n int) io.Reader {
	// Wrap in a bufio.Reader-like line skipper
	return &lineSkipper{reader: r, skip: n}
}

type lineSkipper struct {
	reader  io.Reader
	skip    int
	skipped bool
}

func (ls *lineSkipper) Read(p []byte) (int, error) {
	if !ls.skipped {
		// Read and discard lines
		buf := make([]byte, 1)
		lines := 0
		for lines < ls.skip {
			n, err := ls.reader.Read(buf)
			if err != nil {
				return 0, err
			}
			if n > 0 && buf[0] == '\n' {
				lines++
			}
		}
		ls.skipped = true
	}
	return ls.reader.Read(p)
}
