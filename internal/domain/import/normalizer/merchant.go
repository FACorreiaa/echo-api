// Package normalizer provides merchant sanitization and category detection.
// merchant.go handles merchant name normalization and automatic categorization.
package normalizer

import (
	"regexp"
	"strings"
)

// MerchantInfo contains normalized merchant information
type MerchantInfo struct {
	OriginalName   string `json:"original_name"`
	NormalizedName string `json:"normalized_name"`
	Category       string `json:"category,omitempty"`
	Subcategory    string `json:"subcategory,omitempty"`
}

// MerchantPattern defines a pattern for matching and normalizing merchants
type MerchantPattern struct {
	Pattern     *regexp.Regexp
	Name        string
	Category    string
	Subcategory string
}

// MerchantSanitizer normalizes merchant names and detects categories
type MerchantSanitizer struct {
	patterns []MerchantPattern
}

// NewMerchantSanitizer creates a new sanitizer with common merchant patterns
func NewMerchantSanitizer() *MerchantSanitizer {
	return &MerchantSanitizer{
		patterns: defaultMerchantPatterns(),
	}
}

// Sanitize normalizes a merchant name and detects its category
func (s *MerchantSanitizer) Sanitize(rawMerchant string) MerchantInfo {
	result := MerchantInfo{
		OriginalName:   rawMerchant,
		NormalizedName: rawMerchant,
	}

	// Clean the input
	cleaned := cleanMerchantName(rawMerchant)
	result.NormalizedName = cleaned

	// Try to match against known patterns
	for _, pattern := range s.patterns {
		if pattern.Pattern.MatchString(strings.ToUpper(cleaned)) {
			result.NormalizedName = pattern.Name
			result.Category = pattern.Category
			result.Subcategory = pattern.Subcategory
			return result
		}
	}

	// Fallback: title case the cleaned name
	result.NormalizedName = titleCase(cleaned)
	return result
}

// AddPattern adds a custom merchant pattern
func (s *MerchantSanitizer) AddPattern(pattern string, name, category, subcategory string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	s.patterns = append(s.patterns, MerchantPattern{
		Pattern:     re,
		Name:        name,
		Category:    category,
		Subcategory: subcategory,
	})
	return nil
}

// cleanMerchantName removes common noise from merchant names
func cleanMerchantName(raw string) string {
	// Trim whitespace
	result := strings.TrimSpace(raw)

	// Remove common prefixes
	prefixes := []string{
		"COMPRA ", "COMPRAS ", "PAGAMENTO ", "PAG ", "PGO ",
		"TRF ", "TRANSF ", "TRANSFERENCIA ",
		"MB WAY ", "MBWAY ", "MULTIBANCO ",
		"VISA ", "MASTERCARD ", "MAESTRO ",
		"PURCHASE ", "PAYMENT ", "POS ",
	}
	upper := strings.ToUpper(result)
	for _, prefix := range prefixes {
		if strings.HasPrefix(upper, prefix) {
			result = result[len(prefix):]
			break
		}
	}

	// Remove terminal/reference numbers at the end (e.g., "123456")
	refPattern := regexp.MustCompile(`\s+\d{4,}$`)
	result = refPattern.ReplaceAllString(result, "")

	// Remove date patterns at the end (e.g., "12/01")
	datePattern := regexp.MustCompile(`\s+\d{1,2}/\d{1,2}/?$`)
	result = datePattern.ReplaceAllString(result, "")

	// Collapse multiple spaces
	spacePattern := regexp.MustCompile(`\s+`)
	result = spacePattern.ReplaceAllString(result, " ")

	return strings.TrimSpace(result)
}

// titleCase converts a string to title case
func titleCase(s string) string {
	words := strings.Fields(s)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}
	return strings.Join(words, " ")
}

// defaultMerchantPatterns returns common merchant patterns for Portugal/EU
func defaultMerchantPatterns() []MerchantPattern {
	patterns := []MerchantPattern{
		// Portuguese Supermarkets
		{regexp.MustCompile(`(?i)PINGO\s*DOCE|PGO\s*DOCE`), "Pingo Doce", "Groceries", "Supermarket"},
		{regexp.MustCompile(`(?i)CONTINENTE`), "Continente", "Groceries", "Supermarket"},
		{regexp.MustCompile(`(?i)LIDL`), "Lidl", "Groceries", "Supermarket"},
		{regexp.MustCompile(`(?i)ALDI`), "Aldi", "Groceries", "Supermarket"},
		{regexp.MustCompile(`(?i)MERCADONA`), "Mercadona", "Groceries", "Supermarket"},
		{regexp.MustCompile(`(?i)MINIPRECO|MINI\s*PRECO`), "Minipreço", "Groceries", "Supermarket"},
		{regexp.MustCompile(`(?i)INTERMARCHE`), "Intermarché", "Groceries", "Supermarket"},

		// Coffee & Restaurants
		{regexp.MustCompile(`(?i)STARBUCKS`), "Starbucks", "Food & Drink", "Coffee"},
		{regexp.MustCompile(`(?i)MC\s*DONALDS|MCDONALD`), "McDonald's", "Food & Drink", "Fast Food"},
		{regexp.MustCompile(`(?i)BURGER\s*KING`), "Burger King", "Food & Drink", "Fast Food"},
		{regexp.MustCompile(`(?i)KFC`), "KFC", "Food & Drink", "Fast Food"},
		{regexp.MustCompile(`(?i)PIZZA\s*HUT`), "Pizza Hut", "Food & Drink", "Restaurant"},
		{regexp.MustCompile(`(?i)UBER\s*EATS`), "Uber Eats", "Food & Drink", "Delivery"},
		{regexp.MustCompile(`(?i)GLOVO`), "Glovo", "Food & Drink", "Delivery"},
		{regexp.MustCompile(`(?i)BOLT\s*FOOD`), "Bolt Food", "Food & Drink", "Delivery"},

		// Transport (Note: delivery patterns above will match first for UBER EATS, BOLT FOOD)
		{regexp.MustCompile(`(?i)\bUBER\b`), "Uber", "Transport", "Rideshare"},
		{regexp.MustCompile(`(?i)\bBOLT\b`), "Bolt", "Transport", "Rideshare"},
		{regexp.MustCompile(`(?i)FREE\s*NOW|FREENOW`), "Free Now", "Transport", "Rideshare"},
		{regexp.MustCompile(`(?i)VIVA\s*VIAGEM`), "Viva Viagem", "Transport", "Public Transit"},
		{regexp.MustCompile(`(?i)CP\s*-\s*COMBOIOS|COMBOIOS\s*PORTUGAL`), "CP", "Transport", "Train"},
		{regexp.MustCompile(`(?i)RYANAIR`), "Ryanair", "Transport", "Flights"},
		{regexp.MustCompile(`(?i)TAP\s*PORTUGAL|TAP\s*AIR`), "TAP", "Transport", "Flights"},

		// Utilities
		{regexp.MustCompile(`(?i)EDP\s*`), "EDP", "Utilities", "Electricity"},
		{regexp.MustCompile(`(?i)EPAL`), "EPAL", "Utilities", "Water"},
		{regexp.MustCompile(`(?i)GALP`), "Galp", "Utilities", "Gas"},
		{regexp.MustCompile(`(?i)MEO|ALTICE`), "MEO", "Utilities", "Telecom"},
		{regexp.MustCompile(`(?i)NOS\s*`), "NOS", "Utilities", "Telecom"},
		{regexp.MustCompile(`(?i)VODAFONE`), "Vodafone", "Utilities", "Telecom"},

		// Shopping
		{regexp.MustCompile(`(?i)AMAZON`), "Amazon", "Shopping", "Online"},
		{regexp.MustCompile(`(?i)ZARA`), "Zara", "Shopping", "Clothing"},
		{regexp.MustCompile(`(?i)H\s*&\s*M|H&M`), "H&M", "Shopping", "Clothing"},
		{regexp.MustCompile(`(?i)PRIMARK`), "Primark", "Shopping", "Clothing"},
		{regexp.MustCompile(`(?i)IKEA`), "IKEA", "Shopping", "Home"},
		{regexp.MustCompile(`(?i)WORTEN`), "Worten", "Shopping", "Electronics"},
		{regexp.MustCompile(`(?i)FNAC`), "FNAC", "Shopping", "Electronics"},

		// Entertainment
		{regexp.MustCompile(`(?i)NETFLIX`), "Netflix", "Entertainment", "Streaming"},
		{regexp.MustCompile(`(?i)SPOTIFY`), "Spotify", "Entertainment", "Streaming"},
		{regexp.MustCompile(`(?i)DISNEY\s*\+|DISNEYPLUS`), "Disney+", "Entertainment", "Streaming"},
		{regexp.MustCompile(`(?i)APPLE\.COM|APPLE\s*MUSIC`), "Apple", "Entertainment", "Streaming"},
		{regexp.MustCompile(`(?i)PLAYSTATION|PSN`), "PlayStation", "Entertainment", "Gaming"},
		{regexp.MustCompile(`(?i)XBOX|MICROSOFT\s*GAMES`), "Xbox", "Entertainment", "Gaming"},
		{regexp.MustCompile(`(?i)STEAM`), "Steam", "Entertainment", "Gaming"},

		// Health
		{regexp.MustCompile(`(?i)CONTINENTE\s*SAUDE|FARMACIA`), "Farmácia", "Health", "Pharmacy"},
		{regexp.MustCompile(`(?i)WELLS`), "Wells", "Health", "Pharmacy"},

		// Banks/Finance
		{regexp.MustCompile(`(?i)CGD|CAIXA\s*GERAL`), "CGD", "Finance", "Bank"},
		{regexp.MustCompile(`(?i)BPI`), "BPI", "Finance", "Bank"},
		{regexp.MustCompile(`(?i)MILLENNIUM`), "Millennium BCP", "Finance", "Bank"},
		{regexp.MustCompile(`(?i)SANTANDER`), "Santander", "Finance", "Bank"},
		{regexp.MustCompile(`(?i)NOVO\s*BANCO`), "Novo Banco", "Finance", "Bank"},
		{regexp.MustCompile(`(?i)REVOLUT`), "Revolut", "Finance", "Digital Bank"},
		{regexp.MustCompile(`(?i)PAYPAL`), "PayPal", "Finance", "Payment"},
	}
	return patterns
}
