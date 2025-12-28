// Package sniffer provides automatic detection of CSV/TSV file formats.
// It identifies delimiters, header rows, and generates fingerprints for bank recognition.
package sniffer

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"unicode"
)

// Common bank statement header keywords (multi-language)
var headerKeywords = []string{
	// Portuguese
	"data mov", "data mov.", "descrição", "descricao", "débito", "debito", "crédito", "credito",
	"data valor", "saldo", "categoria",
	// English
	"date", "description", "amount", "debit", "credit", "balance", "category", "merchant",
	// Spanish
	"fecha", "descripción", "descripcion", "importe", "cargo", "abono",
}

// FileConfig holds the detected configuration for a CSV/TSV file
type FileConfig struct {
	Delimiter   rune       // The field delimiter (';', ',', '\t')
	SkipLines   int        // Number of metadata lines before headers
	Headers     []string   // Detected header names
	Fingerprint string     // SHA256 hash of normalized headers
	SampleRows  [][]string // First few data rows for preview
}

// DetectOptions allows callers to override header row or delimiter detection.
type DetectOptions struct {
	// HeaderRowIndex is a 0-based index for the header row. Set to -1 to auto-detect.
	HeaderRowIndex int
	// Delimiter overrides the detected delimiter when non-zero.
	Delimiter rune
}

// ColumnSuggestions provides auto-detected column indices
type ColumnSuggestions struct {
	DateCol       int  // Suggested date column index (-1 if not found)
	DescCol       int  // Suggested description column index
	AmountCol     int  // Suggested single amount column (-1 if separate debit/credit)
	DebitCol      int  // Suggested debit column index
	CreditCol     int  // Suggested credit column index
	CategoryCol   int  // Suggested category column index (-1 if not found)
	IsDoubleEntry bool // True if separate debit/credit columns detected
}

// RegionalDialect represents inferred regional formatting for amounts and dates
type RegionalDialect struct {
	DecimalSeparator   rune    // '.' (US) or ',' (EU)
	ThousandsSeparator rune    // ',' (US) or '.' (EU)
	DateFormat         string  // "DD/MM/YYYY" or "MM/DD/YYYY"
	CurrencyHint       string  // "EUR", "USD", "BRL" if detected
	Confidence         float64 // 0.0-1.0 confidence score
	IsEuropeanFormat   bool    // Convenience flag: true if comma is decimal separator
}

// ProbeDialect analyzes sample rows to infer the regional "dialect" of the file.
// It examines amount columns for decimal separators and date columns for format.
func ProbeDialect(sampleRows [][]string, amountIdx int, dateIdx int) *RegionalDialect {
	dialect := &RegionalDialect{
		DecimalSeparator:   '.',
		ThousandsSeparator: ',',
		DateFormat:         "MM/DD/YYYY",
		Confidence:         0.5,
		IsEuropeanFormat:   false,
	}

	europeanHints := 0
	usHints := 0
	totalAmountSamples := 0
	dateIsDD := false
	dateIsMM := false

	for _, row := range sampleRows {
		// Analyze amount column for decimal separator
		if amountIdx >= 0 && amountIdx < len(row) {
			val := row[amountIdx]
			if val != "" {
				totalAmountSamples++
				hint := analyzeAmountFormat(val)
				if hint > 0 {
					europeanHints++
				} else if hint < 0 {
					usHints++
				}
			}
		}

		// Analyze date column for DD/MM vs MM/DD
		if dateIdx >= 0 && dateIdx < len(row) {
			dateVal := row[dateIdx]
			if dateVal != "" {
				ddFirst := analyzeDateFormat(dateVal)
				if ddFirst {
					dateIsDD = true
				} else {
					dateIsMM = true
				}
			}
		}

		// Look for currency symbols in all columns
		for _, cell := range row {
			if strings.Contains(cell, "€") || strings.Contains(cell, "EUR") {
				dialect.CurrencyHint = "EUR"
				europeanHints++
			} else if strings.Contains(cell, "R$") || strings.Contains(cell, "BRL") {
				dialect.CurrencyHint = "BRL"
				europeanHints++ // Brazil uses European format
			} else if strings.Contains(cell, "$") && !strings.Contains(cell, "R$") {
				if dialect.CurrencyHint == "" {
					dialect.CurrencyHint = "USD"
				}
				usHints++
			}
		}
	}

	// Determine format based on hints
	if europeanHints > usHints {
		dialect.DecimalSeparator = ','
		dialect.ThousandsSeparator = '.'
		dialect.IsEuropeanFormat = true
	} else if usHints > europeanHints {
		dialect.DecimalSeparator = '.'
		dialect.ThousandsSeparator = ','
		dialect.IsEuropeanFormat = false
	}

	// Calculate confidence
	totalHints := europeanHints + usHints
	if totalHints > 0 {
		winningHints := europeanHints
		if usHints > europeanHints {
			winningHints = usHints
		}
		dialect.Confidence = float64(winningHints) / float64(totalHints)
	}

	// Date format
	if dateIsDD && !dateIsMM {
		dialect.DateFormat = "DD/MM/YYYY"
	} else if !dateIsDD && dateIsMM {
		dialect.DateFormat = "MM/DD/YYYY"
	} else {
		// Ambiguous - default to European if other hints suggest it
		if dialect.IsEuropeanFormat {
			dialect.DateFormat = "DD/MM/YYYY"
		}
	}

	return dialect
}

// analyzeAmountFormat returns: >0 for European, <0 for US, 0 for ambiguous
func analyzeAmountFormat(val string) int {
	// Clean the value
	cleaned := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' || r == ',' || r == '.' || r == '-' {
			return r
		}
		return -1
	}, val)
	cleaned = strings.TrimPrefix(cleaned, "-")

	if cleaned == "" {
		return 0
	}

	hasComma := strings.Contains(cleaned, ",")
	hasDot := strings.Contains(cleaned, ".")

	switch {
	case hasComma && hasDot:
		// Both present: last one is decimal separator
		if strings.LastIndex(cleaned, ",") > strings.LastIndex(cleaned, ".") {
			return 1 // European: 1.234,56
		}
		return -1 // US: 1,234.56

	case hasComma && !hasDot:
		// Only comma: check if it looks like a decimal (max 2 digits after)
		idx := strings.LastIndex(cleaned, ",")
		afterComma := cleaned[idx+1:]
		if len(afterComma) <= 2 {
			return 1 // Likely European decimal
		}
		return 0 // Could be US thousands separator

	case hasDot && !hasComma:
		// Only dot: check if it looks like a decimal
		idx := strings.LastIndex(cleaned, ".")
		afterDot := cleaned[idx+1:]
		if len(afterDot) <= 2 {
			return -1 // Likely US decimal
		}
		return 0 // Could be European thousands separator
	}

	return 0
}

// analyzeDateFormat returns true if the date is definitely DD-first (day > 12)
func analyzeDateFormat(dateVal string) bool {
	// Split by common separators
	parts := strings.FieldsFunc(dateVal, func(r rune) bool {
		return r == '/' || r == '-' || r == '.'
	})

	if len(parts) >= 2 {
		// Try to parse the first part as a number
		firstPart := strings.TrimSpace(parts[0])
		var day int
		for _, c := range firstPart {
			if c >= '0' && c <= '9' {
				day = day*10 + int(c-'0')
			} else {
				break
			}
		}
		// If first part is > 12, it must be a day
		if day > 12 && day <= 31 {
			return true
		}
	}
	return false
}

var (
	ErrEmptyFile        = errors.New("file is empty")
	ErrNoHeadersFound   = errors.New("could not find data headers")
	ErrInvalidDelimiter = errors.New("could not detect valid delimiter")
)

// DetectConfig analyzes a CSV/TSV file and returns its configuration
func DetectConfig(data []byte) (*FileConfig, error) {
	return DetectConfigWithOptions(data, nil)
}

// DetectConfigWithOptions analyzes a CSV/TSV file with optional overrides.
func DetectConfigWithOptions(data []byte, opts *DetectOptions) (*FileConfig, error) {
	if len(data) == 0 {
		return nil, ErrEmptyFile
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return nil, ErrEmptyFile
	}

	var (
		delimiter rune
		skipLines int
		err       error
	)
	if opts != nil && opts.HeaderRowIndex >= 0 {
		if opts.HeaderRowIndex >= len(lines) {
			return nil, ErrNoHeadersFound
		}
		skipLines = opts.HeaderRowIndex
		if opts.Delimiter != 0 {
			delimiter = opts.Delimiter
		} else {
			line := cleanLine(lines[skipLines], skipLines == 0)
			delimiter, _ = detectDelimiter(line)
			if delimiter == 0 {
				return nil, ErrInvalidDelimiter
			}
		}
	} else {
		// Try to find the header row
		delimiter, skipLines, err = findHeaderRow(lines)
		if err != nil {
			return nil, err
		}
	}

	// Parse headers
	headerLine := cleanLine(lines[skipLines], skipLines == 0)
	reader := csv.NewReader(strings.NewReader(headerLine))
	reader.Comma = delimiter
	reader.LazyQuotes = true

	headers, err := reader.Read()
	if err != nil {
		return nil, err
	}

	// Clean headers
	for i, h := range headers {
		headers[i] = strings.TrimSpace(h)
	}

	// Generate fingerprint
	fingerprint := generateFingerprint(headers)

	// Get sample rows (up to 5)
	sampleRows := getSampleRows(data, delimiter, skipLines+1, 5)

	return &FileConfig{
		Delimiter:   delimiter,
		SkipLines:   skipLines,
		Headers:     headers,
		Fingerprint: fingerprint,
		SampleRows:  sampleRows,
	}, nil
}

// SuggestColumns attempts to auto-match columns based on header names
func SuggestColumns(headers []string) *ColumnSuggestions {
	suggestions := &ColumnSuggestions{
		DateCol:     -1,
		DescCol:     -1,
		AmountCol:   -1,
		DebitCol:    -1,
		CreditCol:   -1,
		CategoryCol: -1,
	}

	for i, header := range headers {
		h := strings.ToLower(strings.TrimSpace(header))

		// Date detection
		if suggestions.DateCol == -1 {
			if strings.Contains(h, "data mov") || strings.Contains(h, "date") ||
				strings.Contains(h, "fecha") || h == "data" {
				suggestions.DateCol = i
			}
		}

		// Description detection
		if suggestions.DescCol == -1 {
			if strings.Contains(h, "descri") || strings.Contains(h, "merchant") ||
				strings.Contains(h, "description") || h == "nome" || h == "name" {
				suggestions.DescCol = i
			}
		}

		// Debit detection
		if suggestions.DebitCol == -1 {
			if strings.Contains(h, "débito") || strings.Contains(h, "debito") ||
				strings.Contains(h, "debit") || strings.Contains(h, "cargo") {
				suggestions.DebitCol = i
			}
		}

		// Credit detection
		if suggestions.CreditCol == -1 {
			if strings.Contains(h, "crédito") || strings.Contains(h, "credito") ||
				strings.Contains(h, "credit") || strings.Contains(h, "abono") {
				suggestions.CreditCol = i
			}
		}

		// Single amount detection
		if suggestions.AmountCol == -1 {
			if h == "amount" || h == "valor" || h == "importe" || h == "montante" {
				suggestions.AmountCol = i
			}
		}

		// Category detection
		if suggestions.CategoryCol == -1 {
			if strings.Contains(h, "categ") || strings.Contains(h, "category") ||
				strings.Contains(h, "tipo") || strings.Contains(h, "type") {
				suggestions.CategoryCol = i
			}
		}
	}

	// Determine if double-entry (separate debit/credit)
	suggestions.IsDoubleEntry = suggestions.DebitCol != -1 && suggestions.CreditCol != -1

	return suggestions
}

// findHeaderRow locates the header row and its delimiter
func findHeaderRow(lines []string) (rune, int, error) {
	// Track the best candidate among lines with no keywords (fallback)
	fallbackIndex := -1
	fallbackDelimiter := rune(0)
	fallbackCount := 0

	// Track the best candidate among lines WITH keywords (preferred)
	keywordIndex := -1
	keywordDelimiter := rune(0)
	keywordCount := 0

	for i, line := range lines {
		if i > 20 { // Don't search more than 20 lines
			break
		}

		line = cleanLine(line, i == 0)
		if line == "" {
			continue
		}
		lineLower := strings.ToLower(line)

		// Detect delimiter and column count for this line
		delimiter, count := detectDelimiter(line)
		if count < 1 {
			continue // Not enough columns to be a valid header
		}

		// Check if this line contains header keywords
		keywordMatches := 0
		for _, kw := range headerKeywords {
			if strings.Contains(lineLower, kw) {
				keywordMatches++
			}
		}

		if keywordMatches > 0 {
			// Prefer lines with MORE columns (real headers have many columns, metadata has few)
			// Also prefer lines with multiple keyword matches
			score := count*10 + keywordMatches
			bestScore := keywordCount*10 + 1 // Approximate previous score
			if keywordIndex == -1 || score > bestScore || (score == bestScore && count > keywordCount) {
				keywordCount = count
				keywordDelimiter = delimiter
				keywordIndex = i
			}
		} else {
			// Track best non-keyword line as fallback
			if count > fallbackCount {
				fallbackCount = count
				fallbackDelimiter = delimiter
				fallbackIndex = i
			}
		}
	}

	// Prefer keyword-containing lines with at least 3 columns
	if keywordIndex >= 0 && keywordCount >= 2 {
		return keywordDelimiter, keywordIndex, nil
	}

	// Fall back to non-keyword line with most columns
	if fallbackCount >= 2 {
		return fallbackDelimiter, fallbackIndex, nil
	}

	return 0, 0, ErrNoHeadersFound
}

func cleanLine(line string, firstLine bool) string {
	line = strings.TrimRight(line, "\r")
	if firstLine {
		line = strings.TrimPrefix(line, "\uFEFF")
	}
	return strings.TrimSpace(line)
}

func detectDelimiter(line string) (rune, int) {
	delimiters := []rune{';', '\t', ',', '|'}
	bestDelimiter := rune(0)
	bestCount := 0
	for _, d := range delimiters {
		count := strings.Count(line, string(d))
		if count > bestCount {
			bestCount = count
			bestDelimiter = d
		}
	}
	return bestDelimiter, bestCount
}

// generateFingerprint creates a unique hash from header names
func generateFingerprint(headers []string) string {
	// Normalize headers: lowercase, remove non-alphanumeric, sort-ish
	var normalized []string
	for _, h := range headers {
		clean := strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				return unicode.ToLower(r)
			}
			return -1
		}, h)
		if clean != "" {
			normalized = append(normalized, clean)
		}
	}

	// Join and hash
	joined := strings.Join(normalized, "|")
	hash := sha256.Sum256([]byte(joined))
	return hex.EncodeToString(hash[:])
}

// getSampleRows returns the first N data rows after the header
func getSampleRows(data []byte, delimiter rune, startLine, maxRows int) [][]string {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.Comma = delimiter
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1 // Allow variable fields

	var rows [][]string
	lineNum := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		if lineNum >= startLine {
			rows = append(rows, record)
			if len(rows) >= maxRows {
				break
			}
		}
		lineNum++
	}

	return rows
}
