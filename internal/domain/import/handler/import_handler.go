package handler

import (
	"context"
	"log/slog"

	"buf.build/gen/go/echo-tracker/echo/connectrpc/go/echo/v1/echov1connect"
	echov1 "buf.build/gen/go/echo-tracker/echo/protocolbuffers/go/echo/v1"
	"connectrpc.com/connect"
	"github.com/google/uuid"

	importservice "github.com/FACorreiaa/smart-finance-tracker/internal/domain/import/service"
	"github.com/FACorreiaa/smart-finance-tracker/pkg/interceptors"
)

var _ echov1connect.ImportServiceHandler = (*ImportHandler)(nil)

// ImportHandler handles Import service RPCs
type ImportHandler struct {
	importSvc *importservice.ImportService
	logger    *slog.Logger
}

// NewImportHandler creates a new import handler
func NewImportHandler(importSvc *importservice.ImportService, logger *slog.Logger) *ImportHandler {
	return &ImportHandler{
		importSvc: importSvc,
		logger:    logger,
	}
}

// AnalyzeCsvFile analyzes a CSV file and returns detected configuration
func (h *ImportHandler) AnalyzeCsvFile(ctx context.Context, req *connect.Request[echov1.AnalyzeCsvFileRequest]) (*connect.Response[echov1.AnalyzeCsvFileResponse], error) {
	// Get user ID from auth context
	userIDStr, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	result, err := h.importSvc.AnalyzeFile(ctx, userID, req.Msg.GetCsvBytes())
	if err != nil {
		h.logger.Error("failed to analyze CSV file", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Convert sample rows to proto format
	var sampleRows []*echov1.CsvSampleRow
	if result.FileConfig != nil && len(result.FileConfig.SampleRows) > 0 {
		sampleRows = make([]*echov1.CsvSampleRow, 0, len(result.FileConfig.SampleRows))
		for _, row := range result.FileConfig.SampleRows {
			sampleRows = append(sampleRows, &echov1.CsvSampleRow{
				Cells: row,
			})
		}
	}

	// Build suggestions from detected columns
	var suggestions *echov1.CsvColumnSuggestions
	if result.ColumnSuggestions != nil {
		suggestions = &echov1.CsvColumnSuggestions{
			DateCol:       int32(result.ColumnSuggestions.DateCol),
			DescCol:       int32(result.ColumnSuggestions.DescCol),
			AmountCol:     int32(result.ColumnSuggestions.AmountCol),
			DebitCol:      int32(result.ColumnSuggestions.DebitCol),
			CreditCol:     int32(result.ColumnSuggestions.CreditCol),
			CategoryCol:   int32(result.ColumnSuggestions.CategoryCol),
			IsDoubleEntry: result.ColumnSuggestions.IsDoubleEntry,
		}
	}

	// Build probed dialect
	var dialect *echov1.CsvRegionalDialect
	if result.ProbedDialect != nil {
		dialect = &echov1.CsvRegionalDialect{
			IsEuropeanFormat: result.ProbedDialect.IsEuropeanFormat,
			DateFormat:       result.ProbedDialect.DateFormat,
			Confidence:       result.ProbedDialect.Confidence,
			CurrencyHint:     result.ProbedDialect.CurrencyHint,
		}
	}

	// Build response from file config
	var headers []string
	var delimiter string
	var skipLines int32
	var fingerprint string

	if result.FileConfig != nil {
		headers = result.FileConfig.Headers
		delimiter = string(result.FileConfig.Delimiter)
		skipLines = int32(result.FileConfig.SkipLines)
		fingerprint = result.FileConfig.Fingerprint
	}

	return connect.NewResponse(&echov1.AnalyzeCsvFileResponse{
		Headers:       headers,
		SampleRows:    sampleRows,
		Delimiter:     delimiter,
		SkipLines:     skipLines,
		Fingerprint:   fingerprint,
		Suggestions:   suggestions,
		ProbedDialect: dialect,
		MappingFound:  result.MappingFound,
		CanAutoImport: result.CanAutoImport,
	}), nil
}

// UploadUserFile stores a file for later processing
func (h *ImportHandler) UploadUserFile(ctx context.Context, req *connect.Request[echov1.UploadUserFileRequest]) (*connect.Response[echov1.UploadUserFileResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

// GetUserFile retrieves a stored file
func (h *ImportHandler) GetUserFile(ctx context.Context, req *connect.Request[echov1.GetUserFileRequest]) (*connect.Response[echov1.GetUserFileResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

// ListUserFiles lists stored files
func (h *ImportHandler) ListUserFiles(ctx context.Context, req *connect.Request[echov1.ListUserFilesRequest]) (*connect.Response[echov1.ListUserFilesResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

// CreateImportJob creates an import job
func (h *ImportHandler) CreateImportJob(ctx context.Context, req *connect.Request[echov1.CreateImportJobRequest]) (*connect.Response[echov1.CreateImportJobResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

// GetImportJob retrieves an import job
func (h *ImportHandler) GetImportJob(ctx context.Context, req *connect.Request[echov1.GetImportJobRequest]) (*connect.Response[echov1.GetImportJobResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

// ListImportJobs lists import jobs
func (h *ImportHandler) ListImportJobs(ctx context.Context, req *connect.Request[echov1.ListImportJobsRequest]) (*connect.Response[echov1.ListImportJobsResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

// GetDocument retrieves a document
func (h *ImportHandler) GetDocument(ctx context.Context, req *connect.Request[echov1.GetDocumentRequest]) (*connect.Response[echov1.GetDocumentResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

// ListDocuments lists documents
func (h *ImportHandler) ListDocuments(ctx context.Context, req *connect.Request[echov1.ListDocumentsRequest]) (*connect.Response[echov1.ListDocumentsResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}
