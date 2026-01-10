// Package parser provides file parsing utilities for the import service.
// pdf_parser.go is a stub for future PDF OCR implementation.
package parser

import (
	"errors"
	"io"
)

// ErrPDFNotSupported indicates PDF parsing is not yet implemented
var ErrPDFNotSupported = errors.New("PDF parsing not yet supported - coming soon")

// PDFParser is a placeholder for future PDF parsing implementation.
// This will use OCR to extract transaction data from PDF bank statements.
type PDFParser struct {
	// Future: OCR engine configuration
	// Future: Template patterns for common bank statement layouts
}

// NewPDFParser creates a new PDF parser instance.
func NewPDFParser() *PDFParser {
	return &PDFParser{}
}

// ParsePDF is a stub that returns an error indicating PDF support is coming soon.
// Future implementation will:
// 1. Use OCR (Tesseract or cloud service) to extract text
// 2. Apply template matching for known bank statement formats
// 3. Extract transactions with date, description, and amount
func (p *PDFParser) ParsePDF(r io.Reader) ([]map[string]string, error) {
	return nil, ErrPDFNotSupported
}

// DetectBankFormat is a stub for future bank statement format detection.
// Will analyze the PDF layout to identify which bank issued the statement.
func (p *PDFParser) DetectBankFormat(r io.Reader) (string, error) {
	return "", ErrPDFNotSupported
}

// ExtractTables is a stub for future table extraction from PDFs.
// Will use camelot-py or similar to extract tabular data.
func (p *PDFParser) ExtractTables(r io.Reader) ([][]string, error) {
	return nil, ErrPDFNotSupported
}
