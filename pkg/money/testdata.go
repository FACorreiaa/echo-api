package money

import (
	"math/rand"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// TestDataGenerator generates realistic financial test data using gofakeit.
type TestDataGenerator struct {
	faker *gofakeit.Faker
}

// NewTestDataGenerator creates a new test data generator with a random seed.
func NewTestDataGenerator() *TestDataGenerator {
	return &TestDataGenerator{
		faker: gofakeit.New(0), // Random seed
	}
}

// NewTestDataGeneratorWithSeed creates a generator with a specific seed for reproducibility.
func NewTestDataGeneratorWithSeed(seed int64) *TestDataGenerator {
	return &TestDataGenerator{
		faker: gofakeit.New(seed),
	}
}

// ============================================================================
// Transaction Generation
// ============================================================================

// TestTransaction represents a generated test transaction.
type TestTransaction struct {
	ID          uuid.UUID
	Date        time.Time
	Description string
	Amount      *Money
	Category    string
	Merchant    string
	IsExpense   bool
	Tags        []string
}

// Transaction generates a single random transaction.
func (g *TestDataGenerator) Transaction(currency string) TestTransaction {
	isExpense := g.faker.Bool()
	amount := g.RandomAmount(currency, 1, 50000) // $0.01 to $500.00

	if isExpense {
		amount = amount.Negate()
	}

	return TestTransaction{
		ID:          uuid.New(),
		Date:        g.faker.DateRange(time.Now().AddDate(-1, 0, 0), time.Now()),
		Description: g.TransactionDescription(),
		Amount:      amount,
		Category:    g.Category(),
		Merchant:    g.Merchant(),
		IsExpense:   isExpense,
		Tags:        g.Tags(1, 3),
	}
}

// Transactions generates multiple random transactions.
func (g *TestDataGenerator) Transactions(currency string, count int) []TestTransaction {
	txs := make([]TestTransaction, count)
	for i := 0; i < count; i++ {
		txs[i] = g.Transaction(currency)
	}
	return txs
}

// ExpenseTransaction generates a random expense transaction.
func (g *TestDataGenerator) ExpenseTransaction(currency string) TestTransaction {
	tx := g.Transaction(currency)
	tx.IsExpense = true
	if tx.Amount.IsPositive() {
		tx.Amount = tx.Amount.Negate()
	}
	return tx
}

// IncomeTransaction generates a random income transaction.
func (g *TestDataGenerator) IncomeTransaction(currency string) TestTransaction {
	tx := g.Transaction(currency)
	tx.IsExpense = false
	tx.Amount = g.RandomAmount(currency, 100000, 1000000) // $1,000 to $10,000
	tx.Category = g.IncomeCategory()
	tx.Description = g.IncomeDescription()
	return tx
}

// ============================================================================
// Money Generation
// ============================================================================

// RandomAmount generates a random Money value within a cent range.
func (g *TestDataGenerator) RandomAmount(currency string, minCents, maxCents int64) *Money {
	if minCents > maxCents {
		minCents, maxCents = maxCents, minCents
	}
	cents := g.faker.Int64() % (maxCents - minCents + 1)
	if cents < 0 {
		cents = -cents
	}
	return New(minCents+cents, currency)
}

// RandomAmountRange generates a random Money value within a dollar range.
func (g *TestDataGenerator) RandomAmountRange(currency string, minDollars, maxDollars float64) *Money {
	amount := g.faker.Float64Range(minDollars, maxDollars)
	return NewFromFloat(amount, currency)
}

// SmallPurchase generates a typical small purchase amount ($1-$50).
func (g *TestDataGenerator) SmallPurchase(currency string) *Money {
	return g.RandomAmountRange(currency, 1, 50)
}

// MediumPurchase generates a typical medium purchase amount ($50-$500).
func (g *TestDataGenerator) MediumPurchase(currency string) *Money {
	return g.RandomAmountRange(currency, 50, 500)
}

// LargePurchase generates a typical large purchase amount ($500-$5000).
func (g *TestDataGenerator) LargePurchase(currency string) *Money {
	return g.RandomAmountRange(currency, 500, 5000)
}

// Salary generates a realistic monthly salary amount.
func (g *TestDataGenerator) Salary(currency string) *Money {
	// Range from $2,000 to $20,000 monthly
	return g.RandomAmountRange(currency, 2000, 20000)
}

// Bill generates a realistic bill amount ($20-$500).
func (g *TestDataGenerator) Bill(currency string) *Money {
	return g.RandomAmountRange(currency, 20, 500)
}

// ============================================================================
// Description and Category Generation
// ============================================================================

var expenseCategories = []string{
	"Food & Dining", "Groceries", "Transportation", "Gas & Fuel",
	"Shopping", "Entertainment", "Bills & Utilities", "Health & Medical",
	"Travel", "Education", "Personal Care", "Home & Garden",
	"Pets", "Gifts & Donations", "Business", "Investments",
}

var incomeCategories = []string{
	"Salary", "Freelance", "Investments", "Rental Income",
	"Business Income", "Dividends", "Interest", "Refund",
	"Gift", "Bonus", "Commission", "Side Hustle",
}

var merchants = []string{
	"Amazon", "Walmart", "Target", "Costco", "Starbucks",
	"McDonald's", "Uber", "Lyft", "Netflix", "Spotify",
	"Apple", "Google", "Microsoft", "Whole Foods", "Trader Joe's",
	"CVS Pharmacy", "Walgreens", "Shell", "Chevron", "Exxon",
	"Delta Airlines", "United Airlines", "Marriott", "Hilton",
	"Home Depot", "Lowe's", "Best Buy", "IKEA", "Sephora",
}

var transactionDescriptions = []string{
	"Coffee and pastry",
	"Weekly groceries",
	"Gas station fill-up",
	"Online subscription",
	"Restaurant dinner",
	"Utility bill payment",
	"Office supplies",
	"Gym membership",
	"Phone bill",
	"Insurance premium",
	"Medical copay",
	"Parking fee",
	"Public transit",
	"Book purchase",
	"Movie tickets",
	"Hair salon",
	"Pet supplies",
	"Home maintenance",
	"Clothing purchase",
	"Electronics",
}

var incomeDescriptions = []string{
	"Monthly salary deposit",
	"Freelance payment",
	"Client invoice payment",
	"Dividend payment",
	"Interest income",
	"Tax refund",
	"Bonus payment",
	"Commission earned",
	"Rental income",
	"Side project income",
}

// Category returns a random expense category.
func (g *TestDataGenerator) Category() string {
	return expenseCategories[g.faker.Number(0, len(expenseCategories)-1)]
}

// IncomeCategory returns a random income category.
func (g *TestDataGenerator) IncomeCategory() string {
	return incomeCategories[g.faker.Number(0, len(incomeCategories)-1)]
}

// Merchant returns a random merchant name.
func (g *TestDataGenerator) Merchant() string {
	return merchants[g.faker.Number(0, len(merchants)-1)]
}

// TransactionDescription returns a random transaction description.
func (g *TestDataGenerator) TransactionDescription() string {
	return transactionDescriptions[g.faker.Number(0, len(transactionDescriptions)-1)]
}

// IncomeDescription returns a random income description.
func (g *TestDataGenerator) IncomeDescription() string {
	return incomeDescriptions[g.faker.Number(0, len(incomeDescriptions)-1)]
}

// Tags returns a random set of tags.
func (g *TestDataGenerator) Tags(min, max int) []string {
	allTags := []string{
		"recurring", "essential", "discretionary", "business",
		"personal", "tax-deductible", "reimbursable", "subscription",
		"one-time", "monthly", "annual", "quarterly",
	}

	count := g.faker.Number(min, max)
	if count > len(allTags) {
		count = len(allTags)
	}

	// Shuffle and take first count
	shuffled := make([]string, len(allTags))
	copy(shuffled, allTags)
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	return shuffled[:count]
}

// ============================================================================
// Financial Scenario Generation
// ============================================================================

// Budget represents a generated test budget.
type Budget struct {
	ID         uuid.UUID
	Name       string
	Category   string
	Amount     *Money
	Spent      *Money
	Period     string
	StartDate  time.Time
	EndDate    time.Time
	IsRecuring bool
}

// Budget generates a random budget.
func (g *TestDataGenerator) Budget(currency string) Budget {
	amount := g.RandomAmountRange(currency, 100, 5000)
	spentPercent := g.faker.Float64Range(0, 120)
	spent := amount.Percentage(spentPercent)

	periods := []string{"weekly", "monthly", "quarterly", "yearly"}
	period := periods[g.faker.Number(0, len(periods)-1)]

	startDate := time.Now().AddDate(0, 0, -g.faker.Number(0, 30))

	return Budget{
		ID:         uuid.New(),
		Name:       g.faker.BuzzWord() + " Budget",
		Category:   g.Category(),
		Amount:     amount,
		Spent:      spent,
		Period:     period,
		StartDate:  startDate,
		EndDate:    startDate.AddDate(0, 1, 0),
		IsRecuring: g.faker.Bool(),
	}
}

// Account represents a generated test account.
type Account struct {
	ID          uuid.UUID
	Name        string
	Type        string
	Balance     *Money
	Institution string
	AccountNum  string
	IsActive    bool
}

// Account generates a random account.
func (g *TestDataGenerator) Account(currency string) Account {
	types := []string{"checking", "savings", "credit", "investment", "cash"}
	accountType := types[g.faker.Number(0, len(types)-1)]

	var balance *Money
	switch accountType {
	case "credit":
		// Credit cards typically have negative balance (amount owed)
		balance = g.RandomAmountRange(currency, -10000, 0)
	case "savings":
		balance = g.RandomAmountRange(currency, 1000, 100000)
	case "investment":
		balance = g.RandomAmountRange(currency, 5000, 500000)
	case "cash":
		balance = g.RandomAmountRange(currency, 50, 1000)
	default:
		balance = g.RandomAmountRange(currency, 100, 25000)
	}

	return Account{
		ID:          uuid.New(),
		Name:        g.faker.Company() + " " + accountType,
		Type:        accountType,
		Balance:     balance,
		Institution: g.faker.Company(),
		AccountNum:  g.faker.DigitN(4),
		IsActive:    g.faker.Bool(),
	}
}

// ============================================================================
// Loan and Investment Scenarios
// ============================================================================

// Loan represents a generated test loan.
type Loan struct {
	ID             uuid.UUID
	Name           string
	Principal      *Money
	InterestRate   float64
	TermMonths     int
	MonthlyPayment *Money
	TotalInterest  *Money
	StartDate      time.Time
}

// Loan generates a random loan.
func (g *TestDataGenerator) Loan(currency string) Loan {
	loanTypes := []string{"Mortgage", "Auto Loan", "Personal Loan", "Student Loan", "Business Loan"}
	loanType := loanTypes[g.faker.Number(0, len(loanTypes)-1)]

	var principal *Money
	var termMonths int
	var rate float64

	switch loanType {
	case "Mortgage":
		principal = g.RandomAmountRange(currency, 100000, 1000000)
		termMonths = g.faker.Number(180, 360) // 15-30 years
		rate = g.faker.Float64Range(3.0, 8.0)
	case "Auto Loan":
		principal = g.RandomAmountRange(currency, 15000, 60000)
		termMonths = g.faker.Number(36, 72) // 3-6 years
		rate = g.faker.Float64Range(4.0, 12.0)
	case "Student Loan":
		principal = g.RandomAmountRange(currency, 10000, 200000)
		termMonths = g.faker.Number(120, 240) // 10-20 years
		rate = g.faker.Float64Range(4.0, 8.0)
	default:
		principal = g.RandomAmountRange(currency, 5000, 50000)
		termMonths = g.faker.Number(12, 60) // 1-5 years
		rate = g.faker.Float64Range(6.0, 18.0)
	}

	return Loan{
		ID:             uuid.New(),
		Name:           loanType,
		Principal:      principal,
		InterestRate:   rate,
		TermMonths:     termMonths,
		MonthlyPayment: principal.MonthlyPayment(rate, termMonths),
		TotalInterest:  principal.LoanInterest(rate, termMonths),
		StartDate:      g.faker.DateRange(time.Now().AddDate(-10, 0, 0), time.Now()),
	}
}

// Investment represents a generated test investment.
type Investment struct {
	ID           uuid.UUID
	Name         string
	Type         string
	Principal    *Money
	CurrentValue *Money
	Return       *Money
	ReturnPct    decimal.Decimal
	PurchaseDate time.Time
}

// Investment generates a random investment.
func (g *TestDataGenerator) Investment(currency string) Investment {
	types := []string{"Stock", "ETF", "Mutual Fund", "Bond", "REIT", "Crypto"}
	investmentType := types[g.faker.Number(0, len(types)-1)]

	principal := g.RandomAmountRange(currency, 1000, 100000)
	returnPct := g.faker.Float64Range(-30, 100) // -30% to +100%
	currentValue := principal.AddPercentage(returnPct)
	returnAmount, _ := currentValue.Subtract(principal)

	return Investment{
		ID:           uuid.New(),
		Name:         g.faker.Company() + " " + investmentType,
		Type:         investmentType,
		Principal:    principal,
		CurrentValue: currentValue,
		Return:       returnAmount,
		ReturnPct:    decimal.NewFromFloat(returnPct),
		PurchaseDate: g.faker.DateRange(time.Now().AddDate(-5, 0, 0), time.Now()),
	}
}

// ============================================================================
// Batch Generators for Testing
// ============================================================================

// MonthlyTransactionSet generates a realistic month of transactions.
func (g *TestDataGenerator) MonthlyTransactionSet(currency string) []TestTransaction {
	txs := make([]TestTransaction, 0, 50)

	// Add recurring income (1-2 paychecks)
	paycheckCount := g.faker.Number(1, 2)
	for i := 0; i < paycheckCount; i++ {
		txs = append(txs, g.IncomeTransaction(currency))
	}

	// Add bills (5-10)
	billCount := g.faker.Number(5, 10)
	for i := 0; i < billCount; i++ {
		tx := g.ExpenseTransaction(currency)
		tx.Amount = g.Bill(currency).Negate()
		tx.Category = "Bills & Utilities"
		txs = append(txs, tx)
	}

	// Add random daily expenses (20-40)
	expenseCount := g.faker.Number(20, 40)
	for i := 0; i < expenseCount; i++ {
		txs = append(txs, g.ExpenseTransaction(currency))
	}

	return txs
}

// TaxableTransactionSet generates transactions suitable for tax testing.
func (g *TestDataGenerator) TaxableTransactionSet(currency string, taxRate float64) []TestTransaction {
	txs := make([]TestTransaction, 0, 20)

	for i := 0; i < 20; i++ {
		tx := g.Transaction(currency)
		// Add tax to expense amounts
		if tx.IsExpense && tx.Amount.IsNegative() {
			baseAmount := tx.Amount.Abs()
			withTax := baseAmount.WithTax(taxRate)
			tx.Amount = withTax.Negate()
		}
		txs = append(txs, tx)
	}

	return txs
}
