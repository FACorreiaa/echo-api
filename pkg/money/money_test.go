package money

import (
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Basic Money Operations Tests
// ============================================================================

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		cents    int64
		currency string
		want     int64
	}{
		{"positive cents", 1234, USD, 1234},
		{"zero", 0, USD, 0},
		{"negative cents", -5000, USD, -5000},
		{"large amount", 999999999, USD, 999999999},
		{"euro", 1000, EUR, 1000},
		{"yen (no decimals)", 10000, JPY, 10000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(tt.cents, tt.currency)
			assert.Equal(t, tt.want, m.Amount())
			assert.Equal(t, tt.currency, m.Currency())
		})
	}
}

func TestNewFromFloat(t *testing.T) {
	tests := []struct {
		name     string
		amount   float64
		currency string
		want     int64
	}{
		{"simple decimal", 12.34, USD, 1234},
		{"whole number", 100.00, USD, 10000},
		{"zero", 0.0, USD, 0},
		{"negative", -50.99, USD, -5099},
		{"small amount", 0.01, USD, 1},
		{"rounding", 12.345, USD, 1235}, // Should round to nearest cent
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewFromFloat(tt.amount, tt.currency)
			assert.Equal(t, tt.want, m.Amount())
		})
	}
}

func TestNewFromDecimal(t *testing.T) {
	tests := []struct {
		name     string
		amount   string
		currency string
		want     int64
	}{
		{"precise decimal", "123.45", USD, 12345},
		{"many decimals", "99.999", USD, 10000}, // Rounds up
		{"whole number", "500", USD, 50000},
		{"negative", "-25.50", USD, -2550},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, _ := decimal.NewFromString(tt.amount)
			m := NewFromDecimal(d, tt.currency)
			assert.Equal(t, tt.want, m.Amount())
		})
	}
}

func TestNewFromString(t *testing.T) {
	tests := []struct {
		name     string
		amount   string
		currency string
		european bool
		want     int64
		wantErr  bool
	}{
		{"simple", "123.45", USD, false, 12345, false},
		{"with comma thousands", "1,234.56", USD, false, 123456, false},
		{"european format", "1.234,56", EUR, true, 123456, false},
		{"with dollar sign", "$99.99", USD, false, 9999, false},
		{"with euro sign", "€50,00", EUR, true, 5000, false},
		{"with spaces", "  100.00  ", USD, false, 10000, false},
		{"invalid", "abc", USD, false, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewFromString(tt.amount, tt.currency, tt.european)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, m.Amount())
		})
	}
}

func TestZero(t *testing.T) {
	m := Zero(USD)
	assert.True(t, m.IsZero())
	assert.Equal(t, int64(0), m.Amount())
	assert.Equal(t, USD, m.Currency())
}

// ============================================================================
// Arithmetic Operations Tests
// ============================================================================

func TestAdd(t *testing.T) {
	tests := []struct {
		name    string
		a       *Money
		b       *Money
		want    int64
		wantErr bool
	}{
		{"positive + positive", New(1000, USD), New(500, USD), 1500, false},
		{"positive + negative", New(1000, USD), New(-300, USD), 700, false},
		{"negative + negative", New(-100, USD), New(-200, USD), -300, false},
		{"with zero", New(1000, USD), Zero(USD), 1000, false},
		{"nil + value", nil, New(500, USD), 500, false},
		{"different currencies", New(100, USD), New(100, EUR), 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.a.Add(tt.b)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, result.Amount())
		})
	}
}

func TestSubtract(t *testing.T) {
	tests := []struct {
		name    string
		a       *Money
		b       *Money
		want    int64
		wantErr bool
	}{
		{"positive - positive", New(1000, USD), New(300, USD), 700, false},
		{"positive - negative", New(1000, USD), New(-300, USD), 1300, false},
		{"result negative", New(100, USD), New(300, USD), -200, false},
		{"with zero", New(1000, USD), Zero(USD), 1000, false},
		{"different currencies", New(100, USD), New(100, EUR), 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.a.Subtract(tt.b)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, result.Amount())
		})
	}
}

func TestMultiply(t *testing.T) {
	tests := []struct {
		name   string
		m      *Money
		factor int64
		want   int64
	}{
		{"positive * positive", New(100, USD), 5, 500},
		{"positive * negative", New(100, USD), -3, -300},
		{"positive * zero", New(100, USD), 0, 0},
		{"negative * positive", New(-100, USD), 4, -400},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.m.Multiply(tt.factor)
			assert.Equal(t, tt.want, result.Amount())
		})
	}
}

// ============================================================================
// Comparison Tests
// ============================================================================

func TestComparisons(t *testing.T) {
	a := New(1000, USD)
	b := New(500, USD)
	c := New(1000, USD)

	assert.True(t, a.GreaterThan(b))
	assert.False(t, b.GreaterThan(a))
	assert.True(t, b.LessThan(a))
	assert.False(t, a.LessThan(b))
	assert.True(t, a.Equals(c))
	assert.False(t, a.Equals(b))
}

func TestCompare(t *testing.T) {
	tests := []struct {
		name string
		a    *Money
		b    *Money
		want int
	}{
		{"greater", New(1000, USD), New(500, USD), 1},
		{"less", New(500, USD), New(1000, USD), -1},
		{"equal", New(1000, USD), New(1000, USD), 0},
		{"nil vs positive", nil, New(100, USD), -1},
		{"nil vs nil", nil, nil, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.a.Compare(tt.b))
		})
	}
}

// ============================================================================
// Precision Calculation Tests
// ============================================================================

func TestPercentage(t *testing.T) {
	tests := []struct {
		name    string
		amount  int64
		percent float64
		want    int64
	}{
		{"10% of $100", 10000, 10, 1000},
		{"25% of $200", 20000, 25, 5000},
		{"50% of $50", 5000, 50, 2500},
		{"8.25% of $100 (tax)", 10000, 8.25, 825},
		{"15.5% of $1000", 100000, 15.5, 15500},
		{"100% of $50", 5000, 100, 5000},
		{"0% of $100", 10000, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(tt.amount, USD)
			result := m.Percentage(tt.percent)
			assert.Equal(t, tt.want, result.Amount())
		})
	}
}

func TestTax(t *testing.T) {
	// $100.00 with 8.25% tax
	base := New(10000, USD)
	tax := base.Tax(8.25)
	withTax := base.WithTax(8.25)

	assert.Equal(t, int64(825), tax.Amount())       // $8.25 tax
	assert.Equal(t, int64(10825), withTax.Amount()) // $108.25 total

	// Extract tax from tax-inclusive amount
	extractedTax := withTax.ExtractTax(8.25)
	extractedBase := withTax.BaseFromTaxInclusive(8.25)

	// Should get back approximately the original values
	assert.InDelta(t, 825, extractedTax.Amount(), 1)
	assert.InDelta(t, 10000, extractedBase.Amount(), 1)
}

func TestSimpleInterest(t *testing.T) {
	// $10,000 at 5% for 2 years = $1,000 interest
	principal := New(1000000, USD) // $10,000
	interest := principal.SimpleInterest(5, 2)

	assert.Equal(t, int64(100000), interest.Amount()) // $1,000

	// With interest
	withInterest := principal.WithSimpleInterest(5, 2)
	assert.Equal(t, int64(1100000), withInterest.Amount()) // $11,000
}

func TestCompoundInterest(t *testing.T) {
	// $10,000 at 5% compounded monthly for 1 year
	principal := New(1000000, USD) // $10,000
	interest := principal.CompoundInterest(5, 1, 12)

	// Expected: ~$511.62 (5.1162% effective rate)
	// Allow some tolerance for decimal precision
	assert.InDelta(t, 51162, interest.Amount(), 100)
}

func TestMonthlyPayment(t *testing.T) {
	// $200,000 mortgage at 6% for 30 years (360 months)
	principal := New(20000000, USD) // $200,000
	monthly := principal.MonthlyPayment(6, 360)

	// Expected: ~$1,199.10 per month
	assert.InDelta(t, 119910, monthly.Amount(), 100)

	// Total cost and interest
	totalCost := principal.TotalLoanCost(6, 360)
	totalInterest := principal.LoanInterest(6, 360)

	// Total payments: ~$431,676
	assert.InDelta(t, 43167600, totalCost.Amount(), 50000)
	// Total interest: ~$231,676
	assert.InDelta(t, 23167600, totalInterest.Amount(), 50000)
}

func TestMonthlyPaymentZeroInterest(t *testing.T) {
	// $12,000 loan at 0% for 12 months = $1,000/month
	principal := New(1200000, USD)
	monthly := principal.MonthlyPayment(0, 12)

	assert.Equal(t, int64(100000), monthly.Amount())
}

func TestDiscount(t *testing.T) {
	// $100 with 20% discount = $80
	original := New(10000, USD)
	discounted := original.Discount(20)

	assert.Equal(t, int64(8000), discounted.Amount())
}

func TestMarkUp(t *testing.T) {
	// $50 with 100% markup = $100
	cost := New(5000, USD)
	selling := cost.MarkUp(100)

	assert.Equal(t, int64(10000), selling.Amount())
}

func TestMargin(t *testing.T) {
	// $70 cost with 30% margin = $100 selling price
	// margin = (selling - cost) / selling
	// 0.30 = (selling - 70) / selling
	// 0.30 * selling = selling - 70
	// 70 = selling - 0.30 * selling
	// 70 = 0.70 * selling
	// selling = 100
	cost := New(7000, USD)
	selling := cost.Margin(30)

	assert.Equal(t, int64(10000), selling.Amount())
}

func TestPercentageOf(t *testing.T) {
	part := New(2500, USD)   // $25
	whole := New(10000, USD) // $100

	pct := part.PercentageOf(whole)
	assert.True(t, pct.Equal(decimal.NewFromInt(25)))
}

// ============================================================================
// Rounding Tests
// ============================================================================

func TestRound(t *testing.T) {
	tests := []struct {
		name   string
		amount int64
		unit   int64
		want   int64
	}{
		{"round up", 123, 5, 125},   // 123 is 3 away from 125, 2 from 120 -> round up
		{"round down", 121, 5, 120}, // 121 is 4 away from 125, 1 from 120 -> round down
		{"exact", 125, 5, 125},
		{"round to 10", 1234, 10, 1230},   // 1234 is 4 away from 1230 -> round down
		{"round to 100", 1250, 100, 1300}, // 1250 is exactly halfway -> round up
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(tt.amount, USD)
			result := m.Round(tt.unit)
			assert.Equal(t, tt.want, result.Amount())
		})
	}
}

func TestRoundUp(t *testing.T) {
	m := New(123, USD)
	assert.Equal(t, int64(125), m.RoundUp(5).Amount())
	assert.Equal(t, int64(125), m.RoundUp(25).Amount())
	assert.Equal(t, int64(200), m.RoundUp(100).Amount())
}

func TestRoundDown(t *testing.T) {
	m := New(127, USD)
	assert.Equal(t, int64(125), m.RoundDown(5).Amount())
	assert.Equal(t, int64(125), m.RoundDown(25).Amount())
	assert.Equal(t, int64(100), m.RoundDown(100).Amount())
}

// ============================================================================
// Split and Allocate Tests
// ============================================================================

func TestSplit(t *testing.T) {
	// $100 split 3 ways
	m := New(10000, USD)
	parts, err := m.Split(3)

	require.NoError(t, err)
	require.Len(t, parts, 3)

	// Should sum to original amount (no money lost)
	total := int64(0)
	for _, p := range parts {
		total += p.Amount()
	}
	assert.Equal(t, int64(10000), total)

	// Distribution: 3334 + 3333 + 3333 = 10000
	assert.Equal(t, int64(3334), parts[0].Amount())
	assert.Equal(t, int64(3333), parts[1].Amount())
	assert.Equal(t, int64(3333), parts[2].Amount())
}

func TestAllocate(t *testing.T) {
	// $100 allocated 50:30:20
	m := New(10000, USD)
	parts, err := m.Allocate([]int{50, 30, 20})

	require.NoError(t, err)
	require.Len(t, parts, 3)

	// Should sum to original amount
	total := int64(0)
	for _, p := range parts {
		total += p.Amount()
	}
	assert.Equal(t, int64(10000), total)

	// Check proportions (allowing for rounding)
	assert.InDelta(t, 5000, parts[0].Amount(), 1)
	assert.InDelta(t, 3000, parts[1].Amount(), 1)
	assert.InDelta(t, 2000, parts[2].Amount(), 1)
}

// ============================================================================
// Currency Conversion Tests
// ============================================================================

func TestConvert(t *testing.T) {
	// $100 USD to EUR at rate 0.85
	usd := New(10000, USD)
	rate := decimal.NewFromFloat(0.85)
	eur := usd.Convert(EUR, rate)

	assert.Equal(t, int64(8500), eur.Amount())
	assert.Equal(t, EUR, eur.Currency())
}

func TestSameCurrency(t *testing.T) {
	a := New(100, USD)
	b := New(200, USD)
	c := New(100, EUR)

	assert.True(t, a.SameCurrency(b))
	assert.False(t, a.SameCurrency(c))
}

// ============================================================================
// JSON Marshaling Tests
// ============================================================================

func TestJSONMarshal(t *testing.T) {
	m := New(12345, USD)
	data, err := json.Marshal(m)

	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, float64(12345), result["amount"])
	assert.Equal(t, "USD", result["currency"])
	assert.Contains(t, result["display"], "$")
}

func TestJSONUnmarshal(t *testing.T) {
	data := []byte(`{"amount": 9999, "currency": "EUR"}`)

	var m Money
	err := json.Unmarshal(data, &m)

	require.NoError(t, err)
	assert.Equal(t, int64(9999), m.Amount())
	assert.Equal(t, EUR, m.Currency())
}

// ============================================================================
// Display and Formatting Tests
// ============================================================================

func TestDisplay(t *testing.T) {
	tests := []struct {
		name     string
		cents    int64
		currency string
		contains string
	}{
		{"USD", 12345, USD, "$"},
		{"EUR", 12345, EUR, "€"},
		{"negative", -5000, USD, "-"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(tt.cents, tt.currency)
			assert.Contains(t, m.Display(), tt.contains)
		})
	}
}

func TestString(t *testing.T) {
	m := New(12345, USD)
	s := m.String()
	assert.Equal(t, "123.45", s)
}

func TestToDecimal(t *testing.T) {
	m := New(12345, USD)
	d := m.ToDecimal()

	expected, _ := decimal.NewFromString("123.45")
	assert.True(t, d.Equal(expected))
}

func TestToFloat64(t *testing.T) {
	m := New(12345, USD)
	f := m.ToFloat64()

	assert.InDelta(t, 123.45, f, 0.001)
}

// ============================================================================
// Edge Cases and Nil Safety Tests
// ============================================================================

func TestNilSafety(t *testing.T) {
	var m *Money

	// All these should not panic
	assert.Equal(t, int64(0), m.Amount())
	assert.Equal(t, "", m.Currency())
	assert.Equal(t, "", m.CurrencySymbol())
	assert.True(t, m.IsZero())
	assert.False(t, m.IsPositive())
	assert.False(t, m.IsNegative())
	assert.Equal(t, "$0.00", m.Display())
	assert.Equal(t, "0.00", m.String())
	assert.True(t, m.ToDecimal().IsZero())
	assert.Equal(t, int64(0), m.Abs().Amount())
	assert.Equal(t, int64(0), m.Negate().Amount())
	assert.Equal(t, int64(0), m.Percentage(10).Amount())
	assert.Equal(t, int64(0), m.Multiply(5).Amount())
}

// ============================================================================
// Test Data Generator Tests
// ============================================================================

func TestTestDataGenerator(t *testing.T) {
	gen := NewTestDataGeneratorWithSeed(42)

	t.Run("generates transaction", func(t *testing.T) {
		tx := gen.Transaction(USD)
		assert.NotEmpty(t, tx.ID)
		assert.NotEmpty(t, tx.Description)
		assert.NotNil(t, tx.Amount)
		assert.NotEmpty(t, tx.Category)
	})

	t.Run("generates multiple transactions", func(t *testing.T) {
		txs := gen.Transactions(USD, 10)
		assert.Len(t, txs, 10)
	})

	t.Run("generates expense", func(t *testing.T) {
		tx := gen.ExpenseTransaction(USD)
		assert.True(t, tx.IsExpense)
		assert.True(t, tx.Amount.IsNegative())
	})

	t.Run("generates income", func(t *testing.T) {
		tx := gen.IncomeTransaction(USD)
		assert.False(t, tx.IsExpense)
		assert.True(t, tx.Amount.IsPositive())
	})

	t.Run("generates budget", func(t *testing.T) {
		budget := gen.Budget(USD)
		assert.NotEmpty(t, budget.ID)
		assert.NotEmpty(t, budget.Name)
		assert.NotNil(t, budget.Amount)
	})

	t.Run("generates account", func(t *testing.T) {
		account := gen.Account(USD)
		assert.NotEmpty(t, account.ID)
		assert.NotEmpty(t, account.Type)
		assert.NotNil(t, account.Balance)
	})

	t.Run("generates loan", func(t *testing.T) {
		loan := gen.Loan(USD)
		assert.NotEmpty(t, loan.ID)
		assert.NotNil(t, loan.Principal)
		assert.NotNil(t, loan.MonthlyPayment)
		assert.Greater(t, loan.InterestRate, 0.0)
	})

	t.Run("generates investment", func(t *testing.T) {
		inv := gen.Investment(USD)
		assert.NotEmpty(t, inv.ID)
		assert.NotEmpty(t, inv.Type)
		assert.NotNil(t, inv.Principal)
		assert.NotNil(t, inv.CurrentValue)
	})

	t.Run("generates monthly set", func(t *testing.T) {
		txs := gen.MonthlyTransactionSet(USD)
		assert.Greater(t, len(txs), 20)
	})
}

// ============================================================================
// Benchmarks
// ============================================================================

func BenchmarkNew(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = New(12345, USD)
	}
}

func BenchmarkNewFromFloat(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewFromFloat(123.45, USD)
	}
}

func BenchmarkNewFromDecimal(b *testing.B) {
	d := decimal.NewFromFloat(123.45)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewFromDecimal(d, USD)
	}
}

func BenchmarkAdd(b *testing.B) {
	a := New(10000, USD)
	c := New(5000, USD)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = a.Add(c)
	}
}

func BenchmarkPercentage(b *testing.B) {
	m := New(10000, USD)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.Percentage(8.25)
	}
}

func BenchmarkTax(b *testing.B) {
	m := New(10000, USD)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.WithTax(8.25)
	}
}

func BenchmarkSimpleInterest(b *testing.B) {
	principal := New(1000000, USD)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = principal.SimpleInterest(5, 2)
	}
}

func BenchmarkCompoundInterest(b *testing.B) {
	principal := New(1000000, USD)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = principal.CompoundInterest(5, 1, 12)
	}
}

func BenchmarkMonthlyPayment(b *testing.B) {
	principal := New(20000000, USD)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = principal.MonthlyPayment(6, 360)
	}
}

func BenchmarkSplit(b *testing.B) {
	m := New(10000, USD)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = m.Split(3)
	}
}

func BenchmarkAllocate(b *testing.B) {
	m := New(10000, USD)
	ratios := []int{50, 30, 20}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = m.Allocate(ratios)
	}
}

func BenchmarkConvert(b *testing.B) {
	m := New(10000, USD)
	rate := decimal.NewFromFloat(0.85)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.Convert(EUR, rate)
	}
}

func BenchmarkJSONMarshal(b *testing.B) {
	m := New(12345, USD)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(m)
	}
}

func BenchmarkJSONUnmarshal(b *testing.B) {
	data := []byte(`{"amount": 12345, "currency": "USD"}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var m Money
		_ = json.Unmarshal(data, &m)
	}
}

func BenchmarkTestDataGenerator_Transaction(b *testing.B) {
	gen := NewTestDataGeneratorWithSeed(42)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = gen.Transaction(USD)
	}
}

func BenchmarkTestDataGenerator_MonthlySet(b *testing.B) {
	gen := NewTestDataGeneratorWithSeed(42)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = gen.MonthlyTransactionSet(USD)
	}
}
