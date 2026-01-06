// Package money provides currency-safe financial arithmetic using integer cents
// and the Fowler Money pattern. It ensures precision for all financial calculations
// and proper handling of ISO-4217 currency codes.
package money

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Rhymond/go-money"
	"github.com/shopspring/decimal"
)

// Common currency codes (ISO-4217)
const (
	USD = "USD" // US Dollar
	EUR = "EUR" // Euro
	GBP = "GBP" // British Pound
	BRL = "BRL" // Brazilian Real
	JPY = "JPY" // Japanese Yen (no decimal places)
	CHF = "CHF" // Swiss Franc
	CAD = "CAD" // Canadian Dollar
	AUD = "AUD" // Australian Dollar
	CNY = "CNY" // Chinese Yuan
	MXN = "MXN" // Mexican Peso
)

// Money represents a monetary value with currency.
// It wraps go-money for safe arithmetic and shopspring/decimal for precision calculations.
type Money struct {
	m *money.Money
}

// New creates a new Money value from cents (minor units) and currency code.
// For JPY and other zero-decimal currencies, amount is the actual value.
func New(amountCents int64, currencyCode string) *Money {
	return &Money{
		m: money.New(amountCents, currencyCode),
	}
}

// NewFromFloat creates Money from a floating-point value.
// Use with caution - prefer New() with integer cents when possible.
func NewFromFloat(amount float64, currencyCode string) *Money {
	currency := money.GetCurrency(currencyCode)
	if currency == nil {
		currency = money.GetCurrency(USD)
	}

	// Convert to cents using decimal for precision
	d := decimal.NewFromFloat(amount)
	multiplier := decimal.New(1, int32(currency.Fraction))
	cents := d.Mul(multiplier).Round(0).IntPart()

	return New(cents, currencyCode)
}

// NewFromDecimal creates Money from a decimal.Decimal value.
// This is the safest way to create Money from a non-integer value.
func NewFromDecimal(amount decimal.Decimal, currencyCode string) *Money {
	currency := money.GetCurrency(currencyCode)
	if currency == nil {
		currency = money.GetCurrency(USD)
	}

	multiplier := decimal.New(1, int32(currency.Fraction))
	cents := amount.Mul(multiplier).Round(0).IntPart()

	return New(cents, currencyCode)
}

// NewFromString parses a string amount and currency.
// Accepts formats like "100.50", "1,234.56", "1.234,56" (European)
func NewFromString(amount string, currencyCode string, europeanFormat bool) (*Money, error) {
	// Clean the string
	amount = strings.TrimSpace(amount)
	amount = strings.ReplaceAll(amount, " ", "")

	// Remove currency symbols
	for _, sym := range []string{"$", "€", "£", "R$", "¥", "₹"} {
		amount = strings.ReplaceAll(amount, sym, "")
	}

	if europeanFormat {
		// European: 1.234,56 -> 1234.56
		amount = strings.ReplaceAll(amount, ".", "")
		amount = strings.ReplaceAll(amount, ",", ".")
	} else {
		// American: 1,234.56 -> 1234.56
		amount = strings.ReplaceAll(amount, ",", "")
	}

	d, err := decimal.NewFromString(amount)
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %w", err)
	}

	return NewFromDecimal(d, currencyCode), nil
}

// Zero returns a zero Money value for the given currency
func Zero(currencyCode string) *Money {
	return New(0, currencyCode)
}

// Amount returns the amount in minor units (cents)
func (m *Money) Amount() int64 {
	if m == nil || m.m == nil {
		return 0
	}
	return m.m.Amount()
}

// Currency returns the ISO-4217 currency code
func (m *Money) Currency() string {
	if m == nil || m.m == nil {
		return ""
	}
	return m.m.Currency().Code
}

// CurrencySymbol returns the currency symbol (e.g., "$", "€")
func (m *Money) CurrencySymbol() string {
	if m == nil || m.m == nil {
		return ""
	}
	return m.m.Currency().Grapheme
}

// IsZero returns true if the amount is zero
func (m *Money) IsZero() bool {
	return m == nil || m.m == nil || m.m.IsZero()
}

// IsPositive returns true if the amount is greater than zero
func (m *Money) IsPositive() bool {
	return m != nil && m.m != nil && m.m.IsPositive()
}

// IsNegative returns true if the amount is less than zero
func (m *Money) IsNegative() bool {
	return m != nil && m.m != nil && m.m.IsNegative()
}

// Abs returns the absolute value
func (m *Money) Abs() *Money {
	if m == nil || m.m == nil {
		return Zero(USD)
	}
	return &Money{m: m.m.Absolute()}
}

// Negate returns the negated value
func (m *Money) Negate() *Money {
	if m == nil || m.m == nil {
		return Zero(USD)
	}
	return &Money{m: m.m.Negative()}
}

// Add adds two Money values. Returns error if currencies don't match.
func (m *Money) Add(other *Money) (*Money, error) {
	if m == nil || m.m == nil {
		return other, nil
	}
	if other == nil || other.m == nil {
		return m, nil
	}

	result, err := m.m.Add(other.m)
	if err != nil {
		return nil, err
	}
	return &Money{m: result}, nil
}

// MustAdd adds two Money values, panics if currencies don't match.
func (m *Money) MustAdd(other *Money) *Money {
	result, err := m.Add(other)
	if err != nil {
		panic(err)
	}
	return result
}

// Subtract subtracts other from m. Returns error if currencies don't match.
func (m *Money) Subtract(other *Money) (*Money, error) {
	if m == nil || m.m == nil {
		if other == nil {
			return Zero(USD), nil
		}
		return other.Negate(), nil
	}
	if other == nil || other.m == nil {
		return m, nil
	}

	result, err := m.m.Subtract(other.m)
	if err != nil {
		return nil, err
	}
	return &Money{m: result}, nil
}

// MustSubtract subtracts other from m, panics if currencies don't match.
func (m *Money) MustSubtract(other *Money) *Money {
	result, err := m.Subtract(other)
	if err != nil {
		panic(err)
	}
	return result
}

// Multiply multiplies by an integer factor
func (m *Money) Multiply(factor int64) *Money {
	if m == nil || m.m == nil {
		return Zero(USD)
	}
	return &Money{m: m.m.Multiply(factor)}
}

// Equals returns true if both values are equal
func (m *Money) Equals(other *Money) bool {
	if m == nil || m.m == nil {
		return other == nil || other.m == nil || other.IsZero()
	}
	if other == nil || other.m == nil {
		return m.IsZero()
	}
	eq, _ := m.m.Equals(other.m)
	return eq
}

// LessThan returns true if m < other
func (m *Money) LessThan(other *Money) bool {
	if m == nil || m.m == nil || other == nil || other.m == nil {
		return false
	}
	lt, _ := m.m.LessThan(other.m)
	return lt
}

// GreaterThan returns true if m > other
func (m *Money) GreaterThan(other *Money) bool {
	if m == nil || m.m == nil || other == nil || other.m == nil {
		return false
	}
	gt, _ := m.m.GreaterThan(other.m)
	return gt
}

// Compare returns -1 if m < other, 0 if equal, 1 if m > other
func (m *Money) Compare(other *Money) int {
	if m == nil || m.m == nil {
		if other == nil || other.m == nil || other.IsZero() {
			return 0
		}
		if other.IsPositive() {
			return -1
		}
		return 1
	}
	cmp, _ := m.m.Compare(other.m)
	return cmp
}

// Display returns a formatted string for display (e.g., "$1,234.56")
func (m *Money) Display() string {
	if m == nil || m.m == nil {
		return "$0.00"
	}
	return m.m.Display()
}

// String returns the amount as a decimal string (e.g., "1234.56")
func (m *Money) String() string {
	if m == nil || m.m == nil {
		return "0.00"
	}
	return m.ToDecimal().String()
}

// ToDecimal converts to decimal.Decimal for precise calculations
func (m *Money) ToDecimal() decimal.Decimal {
	if m == nil || m.m == nil {
		return decimal.Zero
	}
	currency := m.m.Currency()
	d := decimal.NewFromInt(m.m.Amount())
	divisor := decimal.New(1, int32(currency.Fraction))
	return d.Div(divisor)
}

// ToFloat64 converts to float64 (use with caution for display only)
func (m *Money) ToFloat64() float64 {
	return m.ToDecimal().InexactFloat64()
}

// Split divides money into n equal parts, distributing remainder to first parts.
// This ensures no money is lost in division.
func (m *Money) Split(n int) ([]*Money, error) {
	if m == nil || m.m == nil {
		return nil, errors.New("cannot split nil money")
	}
	if n <= 0 {
		return nil, errors.New("n must be positive")
	}

	parts, err := m.m.Split(n)
	if err != nil {
		return nil, err
	}

	result := make([]*Money, len(parts))
	for i, p := range parts {
		result[i] = &Money{m: p}
	}
	return result, nil
}

// Allocate splits money according to given ratios.
// Ratios don't need to sum to 100 - they're relative weights.
// Example: Allocate([]int{1, 1, 1}) splits into thirds.
func (m *Money) Allocate(ratios []int) ([]*Money, error) {
	if m == nil || m.m == nil {
		return nil, errors.New("cannot allocate nil money")
	}

	parts, err := m.m.Allocate(ratios...)
	if err != nil {
		return nil, err
	}

	result := make([]*Money, len(parts))
	for i, p := range parts {
		result[i] = &Money{m: p}
	}
	return result, nil
}

// JSON marshaling
func (m *Money) MarshalJSON() ([]byte, error) {
	if m == nil || m.m == nil {
		return json.Marshal(nil)
	}
	return json.Marshal(map[string]interface{}{
		"amount":   m.Amount(),
		"currency": m.Currency(),
		"display":  m.Display(),
	})
}

func (m *Money) UnmarshalJSON(data []byte) error {
	var v struct {
		Amount   int64  `json:"amount"`
		Currency string `json:"currency"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	m.m = money.New(v.Amount, v.Currency)
	return nil
}

// SQL scanning
func (m *Money) Scan(value interface{}) error {
	if value == nil {
		m.m = nil
		return nil
	}

	switch v := value.(type) {
	case int64:
		m.m = money.New(v, USD) // Default to USD if only amount provided
		return nil
	case float64:
		m.m = money.New(int64(v*100), USD)
		return nil
	default:
		return fmt.Errorf("cannot scan %T into Money", value)
	}
}

func (m *Money) Value() (driver.Value, error) {
	if m == nil || m.m == nil {
		return nil, nil
	}
	return m.Amount(), nil
}

// SameCurrency returns true if both have the same currency
func (m *Money) SameCurrency(other *Money) bool {
	if m == nil || m.m == nil || other == nil || other.m == nil {
		return false
	}
	return m.m.SameCurrency(other.m)
}

// Convert converts to a different currency using the given exchange rate.
// Rate is how many units of target currency per unit of source currency.
func (m *Money) Convert(targetCurrency string, rate decimal.Decimal) *Money {
	if m == nil || m.m == nil {
		return Zero(targetCurrency)
	}

	sourceDecimal := m.ToDecimal()
	targetDecimal := sourceDecimal.Mul(rate)

	return NewFromDecimal(targetDecimal, targetCurrency)
}

// ============================================================================
// Precision Calculations for Tax, Interest, and Percentages
// ============================================================================

// Percentage calculates a percentage of the amount.
// percent is the percentage value (e.g., 15.5 for 15.5%)
// Uses decimal arithmetic for precision.
func (m *Money) Percentage(percent float64) *Money {
	if m == nil || m.m == nil {
		return Zero(USD)
	}

	d := m.ToDecimal()
	pct := decimal.NewFromFloat(percent).Div(decimal.NewFromInt(100))
	result := d.Mul(pct)

	return NewFromDecimal(result, m.Currency())
}

// PercentageDecimal calculates a percentage using decimal.Decimal for maximum precision.
// percent is the percentage value (e.g., decimal.NewFromFloat(15.5) for 15.5%)
func (m *Money) PercentageDecimal(percent decimal.Decimal) *Money {
	if m == nil || m.m == nil {
		return Zero(USD)
	}

	d := m.ToDecimal()
	pct := percent.Div(decimal.NewFromInt(100))
	result := d.Mul(pct)

	return NewFromDecimal(result, m.Currency())
}

// AddPercentage adds a percentage to the amount (e.g., for markup).
// percent is the percentage value (e.g., 10 for 10%)
func (m *Money) AddPercentage(percent float64) *Money {
	if m == nil || m.m == nil {
		return Zero(USD)
	}

	addition := m.Percentage(percent)
	result, _ := m.Add(addition)
	return result
}

// SubtractPercentage subtracts a percentage from the amount (e.g., for discounts).
// percent is the percentage value (e.g., 10 for 10%)
func (m *Money) SubtractPercentage(percent float64) *Money {
	if m == nil || m.m == nil {
		return Zero(USD)
	}

	subtraction := m.Percentage(percent)
	result, _ := m.Subtract(subtraction)
	return result
}

// Tax calculates the tax amount for a given tax rate.
// taxRate is the tax percentage (e.g., 8.25 for 8.25%)
func (m *Money) Tax(taxRate float64) *Money {
	return m.Percentage(taxRate)
}

// WithTax returns the total amount including tax.
// taxRate is the tax percentage (e.g., 8.25 for 8.25%)
func (m *Money) WithTax(taxRate float64) *Money {
	return m.AddPercentage(taxRate)
}

// ExtractTax extracts the tax portion from a tax-inclusive amount.
// taxRate is the tax percentage (e.g., 8.25 for 8.25%)
// Returns the tax amount that was included in the total.
func (m *Money) ExtractTax(taxRate float64) *Money {
	if m == nil || m.m == nil {
		return Zero(USD)
	}

	// If total = base + (base * rate/100)
	// Then total = base * (1 + rate/100)
	// So base = total / (1 + rate/100)
	// And tax = total - base

	d := m.ToDecimal()
	rate := decimal.NewFromFloat(taxRate).Div(decimal.NewFromInt(100))
	divisor := decimal.NewFromInt(1).Add(rate)
	base := d.Div(divisor)
	tax := d.Sub(base)

	return NewFromDecimal(tax, m.Currency())
}

// BaseFromTaxInclusive extracts the base amount from a tax-inclusive amount.
// taxRate is the tax percentage (e.g., 8.25 for 8.25%)
func (m *Money) BaseFromTaxInclusive(taxRate float64) *Money {
	if m == nil || m.m == nil {
		return Zero(USD)
	}

	d := m.ToDecimal()
	rate := decimal.NewFromFloat(taxRate).Div(decimal.NewFromInt(100))
	divisor := decimal.NewFromInt(1).Add(rate)
	base := d.Div(divisor)

	return NewFromDecimal(base, m.Currency())
}

// SimpleInterest calculates simple interest.
// annualRate is the annual interest rate as a percentage (e.g., 5.5 for 5.5%)
// years is the time period in years (can be fractional, e.g., 0.5 for 6 months)
func (m *Money) SimpleInterest(annualRate float64, years float64) *Money {
	if m == nil || m.m == nil {
		return Zero(USD)
	}

	// I = P * r * t
	d := m.ToDecimal()
	rate := decimal.NewFromFloat(annualRate).Div(decimal.NewFromInt(100))
	time := decimal.NewFromFloat(years)
	interest := d.Mul(rate).Mul(time)

	return NewFromDecimal(interest, m.Currency())
}

// WithSimpleInterest returns principal plus simple interest.
func (m *Money) WithSimpleInterest(annualRate float64, years float64) *Money {
	interest := m.SimpleInterest(annualRate, years)
	result, _ := m.Add(interest)
	return result
}

// CompoundInterest calculates compound interest.
// annualRate is the annual interest rate as a percentage (e.g., 5.5 for 5.5%)
// years is the time period in years
// compoundingsPerYear is how often interest compounds (e.g., 12 for monthly, 365 for daily)
func (m *Money) CompoundInterest(annualRate float64, years float64, compoundingsPerYear int) *Money {
	if m == nil || m.m == nil || compoundingsPerYear <= 0 {
		return Zero(USD)
	}

	// A = P * (1 + r/n)^(n*t) - P
	// Where: P = principal, r = annual rate, n = compoundings per year, t = years
	principal := m.ToDecimal()
	rate := decimal.NewFromFloat(annualRate).Div(decimal.NewFromInt(100))
	n := decimal.NewFromInt(int64(compoundingsPerYear))
	t := decimal.NewFromFloat(years)

	// Calculate (1 + r/n)
	ratePerPeriod := rate.Div(n)
	base := decimal.NewFromInt(1).Add(ratePerPeriod)

	// Calculate exponent n*t
	exponent := n.Mul(t).IntPart()

	// Calculate base^exponent using repeated multiplication for precision
	result := decimal.NewFromInt(1)
	for i := int64(0); i < exponent; i++ {
		result = result.Mul(base)
	}

	// Handle fractional exponent part if any
	fractionalExp := n.Mul(t).Sub(decimal.NewFromInt(exponent))
	if !fractionalExp.IsZero() {
		// Approximate fractional exponent using linear interpolation
		// This is a simplification; for exact results, use math.Pow
		fractionalResult := decimal.NewFromInt(1).Add(ratePerPeriod.Mul(fractionalExp))
		result = result.Mul(fractionalResult)
	}

	// Final amount
	finalAmount := principal.Mul(result)
	interest := finalAmount.Sub(principal)

	return NewFromDecimal(interest, m.Currency())
}

// WithCompoundInterest returns principal plus compound interest.
func (m *Money) WithCompoundInterest(annualRate float64, years float64, compoundingsPerYear int) *Money {
	interest := m.CompoundInterest(annualRate, years, compoundingsPerYear)
	result, _ := m.Add(interest)
	return result
}

// MonthlyPayment calculates the monthly payment for a loan (amortization).
// annualRate is the annual interest rate as a percentage
// months is the loan term in months
func (m *Money) MonthlyPayment(annualRate float64, months int) *Money {
	if m == nil || m.m == nil || months <= 0 {
		return Zero(USD)
	}

	if annualRate == 0 {
		// No interest - just divide principal by months
		parts, err := m.Split(months)
		if err != nil || len(parts) == 0 {
			return Zero(m.Currency())
		}
		return parts[0]
	}

	// M = P * [r(1+r)^n] / [(1+r)^n - 1]
	// Where: P = principal, r = monthly rate, n = number of months
	principal := m.ToDecimal()
	monthlyRate := decimal.NewFromFloat(annualRate).Div(decimal.NewFromInt(100)).Div(decimal.NewFromInt(12))
	n := int64(months)

	// Calculate (1+r)^n
	onePlusR := decimal.NewFromInt(1).Add(monthlyRate)
	power := decimal.NewFromInt(1)
	for i := int64(0); i < n; i++ {
		power = power.Mul(onePlusR)
	}

	// Calculate numerator: r * (1+r)^n
	numerator := monthlyRate.Mul(power)

	// Calculate denominator: (1+r)^n - 1
	denominator := power.Sub(decimal.NewFromInt(1))

	// Monthly payment
	payment := principal.Mul(numerator).Div(denominator)

	return NewFromDecimal(payment, m.Currency())
}

// TotalLoanCost calculates the total amount paid over the life of a loan.
// annualRate is the annual interest rate as a percentage
// months is the loan term in months
func (m *Money) TotalLoanCost(annualRate float64, months int) *Money {
	monthlyPayment := m.MonthlyPayment(annualRate, months)
	return monthlyPayment.Multiply(int64(months))
}

// LoanInterest calculates the total interest paid over the life of a loan.
func (m *Money) LoanInterest(annualRate float64, months int) *Money {
	totalCost := m.TotalLoanCost(annualRate, months)
	interest, _ := totalCost.Subtract(m)
	return interest
}

// Discount calculates the discounted price.
// discountPercent is the discount percentage (e.g., 20 for 20% off)
func (m *Money) Discount(discountPercent float64) *Money {
	return m.SubtractPercentage(discountPercent)
}

// MarkUp calculates the marked-up price.
// markupPercent is the markup percentage (e.g., 50 for 50% markup)
func (m *Money) MarkUp(markupPercent float64) *Money {
	return m.AddPercentage(markupPercent)
}

// Margin calculates the selling price needed to achieve a desired profit margin.
// marginPercent is the desired margin (e.g., 30 for 30% margin)
// Margin is calculated as: (selling price - cost) / selling price
func (m *Money) Margin(marginPercent float64) *Money {
	if m == nil || m.m == nil || marginPercent >= 100 {
		return Zero(USD)
	}

	// selling_price = cost / (1 - margin/100)
	cost := m.ToDecimal()
	margin := decimal.NewFromFloat(marginPercent).Div(decimal.NewFromInt(100))
	divisor := decimal.NewFromInt(1).Sub(margin)
	sellingPrice := cost.Div(divisor)

	return NewFromDecimal(sellingPrice, m.Currency())
}

// Round rounds the amount to the nearest specified unit.
// For example, Round(5) rounds to the nearest 5 cents.
func (m *Money) Round(unit int64) *Money {
	if m == nil || m.m == nil || unit <= 0 {
		return m
	}

	amount := m.Amount()
	remainder := amount % unit
	if remainder == 0 {
		return m
	}

	if remainder >= unit/2 {
		amount = amount + (unit - remainder)
	} else {
		amount = amount - remainder
	}

	return New(amount, m.Currency())
}

// RoundUp rounds up to the nearest specified unit.
func (m *Money) RoundUp(unit int64) *Money {
	if m == nil || m.m == nil || unit <= 0 {
		return m
	}

	amount := m.Amount()
	remainder := amount % unit
	if remainder == 0 {
		return m
	}

	return New(amount+(unit-remainder), m.Currency())
}

// RoundDown rounds down to the nearest specified unit.
func (m *Money) RoundDown(unit int64) *Money {
	if m == nil || m.m == nil || unit <= 0 {
		return m
	}

	amount := m.Amount()
	remainder := amount % unit
	if remainder == 0 {
		return m
	}

	return New(amount-remainder, m.Currency())
}

// MultiplyDecimal multiplies by a decimal factor for precise calculations.
func (m *Money) MultiplyDecimal(factor decimal.Decimal) *Money {
	if m == nil || m.m == nil {
		return Zero(USD)
	}

	d := m.ToDecimal()
	result := d.Mul(factor)

	return NewFromDecimal(result, m.Currency())
}

// DivideDecimal divides by a decimal divisor for precise calculations.
func (m *Money) DivideDecimal(divisor decimal.Decimal) *Money {
	if m == nil || m.m == nil || divisor.IsZero() {
		return Zero(USD)
	}

	d := m.ToDecimal()
	result := d.Div(divisor)

	return NewFromDecimal(result, m.Currency())
}

// PercentageOf calculates what percentage this amount is of another amount.
// Returns the percentage as a decimal.Decimal (e.g., 25.5 for 25.5%)
func (m *Money) PercentageOf(total *Money) decimal.Decimal {
	if m == nil || m.m == nil || total == nil || total.m == nil || total.IsZero() {
		return decimal.Zero
	}

	part := m.ToDecimal()
	whole := total.ToDecimal()

	return part.Div(whole).Mul(decimal.NewFromInt(100))
}
