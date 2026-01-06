// Package finance contains the NLP parser for Quick Capture natural language input.
package finance

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ParsedTransaction represents the result of parsing natural language input.
type ParsedTransaction struct {
	Description string    // Cleaned description text
	AmountMinor int64     // Amount in minor units (cents)
	Currency    string    // Detected currency code (EUR, USD)
	Date        time.Time // Transaction date (default: today)
	RawText     string    // Original input text
}

// NLPParser parses natural language transaction input.
type NLPParser struct {
	// Pattern for amounts: $1, 1$, €5, 5€, $10.50, 10.50$, etc.
	amountRegex *regexp.Regexp
}

// NewNLPParser creates a new NLP parser instance.
func NewNLPParser() *NLPParser {
	// Matches: $1, 1$, €5, 5€, $10.50, 10,50€, etc.
	// Groups: (currency_prefix)(amount)(currency_suffix)
	amountPattern := `(?:(\$|€|EUR|USD)\s*)?(\d+(?:[.,]\d{1,2})?)\s*(\$|€|EUR|USD)?`
	return &NLPParser{
		amountRegex: regexp.MustCompile(amountPattern),
	}
}

// Parse extracts transaction details from natural language input.
// Examples:
//   - "Coffee 1$" → Description: "Coffee", Amount: 100 (cents)
//   - "Dinner €25" → Description: "Dinner", Amount: 2500 (cents)
//   - "Uber 12.50$" → Description: "Uber", Amount: 1250 (cents)
//   - "Lunch with friends" → Description: "Lunch with friends", Amount: 0
func (p *NLPParser) Parse(rawText string) ParsedTransaction {
	result := ParsedTransaction{
		RawText: rawText,
		Date:    time.Now(),
	}

	// Find all amount matches
	matches := p.amountRegex.FindAllStringSubmatchIndex(rawText, -1)
	if len(matches) == 0 {
		// No amount found, entire text is description
		result.Description = strings.TrimSpace(rawText)
		return result
	}

	// Use the last match (most likely the amount)
	match := matches[len(matches)-1]
	fullMatchStart := match[0]
	fullMatchEnd := match[1]

	// Extract amount string
	amountStart := match[4] // Group 2 start
	amountEnd := match[5]   // Group 2 end
	amountStr := rawText[amountStart:amountEnd]

	// Detect currency from prefix or suffix
	result.Currency = p.detectCurrency(rawText, match)

	// Parse amount
	result.AmountMinor = p.parseAmount(amountStr)

	// Extract description (text without the amount part)
	description := rawText[:fullMatchStart] + rawText[fullMatchEnd:]
	result.Description = strings.TrimSpace(description)

	// Clean up common artifacts
	result.Description = p.cleanDescription(result.Description)

	return result
}

// detectCurrency extracts currency from regex match groups.
func (p *NLPParser) detectCurrency(text string, match []int) string {
	// Check prefix (group 1)
	if match[2] != -1 && match[3] != -1 {
		prefix := text[match[2]:match[3]]
		return p.normalizeCurrency(prefix)
	}

	// Check suffix (group 3)
	if match[6] != -1 && match[7] != -1 {
		suffix := text[match[6]:match[7]]
		return p.normalizeCurrency(suffix)
	}

	// Default to EUR
	return "EUR"
}

// normalizeCurrency converts currency symbols to ISO codes.
func (p *NLPParser) normalizeCurrency(symbol string) string {
	switch strings.ToUpper(symbol) {
	case "$", "USD":
		return "USD"
	case "€", "EUR":
		return "EUR"
	default:
		return "EUR"
	}
}

// parseAmount converts an amount string to minor units (cents).
func (p *NLPParser) parseAmount(amountStr string) int64 {
	// Handle European format (comma as decimal: 10,50)
	amountStr = strings.Replace(amountStr, ",", ".", 1)

	// Parse as float
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		return 0
	}

	// Convert to cents
	return int64(amount * 100)
}

// cleanDescription removes common artifacts from description.
func (p *NLPParser) cleanDescription(desc string) string {
	// Remove extra whitespace
	desc = strings.Join(strings.Fields(desc), " ")

	// Capitalize first letter
	if len(desc) > 0 {
		desc = strings.ToUpper(desc[:1]) + desc[1:]
	}

	return desc
}
