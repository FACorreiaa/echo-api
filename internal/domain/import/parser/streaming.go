package parser

import (
	"context"
	"encoding/csv"
	"io"
	"runtime"
	"sync"
)

// StreamingParser provides memory-efficient parsing for large CSV files.
// It processes rows through channels with configurable worker pools.
type StreamingParser struct {
	config      ParserConfig
	workerCount int
}

// StreamResult is sent through the channel for each parsed transaction
type StreamResult struct {
	Transaction *ParsedTransaction
	Error       *ParseError
	RowNum      int
}

// StreamStats tracks parsing statistics
type StreamStats struct {
	TotalRows   int64
	ParsedRows  int64
	SkippedRows int64
	ErrorRows   int64
}

// NewStreamingParser creates a streaming parser with configurable worker count
func NewStreamingParser(config ParserConfig, workers int) *StreamingParser {
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	return &StreamingParser{
		config:      config,
		workerCount: workers,
	}
}

// ParseStream reads CSV data and streams results through a channel.
// The channel is closed when parsing completes or context is cancelled.
// Returns the results channel and a stats channel (single value on completion).
func (p *StreamingParser) ParseStream(ctx context.Context, reader io.Reader) (<-chan StreamResult, <-chan StreamStats) {
	results := make(chan StreamResult, p.workerCount*100)
	statsChan := make(chan StreamStats, 1)

	go p.parseAsync(ctx, reader, results, statsChan)

	return results, statsChan
}

func (p *StreamingParser) parseAsync(ctx context.Context, reader io.Reader, results chan<- StreamResult, statsChan chan<- StreamStats) {
	defer close(results)
	defer close(statsChan)

	stats := StreamStats{}

	// Skip lines if configured
	if p.config.SkipLines > 0 {
		reader = skipLines(reader, p.config.SkipLines)
	}

	// Create CSV reader
	csvReader := csv.NewReader(reader)
	if p.config.Delimiter != 0 {
		csvReader.Comma = p.config.Delimiter
	}
	csvReader.LazyQuotes = true
	csvReader.TrimLeadingSpace = true
	csvReader.FieldsPerRecord = -1 // Variable field count
	csvReader.ReuseRecord = true   // Memory optimization

	// Read header to determine column count
	header, err := csvReader.Read()
	if err != nil {
		results <- StreamResult{
			Error: &ParseError{
				Row:     1,
				Message: "failed to read header: " + err.Error(),
			},
		}
		return
	}

	// Create worker pool
	rowsChan := make(chan rowJob, p.workerCount*10)
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < p.workerCount; i++ {
		wg.Add(1)
		go p.worker(ctx, header, rowsChan, results, &wg)
	}

	// Read and dispatch rows
	rowNum := p.config.SkipLines + 2 // 1-indexed + header
	for {
		select {
		case <-ctx.Done():
			close(rowsChan)
			wg.Wait()
			stats.ErrorRows++
			statsChan <- stats
			return
		default:
		}

		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			results <- StreamResult{
				RowNum: rowNum,
				Error: &ParseError{
					Row:     rowNum,
					Message: err.Error(),
				},
			}
			stats.ErrorRows++
			rowNum++
			continue
		}

		stats.TotalRows++

		// Make a copy since ReuseRecord is enabled
		recordCopy := make([]string, len(record))
		copy(recordCopy, record)

		rowsChan <- rowJob{
			record: recordCopy,
			rowNum: rowNum,
		}
		rowNum++
	}

	close(rowsChan)
	wg.Wait()

	// Calculate final stats from results (approximate since we're streaming)
	statsChan <- stats
}

type rowJob struct {
	record []string
	rowNum int
}

func (p *StreamingParser) worker(ctx context.Context, header []string, jobs <-chan rowJob, results chan<- StreamResult, wg *sync.WaitGroup) {
	defer wg.Done()

	// Create a local parser for this worker
	localParser := NewParser(p.config)

	for job := range jobs {
		select {
		case <-ctx.Done():
			return
		default:
		}

		tx, err := localParser.processRecord(job.record, job.rowNum)
		results <- StreamResult{
			Transaction: tx,
			Error:       err,
			RowNum:      job.rowNum,
		}
	}
}

// ParseStreamBatched reads CSV and sends batches of transactions.
// This is more efficient for bulk database inserts.
func (p *StreamingParser) ParseStreamBatched(ctx context.Context, reader io.Reader, batchSize int) (<-chan []ParsedTransaction, <-chan []ParseError) {
	if batchSize <= 0 {
		batchSize = 500
	}

	txChan := make(chan []ParsedTransaction, 10)
	errChan := make(chan []ParseError, 10)

	go p.parseBatchedAsync(ctx, reader, batchSize, txChan, errChan)

	return txChan, errChan
}

func (p *StreamingParser) parseBatchedAsync(ctx context.Context, reader io.Reader, batchSize int, txChan chan<- []ParsedTransaction, errChan chan<- []ParseError) {
	defer close(txChan)
	defer close(errChan)

	// Skip lines if configured
	if p.config.SkipLines > 0 {
		reader = skipLines(reader, p.config.SkipLines)
	}

	// Create CSV reader
	csvReader := csv.NewReader(reader)
	if p.config.Delimiter != 0 {
		csvReader.Comma = p.config.Delimiter
	}
	csvReader.LazyQuotes = true
	csvReader.TrimLeadingSpace = true
	csvReader.FieldsPerRecord = -1

	// Skip header
	_, err := csvReader.Read()
	if err != nil {
		errChan <- []ParseError{{Row: 1, Message: "failed to read header: " + err.Error()}}
		return
	}

	localParser := NewParser(p.config)

	batch := make([]ParsedTransaction, 0, batchSize)
	errors := make([]ParseError, 0)
	rowNum := p.config.SkipLines + 2

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			errors = append(errors, ParseError{Row: rowNum, Message: err.Error()})
			rowNum++
			continue
		}

		tx, parseErr := localParser.processRecord(record, rowNum)
		if parseErr != nil {
			errors = append(errors, *parseErr)
		} else if tx != nil {
			batch = append(batch, *tx)
		}

		// Send batch when full
		if len(batch) >= batchSize {
			txChan <- batch
			batch = make([]ParsedTransaction, 0, batchSize)
		}

		rowNum++
	}

	// Send remaining
	if len(batch) > 0 {
		txChan <- batch
	}
	if len(errors) > 0 {
		errChan <- errors
	}
}

// ChunkReader wraps an io.Reader and provides progress tracking
type ChunkReader struct {
	reader    io.Reader
	bytesRead int64
	totalSize int64
	onProgress func(bytesRead, totalSize int64)
}

// NewChunkReader creates a reader that tracks progress
func NewChunkReader(reader io.Reader, totalSize int64, onProgress func(bytesRead, totalSize int64)) *ChunkReader {
	return &ChunkReader{
		reader:     reader,
		totalSize:  totalSize,
		onProgress: onProgress,
	}
}

func (cr *ChunkReader) Read(p []byte) (int, error) {
	n, err := cr.reader.Read(p)
	cr.bytesRead += int64(n)
	if cr.onProgress != nil {
		cr.onProgress(cr.bytesRead, cr.totalSize)
	}
	return n, err
}

// BytesRead returns the number of bytes read so far
func (cr *ChunkReader) BytesRead() int64 {
	return cr.bytesRead
}

// Progress returns the percentage complete (0-100)
func (cr *ChunkReader) Progress() float64 {
	if cr.totalSize <= 0 {
		return 0
	}
	return float64(cr.bytesRead) / float64(cr.totalSize) * 100
}
