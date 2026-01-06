package parser

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strings"
	"testing"
	"time"
)

// generateCSVData creates test CSV data with specified row count
func generateCSVData(rows int) []byte {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header
	writer.Write([]string{"Date", "Description", "Amount", "Category"})

	// Write data rows
	for i := 0; i < rows; i++ {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		desc := fmt.Sprintf("Transaction %d at Merchant %d", i, i%100)
		amount := fmt.Sprintf("%.2f", float64(i%10000)/100.0)
		category := fmt.Sprintf("Category %d", i%10)
		writer.Write([]string{date, desc, amount, category})
	}

	writer.Flush()
	return buf.Bytes()
}

// BenchmarkParserComparison compares parsing approaches
func BenchmarkParserComparison(b *testing.B) {
	// Test data sizes
	sizes := []int{100, 1000, 10000}

	for _, size := range sizes {
		csvData := generateCSVData(size)

		b.Run(fmt.Sprintf("StandardCSV_%d_rows", size), func(b *testing.B) {
			b.SetBytes(int64(len(csvData)))
			for i := 0; i < b.N; i++ {
				reader := csv.NewReader(bytes.NewReader(csvData))
				reader.FieldsPerRecord = -1
				reader.LazyQuotes = true

				// Skip header
				reader.Read()

				count := 0
				for {
					_, err := reader.Read()
					if err != nil {
						break
					}
					count++
				}
			}
		})

		b.Run(fmt.Sprintf("NewParser_%d_rows", size), func(b *testing.B) {
			config := ParserConfig{
				DateColumn:   0,
				DescColumn:   1,
				AmountColumn: 2,
				CategoryColumn: 3,
			}
			parser := NewParser(config)
			b.SetBytes(int64(len(csvData)))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, _ = parser.Parse(bytes.NewReader(csvData))
			}
		})

		b.Run(fmt.Sprintf("StreamingParser_%d_rows", size), func(b *testing.B) {
			config := ParserConfig{
				DateColumn:   0,
				DescColumn:   1,
				AmountColumn: 2,
				CategoryColumn: 3,
			}
			b.SetBytes(int64(len(csvData)))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				parser := NewStreamingParser(config, 4)
				ctx := context.Background()
				results, _ := parser.ParseStream(ctx, bytes.NewReader(csvData))

				// Consume all results
				for range results {
				}
			}
		})

		b.Run(fmt.Sprintf("StreamingParserBatched_%d_rows", size), func(b *testing.B) {
			config := ParserConfig{
				DateColumn:   0,
				DescColumn:   1,
				AmountColumn: 2,
				CategoryColumn: 3,
			}
			b.SetBytes(int64(len(csvData)))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				parser := NewStreamingParser(config, 4)
				ctx := context.Background()
				txChan, errChan := parser.ParseStreamBatched(ctx, bytes.NewReader(csvData), 500)

				// Consume all batches
				for range txChan {
				}
				for range errChan {
				}
			}
		})
	}
}

// BenchmarkParserMemory measures memory usage during parsing
func BenchmarkParserMemory(b *testing.B) {
	csvData := generateCSVData(10000)

	b.Run("NewParser_Memory", func(b *testing.B) {
		config := ParserConfig{
			DateColumn:   0,
			DescColumn:   1,
			AmountColumn: 2,
			CategoryColumn: 3,
		}
		parser := NewParser(config)

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			result, _ := parser.Parse(bytes.NewReader(csvData))
			_ = len(result.Transactions)
		}
	})

	b.Run("StreamingParser_Memory", func(b *testing.B) {
		config := ParserConfig{
			DateColumn:   0,
			DescColumn:   1,
			AmountColumn: 2,
			CategoryColumn: 3,
		}

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			parser := NewStreamingParser(config, 4)
			ctx := context.Background()
			results, _ := parser.ParseStream(ctx, bytes.NewReader(csvData))

			count := 0
			for range results {
				count++
			}
		}
	})
}

// BenchmarkParserDateFormats tests date parsing performance with different formats
func BenchmarkParserDateFormats(b *testing.B) {
	formats := []struct {
		name   string
		format string
		sample string
	}{
		{"ISO8601", "2006-01-02", "2024-01-15"},
		{"European", "02/01/2006", "15/01/2024"},
		{"US", "01/02/2006", "01/15/2024"},
		{"EuropeanDash", "02-01-2006", "15-01-2024"},
	}

	for _, f := range formats {
		b.Run(f.name, func(b *testing.B) {
			// Generate data with specific format
			var buf bytes.Buffer
			writer := csv.NewWriter(&buf)
			writer.Write([]string{"Date", "Description", "Amount"})
			for i := 0; i < 1000; i++ {
				writer.Write([]string{f.sample, "Test Transaction", "100.00"})
			}
			writer.Flush()
			csvData := buf.Bytes()

			config := ParserConfig{
				DateColumn:   0,
				DescColumn:   1,
				AmountColumn: 2,
				DateFormat:   f.format,
			}
			parser := NewParser(config)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				parser.Parse(bytes.NewReader(csvData))
			}
		})
	}
}

// BenchmarkParserAmountFormats tests amount parsing with different formats
func BenchmarkParserAmountFormats(b *testing.B) {
	formats := []struct {
		name     string
		european bool
		sample   string
	}{
		{"US_Format", false, "1,234.56"},
		{"European_Format", true, "1.234,56"},
		{"Simple", false, "1234.56"},
		{"Negative_US", false, "-1,234.56"},
		{"Negative_European", true, "-1.234,56"},
	}

	for _, f := range formats {
		b.Run(f.name, func(b *testing.B) {
			var buf bytes.Buffer
			writer := csv.NewWriter(&buf)
			writer.Write([]string{"Date", "Description", "Amount"})
			for i := 0; i < 1000; i++ {
				writer.Write([]string{"2024-01-15", "Test Transaction", f.sample})
			}
			writer.Flush()
			csvData := buf.Bytes()

			config := ParserConfig{
				DateColumn:       0,
				DescColumn:       1,
				AmountColumn:     2,
				IsEuropeanFormat: f.european,
			}
			parser := NewParser(config)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				parser.Parse(bytes.NewReader(csvData))
			}
		})
	}
}

// BenchmarkDebitCreditParsing tests double-entry bookkeeping parsing
func BenchmarkDebitCreditParsing(b *testing.B) {
	// Generate debit/credit CSV
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	writer.Write([]string{"Date", "Description", "Debit", "Credit"})
	for i := 0; i < 1000; i++ {
		if i%2 == 0 {
			writer.Write([]string{"2024-01-15", "Expense", "100.00", ""})
		} else {
			writer.Write([]string{"2024-01-15", "Income", "", "100.00"})
		}
	}
	writer.Flush()
	csvData := buf.Bytes()

	config := ParserConfig{
		DateColumn:  0,
		DescColumn:  1,
		DebitColumn: 2,
		CreditColumn: 3,
	}
	parser := NewParser(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser.Parse(bytes.NewReader(csvData))
	}
}

// BenchmarkParserColumnAutoDetect tests header auto-detection performance
func BenchmarkParserColumnAutoDetect(b *testing.B) {
	// CSV with various header names
	headers := [][]string{
		{"date", "description", "amount", "category"},
		{"Data", "Descrição", "Valor", "Categoria"},
		{"Date Mov.", "Details", "Debit", "Credit"},
		{"Datum", "Descripción", "Importe", "Tipo"},
	}

	for i, hdr := range headers {
		b.Run(fmt.Sprintf("Headers_%d", i), func(b *testing.B) {
			var buf bytes.Buffer
			writer := csv.NewWriter(&buf)
			writer.Write(hdr)
			for j := 0; j < 100; j++ {
				if len(hdr) == 4 && strings.Contains(strings.ToLower(hdr[2]), "debit") {
					writer.Write([]string{"2024-01-15", "Test", "100.00", ""})
				} else {
					writer.Write([]string{"2024-01-15", "Test", "100.00", "Food"})
				}
			}
			writer.Flush()
			csvData := buf.Bytes()

			// Auto-detect columns (use -1 for auto)
			config := ParserConfig{
				DateColumn:   -1,
				DescColumn:   -1,
				AmountColumn: -1,
			}
			parser := NewParser(config)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				parser.Parse(bytes.NewReader(csvData))
			}
		})
	}
}
