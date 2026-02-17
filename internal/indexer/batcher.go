package indexer

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/ziadkadry99/auto-doc/internal/walker"
)

// Batcher processes files concurrently through the analyzer with configurable parallelism.
type Batcher struct {
	concurrency int
	analyzer    *FileAnalyzer
	onProgress  ProgressFunc
}

// NewBatcher creates a new Batcher with the given concurrency limit.
func NewBatcher(concurrency int, analyzer *FileAnalyzer, onProgress ProgressFunc) *Batcher {
	if concurrency < 1 {
		concurrency = 1
	}
	return &Batcher{
		concurrency: concurrency,
		analyzer:    analyzer,
		onProgress:  onProgress,
	}
}

// BatchResult holds collected results and errors from batch processing.
type BatchResult struct {
	Results      []AnalyzeResult
	Errors       []error
	InputTokens  int
	OutputTokens int
}

// ProcessFiles analyzes a list of files concurrently.
func (b *Batcher) ProcessFiles(ctx context.Context, files []walker.FileInfo) *BatchResult {
	total := len(files)
	if total == 0 {
		return &BatchResult{}
	}

	// Circuit breaker: cancel remaining work if quota is exhausted.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var quotaExhausted int64

	sem := make(chan struct{}, b.concurrency)
	var mu sync.Mutex
	var processed int64
	result := &BatchResult{}

	var wg sync.WaitGroup
	for _, file := range files {
		// Check circuit breaker before starting new work.
		if atomic.LoadInt64(&quotaExhausted) > 0 {
			mu.Lock()
			result.Errors = append(result.Errors, fmt.Errorf("analyze %s: skipped (API quota exhausted)", file.RelPath))
			mu.Unlock()
			count := atomic.AddInt64(&processed, 1)
			if b.onProgress != nil {
				b.onProgress(int(count), total, file.RelPath)
			}
			continue
		}

		select {
		case <-ctx.Done():
			mu.Lock()
			result.Errors = append(result.Errors, ctx.Err())
			mu.Unlock()
			count := atomic.AddInt64(&processed, 1)
			if b.onProgress != nil {
				b.onProgress(int(count), total, file.RelPath)
			}
			continue
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func(f walker.FileInfo) {
			defer wg.Done()
			defer func() { <-sem }()

			content, err := os.ReadFile(f.Path)
			if err != nil {
				mu.Lock()
				result.Errors = append(result.Errors, fmt.Errorf("read %s: %w", f.RelPath, err))
				mu.Unlock()
				count := atomic.AddInt64(&processed, 1)
				if b.onProgress != nil {
					b.onProgress(int(count), total, f.RelPath)
				}
				return
			}

			ar, err := b.analyzer.Analyze(ctx, f.RelPath, content, f.Language)
			mu.Lock()
			if err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("analyze %s: %w", f.RelPath, err))
				// Detect quota exhaustion and trip circuit breaker.
				errStr := err.Error()
				if strings.Contains(errStr, "RESOURCE_EXHAUSTED") || strings.Contains(errStr, "quota") {
					atomic.StoreInt64(&quotaExhausted, 1)
					cancel()
				}
			} else {
				result.Results = append(result.Results, *ar)
				result.InputTokens += ar.InputTokens
				result.OutputTokens += ar.OutputTokens
			}
			mu.Unlock()

			count := atomic.AddInt64(&processed, 1)
			if b.onProgress != nil {
				b.onProgress(int(count), total, f.RelPath)
			}
		}(file)
	}

	wg.Wait()
	return result
}
