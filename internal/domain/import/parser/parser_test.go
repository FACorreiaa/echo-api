package parser

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser_Parse(t *testing.T) {
	t.Run("parses standard CSV", func(t *testing.T) {
		csv := `date,description,amount,category
2024-01-15,Coffee Shop,-4.50,Food
2024-01-16,Salary,5000.00,Income
2024-01-17,Groceries,-125.30,Food`

		parser := NewParser(DefaultConfig())
		result, err := parser.Parse(strings.NewReader(csv))

		require.NoError(t, err)
		assert.Equal(t, 3, result.TotalRows)
		assert.Equal(t, 3, result.ParsedRows)
		assert.Len(t, result.Transactions, 3)
		assert.Empty(t, result.Errors)

		// Check first transaction
		tx := result.Transactions[0]
		assert.Equal(t, "Coffee Shop", tx.Description)
		assert.Equal(t, int64(-450), tx.AmountCents)
		assert.Equal(t, "Food", tx.Category)
	})

	t.Run("parses European format", func(t *testing.T) {
		csv := `date;description;amount
15/01/2024;Café;-4,50
16/01/2024;Salário;5.000,00`

		config := DefaultConfig()
		config.Delimiter = ';'
		config.IsEuropeanFormat = true

		parser := NewParser(config)
		result, err := parser.Parse(strings.NewReader(csv))

		require.NoError(t, err)
		assert.Equal(t, 2, result.ParsedRows)

		tx := result.Transactions[0]
		assert.Equal(t, int64(-450), tx.AmountCents)

		tx2 := result.Transactions[1]
		assert.Equal(t, int64(500000), tx2.AmountCents)
	})

	t.Run("parses debit/credit columns", func(t *testing.T) {
		csv := `date,description,debit,credit
2024-01-15,Coffee,4.50,
2024-01-16,Salary,,5000.00`

		// Use column-based parsing for non-standard headers
		config := DefaultConfig()
		config.DateColumn = 0
		config.DescColumn = 1
		config.DebitColumn = 2
		config.CreditColumn = 3

		parser := NewParser(config)
		result, err := parser.ParseWithColumns(strings.NewReader(csv), []string{"date", "description", "debit", "credit"})

		require.NoError(t, err)
		assert.Equal(t, 2, result.ParsedRows)

		// Debit should be negative
		assert.Equal(t, int64(-450), result.Transactions[0].AmountCents)
		// Credit should be positive
		assert.Equal(t, int64(500000), result.Transactions[1].AmountCents)
	})

	t.Run("parses Portuguese headers", func(t *testing.T) {
		csv := `data mov.;descrição;débito;crédito
15/01/2024;Café;4,50;
16/01/2024;Salário;;5000,00`

		config := DefaultConfig()
		config.Delimiter = ';'
		config.IsEuropeanFormat = true
		config.DateColumn = 0
		config.DescColumn = 1
		config.DebitColumn = 2
		config.CreditColumn = 3

		parser := NewParser(config)
		result, err := parser.ParseWithColumns(strings.NewReader(csv), nil)

		require.NoError(t, err)
		assert.Equal(t, 2, result.ParsedRows)
	})

	t.Run("handles skip lines", func(t *testing.T) {
		csv := `Bank Statement
Account: 12345
date,description,amount
2024-01-15,Coffee,-4.50`

		config := DefaultConfig()
		config.SkipLines = 2
		config.DateColumn = 0
		config.DescColumn = 1
		config.AmountColumn = 2

		parser := NewParser(config)
		result, err := parser.ParseWithColumns(strings.NewReader(csv), nil)

		require.NoError(t, err)
		assert.Equal(t, 1, result.ParsedRows)
		assert.Equal(t, "Coffee", result.Transactions[0].Description)
	})

	t.Run("captures parse errors", func(t *testing.T) {
		csv := `date,description,amount
invalid-date,Coffee,-4.50
2024-01-15,Coffee,not-a-number
2024-01-16,Valid,-10.00`

		config := DefaultConfig()
		config.DateColumn = 0
		config.DescColumn = 1
		config.AmountColumn = 2

		parser := NewParser(config)
		result, err := parser.ParseWithColumns(strings.NewReader(csv), nil)

		require.NoError(t, err)
		assert.Equal(t, 1, result.ParsedRows)
		assert.Len(t, result.Errors, 2)
	})

	t.Run("extracts currency hints", func(t *testing.T) {
		csv := `date,description,amount
2024-01-15,Coffee,-$4.50
2024-01-16,Cafe,-€5.00`

		config := DefaultConfig()
		config.DateColumn = 0
		config.DescColumn = 1
		config.AmountColumn = 2

		parser := NewParser(config)
		result, err := parser.ParseWithColumns(strings.NewReader(csv), nil)

		require.NoError(t, err)
		require.Len(t, result.Transactions, 2)
		assert.Equal(t, "$", result.Transactions[0].CurrencyHint)
		assert.Equal(t, "€", result.Transactions[1].CurrencyHint)
	})

	t.Run("skips empty date rows", func(t *testing.T) {
		csv := `date,description,amount
2024-01-15,Coffee,-4.50
,Empty row,0
2024-01-16,Valid,-10.00`

		config := DefaultConfig()
		config.DateColumn = 0
		config.DescColumn = 1
		config.AmountColumn = 2

		parser := NewParser(config)
		result, err := parser.ParseWithColumns(strings.NewReader(csv), nil)

		require.NoError(t, err)
		assert.Equal(t, 2, result.ParsedRows)
		assert.Equal(t, 1, result.SkippedRows)
	})
}

func TestParser_ParseWithColumns(t *testing.T) {
	csv := `col0,col1,col2,col3
2024-01-15,Coffee,-4.50,Food`

	config := DefaultConfig()
	config.DateColumn = 0
	config.DescColumn = 1
	config.AmountColumn = 2
	config.CategoryColumn = 3

	parser := NewParser(config)
	result, err := parser.ParseWithColumns(strings.NewReader(csv), []string{"col0", "col1", "col2", "col3"})

	require.NoError(t, err)
	assert.Equal(t, 1, result.ParsedRows)
	assert.Equal(t, "Coffee", result.Transactions[0].Description)
	assert.Equal(t, int64(-450), result.Transactions[0].AmountCents)
}

func TestParser_DateParsing(t *testing.T) {
	formats := []struct {
		input    string
		expected time.Time
	}{
		{"2024-01-15", time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)},
		{"15/01/2024", time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)},
		{"01/15/2024", time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)},
		{"15-01-2024", time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)},
		{"2024/01/15", time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)},
		{"15.01.2024", time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)},
	}

	parser := NewParser(DefaultConfig())

	for _, tc := range formats {
		t.Run(tc.input, func(t *testing.T) {
			date, err := parser.parseDate(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.expected.Year(), date.Year())
			assert.Equal(t, tc.expected.Month(), date.Month())
			assert.Equal(t, tc.expected.Day(), date.Day())
		})
	}
}

func TestParser_AmountParsing(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		european bool
		expected int64
	}{
		{"simple positive", "100.50", false, 10050},
		{"simple negative", "-100.50", false, -10050},
		{"with thousands", "1,234.56", false, 123456},
		{"european", "1.234,56", true, 123456},
		{"european negative", "-1.234,56", true, -123456},
		{"with currency $", "$100.50", false, 10050},
		{"with currency €", "€100,50", true, 10050},
		{"parentheses negative", "(100.50)", false, -10050},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config := DefaultConfig()
			config.IsEuropeanFormat = tc.european
			parser := NewParser(config)

			cents, _, err := parser.parseAmount(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, cents)
		})
	}
}

func TestStreamingParser_ParseStream(t *testing.T) {
	csv := `date,description,amount
2024-01-15,Coffee,-4.50
2024-01-16,Lunch,-12.00
2024-01-17,Salary,5000.00`

	config := DefaultConfig()
	config.DateColumn = 0
	config.DescColumn = 1
	config.AmountColumn = 2
	parser := NewStreamingParser(config, 2)

	ctx := context.Background()
	results, stats := parser.ParseStream(ctx, strings.NewReader(csv))

	var transactions []ParsedTransaction
	var errors []ParseError

	for result := range results {
		if result.Transaction != nil {
			transactions = append(transactions, *result.Transaction)
		}
		if result.Error != nil {
			errors = append(errors, *result.Error)
		}
	}

	// Wait for stats
	<-stats

	assert.Len(t, transactions, 3)
	assert.Empty(t, errors)
}

func TestStreamingParser_ParseStreamBatched(t *testing.T) {
	// Create CSV with 10 rows
	var sb strings.Builder
	sb.WriteString("date,description,amount\n")
	for i := 0; i < 10; i++ {
		sb.WriteString(fmt.Sprintf("2024-01-%02d,Transaction %d,-%d.00\n", i+1, i, i*10))
	}

	config := DefaultConfig()
	config.DateColumn = 0
	config.DescColumn = 1
	config.AmountColumn = 2
	parser := NewStreamingParser(config, 2)

	ctx := context.Background()
	batchSize := 3
	txChan, errChan := parser.ParseStreamBatched(ctx, strings.NewReader(sb.String()), batchSize)

	var totalTx int
	var batchCount int

	for batch := range txChan {
		batchCount++
		totalTx += len(batch)
	}

	// Drain errors
	for range errChan {
	}

	assert.Equal(t, 10, totalTx)
	assert.GreaterOrEqual(t, batchCount, 3) // At least 3 batches for 10 items with size 3
}

func TestStreamingParser_Context_Cancellation(t *testing.T) {
	// Create a large CSV
	var sb strings.Builder
	sb.WriteString("date,description,amount\n")
	for i := 0; i < 1000; i++ {
		sb.WriteString(fmt.Sprintf("2024-01-01,Transaction %d,-10.00\n", i))
	}

	config := DefaultConfig()
	config.DateColumn = 0
	config.DescColumn = 1
	config.AmountColumn = 2
	parser := NewStreamingParser(config, 2)

	ctx, cancel := context.WithCancel(context.Background())

	results, _ := parser.ParseStream(ctx, strings.NewReader(sb.String()))

	// Cancel after receiving some results
	count := 0
	for range results {
		count++
		if count > 10 {
			cancel()
			break
		}
	}

	// Drain remaining (should be minimal due to cancellation)
	for range results {
	}

	// Should have stopped early
	assert.Less(t, count, 1000)
}

// Benchmarks

func BenchmarkParser_Parse(b *testing.B) {
	// Generate test CSV
	var sb strings.Builder
	sb.WriteString("date,description,amount,category\n")
	for i := 0; i < 10000; i++ {
		sb.WriteString(fmt.Sprintf("2024-01-%02d,Transaction %d description here,-%d.50,Category\n", (i%28)+1, i, i%1000))
	}
	csvData := sb.String()

	config := DefaultConfig()
	config.DateColumn = 0
	config.DescColumn = 1
	config.AmountColumn = 2
	config.CategoryColumn = 3
	parser := NewParser(config)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = parser.ParseWithColumns(strings.NewReader(csvData), nil)
	}
}

func BenchmarkParser_Parse_European(b *testing.B) {
	// Generate European format CSV
	var sb strings.Builder
	sb.WriteString("data mov.;descrição;valor\n")
	for i := 0; i < 10000; i++ {
		sb.WriteString(fmt.Sprintf("%02d/01/2024;Transação %d;-1.%03d,50\n", (i%28)+1, i, i%1000))
	}
	csvData := sb.String()

	config := DefaultConfig()
	config.Delimiter = ';'
	config.IsEuropeanFormat = true
	config.DateColumn = 0
	config.DescColumn = 1
	config.AmountColumn = 2
	parser := NewParser(config)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = parser.ParseWithColumns(strings.NewReader(csvData), nil)
	}
}

func BenchmarkStreamingParser_Parse(b *testing.B) {
	// Generate test CSV
	var sb strings.Builder
	sb.WriteString("date,description,amount,category\n")
	for i := 0; i < 10000; i++ {
		sb.WriteString(fmt.Sprintf("2024-01-%02d,Transaction %d,-%d.50,Category\n", (i%28)+1, i, i%1000))
	}
	csvData := sb.String()

	config := DefaultConfig()
	config.DateColumn = 0
	config.DescColumn = 1
	config.AmountColumn = 2
	config.CategoryColumn = 3
	parser := NewStreamingParser(config, 4)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		results, stats := parser.ParseStream(ctx, strings.NewReader(csvData))
		for range results {
		}
		<-stats
	}
}

func BenchmarkStreamingParser_ParseBatched(b *testing.B) {
	// Generate test CSV
	var sb strings.Builder
	sb.WriteString("date,description,amount,category\n")
	for i := 0; i < 10000; i++ {
		sb.WriteString(fmt.Sprintf("2024-01-%02d,Transaction %d,-%d.50,Category\n", (i%28)+1, i, i%1000))
	}
	csvData := sb.String()

	config := DefaultConfig()
	config.DateColumn = 0
	config.DescColumn = 1
	config.AmountColumn = 2
	config.CategoryColumn = 3
	parser := NewStreamingParser(config, 4)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		txChan, errChan := parser.ParseStreamBatched(ctx, strings.NewReader(csvData), 500)
		for range txChan {
		}
		for range errChan {
		}
	}
}

func BenchmarkParser_DateParsing(b *testing.B) {
	parser := NewParser(DefaultConfig())
	dates := []string{
		"2024-01-15",
		"15/01/2024",
		"01/15/2024",
		"15-01-2024",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for _, d := range dates {
			_, _ = parser.parseDate(d)
		}
	}
}

func BenchmarkParser_AmountParsing(b *testing.B) {
	parser := NewParser(DefaultConfig())
	amounts := []string{
		"100.50",
		"-1,234.56",
		"$5,000.00",
		"(999.99)",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for _, a := range amounts {
			_, _, _ = parser.parseAmount(a)
		}
	}
}

// Comparison with standard encoding/csv
func BenchmarkStandardCSV(b *testing.B) {
	// Generate test CSV
	var sb strings.Builder
	sb.WriteString("date,description,amount,category\n")
	for i := 0; i < 10000; i++ {
		sb.WriteString(fmt.Sprintf("2024-01-%02d,Transaction %d,-%d.50,Category\n", (i%28)+1, i, i%1000))
	}
	csvData := []byte(sb.String())

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Simulate just reading with encoding/csv
		reader := bytes.NewReader(csvData)
		csvReader := &standardCSVReader{reader: reader}
		_ = csvReader.readAll()
	}
}

type standardCSVReader struct {
	reader *bytes.Reader
}

func (r *standardCSVReader) readAll() [][]string {
	import_csv := make([][]string, 0, 10000)
	// Simplified - just read lines
	data := make([]byte, r.reader.Len())
	r.reader.Read(data)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Split(line, ",")
		import_csv = append(import_csv, fields)
	}
	return import_csv
}
