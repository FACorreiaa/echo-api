package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"strings"

	"buf.build/gen/go/echo-tracker/echo/connectrpc/go/echo/v1/echov1connect"
	echov1 "buf.build/gen/go/echo-tracker/echo/protocolbuffers/go/echo/v1"
	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	importservice "github.com/FACorreiaa/smart-finance-tracker/internal/domain/import/service"
	planexcel "github.com/FACorreiaa/smart-finance-tracker/internal/domain/plan/excel"
	"github.com/FACorreiaa/smart-finance-tracker/pkg/interceptors"
	"github.com/FACorreiaa/smart-finance-tracker/pkg/storage"
)

var _ echov1connect.ImportServiceHandler = (*ImportHandler)(nil)

// ImportHandler handles Import service RPCs
type ImportHandler struct {
	importSvc *importservice.ImportService
	storage   storage.Storage
	logger    *slog.Logger
}

// NewImportHandler creates a new import handler
func NewImportHandler(importSvc *importservice.ImportService, fileStorage storage.Storage, logger *slog.Logger) *ImportHandler {
	return &ImportHandler{
		importSvc: importSvc,
		storage:   fileStorage,
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
	// Get user ID from auth context
	userIDStr, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Validate request
	fileBytes := req.Msg.GetFileBytes()
	if len(fileBytes) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, nil)
	}

	// Calculate checksum if not provided
	checksum := req.Msg.GetChecksumSha256()
	if checksum == "" {
		hash := sha256.Sum256(fileBytes)
		checksum = hex.EncodeToString(hash[:])
	}

	// Upload to storage
	fileInfo, err := h.storage.Upload(
		ctx,
		userID,
		req.Msg.GetFileName(),
		req.Msg.GetMimeType(),
		bytes.NewReader(fileBytes),
	)
	if err != nil {
		h.logger.Error("failed to upload file to storage", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Create DB record via import service
	userFile, err := h.importSvc.CreateUserFile(ctx, importservice.CreateUserFileInput{
		UserID:     userID,
		FileID:     fileInfo.ID,
		Type:       protoFileTypeToString(req.Msg.GetType()),
		MimeType:   req.Msg.GetMimeType(),
		FileName:   req.Msg.GetFileName(),
		SizeBytes:  int64(len(fileBytes)),
		Checksum:   checksum,
		StorageURL: fileInfo.Path,
	})
	if err != nil {
		h.logger.Error("failed to create user file record", slog.Any("error", err))
		// Try to clean up the uploaded file
		_ = h.storage.Delete(ctx, userID, fileInfo.ID)
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&echov1.UploadUserFileResponse{
		File: &echov1.UserFile{
			Id:             userFile.ID.String(),
			UserId:         userFile.UserID.String(),
			Type:           req.Msg.GetType(),
			MimeType:       userFile.MimeType,
			FileName:       userFile.FileName,
			SizeBytes:      userFile.SizeBytes,
			ChecksumSha256: &userFile.Checksum,
			StorageUrl:     userFile.StorageURL,
			CreatedAt:      timestamppb.New(userFile.CreatedAt),
		},
	}), nil
}

// protoFileTypeToString converts proto file type to string for DB
func protoFileTypeToString(ft echov1.UserFileType) string {
	switch ft {
	case echov1.UserFileType_USER_FILE_TYPE_CSV:
		return "csv"
	case echov1.UserFileType_USER_FILE_TYPE_XLSX:
		return "xlsx"
	case echov1.UserFileType_USER_FILE_TYPE_PDF:
		return "pdf"
	case echov1.UserFileType_USER_FILE_TYPE_IMAGE:
		return "image"
	default:
		return "csv"
	}
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

// AnalyzeFile analyzes any file (CSV/TSV/XLSX) and returns routing hint
func (h *ImportHandler) AnalyzeFile(ctx context.Context, req *connect.Request[echov1.AnalyzeFileRequest]) (*connect.Response[echov1.AnalyzeFileResponse], error) {
	// Get user ID from auth context
	userIDStr, ok := interceptors.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	fileBytes := req.Msg.GetFileBytes()
	fileName := req.Msg.GetFileName()
	mimeType := req.Msg.GetMimeType()

	h.logger.Info("analyzing file",
		slog.String("user_id", userID.String()),
		slog.String("file_name", fileName),
		slog.String("mime_type", mimeType),
		slog.Int("size_bytes", len(fileBytes)),
	)

	// Determine file type
	isExcel := strings.HasSuffix(strings.ToLower(fileName), ".xlsx") ||
		strings.HasSuffix(strings.ToLower(fileName), ".xls") ||
		strings.Contains(mimeType, "spreadsheet") ||
		strings.Contains(mimeType, "excel")

	if isExcel {
		return h.analyzeExcelFile(ctx, userID, fileBytes, fileName)
	}

	// Default: treat as CSV/TSV
	return h.analyzeCsvTsvFile(ctx, userID, fileBytes)
}

// analyzeExcelFile analyzes an Excel file and determines if it's a budget or transactions
func (h *ImportHandler) analyzeExcelFile(ctx context.Context, userID uuid.UUID, fileBytes []byte, fileName string) (*connect.Response[echov1.AnalyzeFileResponse], error) {
	parser, err := planexcel.NewParserFromReader(bytes.NewReader(fileBytes))
	if err != nil {
		h.logger.Error("failed to parse Excel file", slog.Any("error", err))
		return connect.NewResponse(&echov1.AnalyzeFileResponse{
			RoutingHint:  echov1.ImportRoutingHint_IMPORT_ROUTING_HINT_TRANSACTIONS,
			FileType:     echov1.UserFileType_USER_FILE_TYPE_XLSX,
			ErrorMessage: "Failed to parse Excel file: " + err.Error(),
		}), nil
	}
	defer parser.Close()

	// Analyze all sheets
	analyses, suggestedSheet, err := parser.AnalyzeAllSheets()
	if err != nil {
		h.logger.Error("failed to analyze Excel sheets", slog.Any("error", err))
		return connect.NewResponse(&echov1.AnalyzeFileResponse{
			RoutingHint:  echov1.ImportRoutingHint_IMPORT_ROUTING_HINT_TRANSACTIONS,
			FileType:     echov1.UserFileType_USER_FILE_TYPE_XLSX,
			ErrorMessage: "Failed to analyze sheets: " + err.Error(),
		}), nil
	}

	// Determine if this is a planning sheet (has formulas) or data dump (transactions)
	var isLivingPlan bool
	var totalFormulas int
	var sheets []*echov1.ExcelSheetInfo
	var detectedCategories []string

	for _, a := range analyses {
		totalFormulas += a.FormulaCount
		if a.Type == planexcel.SheetTypeLivingPlan {
			isLivingPlan = true
		}

		sheets = append(sheets, &echov1.ExcelSheetInfo{
			Name:               a.Name,
			RowCount:           int32(a.RowCount),
			ColumnCount:        int32(a.ColCount),
			FormulaCount:       int32(a.FormulaCount),
			IsLivingPlan:       a.Type == planexcel.SheetTypeLivingPlan,
			DetectedCategories: a.DetectedCategories,
			MonthColumns:       a.MonthColumns,
		})

		detectedCategories = append(detectedCategories, a.DetectedCategories...)
	}

	// Determine routing hint
	routingHint := echov1.ImportRoutingHint_IMPORT_ROUTING_HINT_TRANSACTIONS
	if isLivingPlan || totalFormulas > 5 {
		routingHint = echov1.ImportRoutingHint_IMPORT_ROUTING_HINT_PLANNING
	}

	h.logger.Info("Excel file analyzed",
		slog.String("file_name", fileName),
		slog.Int("sheets", len(sheets)),
		slog.Int("total_formulas", totalFormulas),
		slog.Bool("is_living_plan", isLivingPlan),
		slog.String("routing_hint", routingHint.String()),
	)

	return connect.NewResponse(&echov1.AnalyzeFileResponse{
		RoutingHint: routingHint,
		FileType:    echov1.UserFileType_USER_FILE_TYPE_XLSX,
		PlanAnalysis: &echov1.ExcelPlanAnalysis{
			Sheets:             sheets,
			SuggestedSheet:     suggestedSheet,
			DetectedCategories: detectedCategories,
			FormulaCount:       int32(totalFormulas),
			IsLivingPlan:       isLivingPlan,
		},
	}), nil
}

// analyzeCsvTsvFile analyzes a CSV/TSV file
func (h *ImportHandler) analyzeCsvTsvFile(ctx context.Context, userID uuid.UUID, fileBytes []byte) (*connect.Response[echov1.AnalyzeFileResponse], error) {
	result, err := h.importSvc.AnalyzeFile(ctx, userID, fileBytes)
	if err != nil {
		h.logger.Error("failed to analyze CSV/TSV file", slog.Any("error", err))
		return connect.NewResponse(&echov1.AnalyzeFileResponse{
			RoutingHint:  echov1.ImportRoutingHint_IMPORT_ROUTING_HINT_TRANSACTIONS,
			FileType:     echov1.UserFileType_USER_FILE_TYPE_CSV,
			ErrorMessage: "Failed to analyze file: " + err.Error(),
		}), nil
	}

	// Build CSV analysis response
	var sampleRows []*echov1.CsvSampleRow
	if result.FileConfig != nil {
		for _, row := range result.FileConfig.SampleRows {
			sampleRows = append(sampleRows, &echov1.CsvSampleRow{Cells: row})
		}
	}

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

	var dialect *echov1.CsvRegionalDialect
	if result.ProbedDialect != nil {
		dialect = &echov1.CsvRegionalDialect{
			IsEuropeanFormat: result.ProbedDialect.IsEuropeanFormat,
			DateFormat:       result.ProbedDialect.DateFormat,
			Confidence:       result.ProbedDialect.Confidence,
			CurrencyHint:     result.ProbedDialect.CurrencyHint,
		}
	}

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

	return connect.NewResponse(&echov1.AnalyzeFileResponse{
		RoutingHint: echov1.ImportRoutingHint_IMPORT_ROUTING_HINT_TRANSACTIONS,
		FileType:    echov1.UserFileType_USER_FILE_TYPE_CSV,
		CsvAnalysis: &echov1.AnalyzeCsvFileResponse{
			Headers:       headers,
			SampleRows:    sampleRows,
			Delimiter:     delimiter,
			SkipLines:     skipLines,
			Fingerprint:   fingerprint,
			Suggestions:   suggestions,
			ProbedDialect: dialect,
			MappingFound:  result.MappingFound,
			CanAutoImport: result.CanAutoImport,
		},
	}), nil
}
