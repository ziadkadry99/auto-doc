package indexer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ziadkadry99/auto-doc/internal/config"
	"github.com/ziadkadry99/auto-doc/internal/llm"
)

// FileAnalyzer sends source files to an LLM and parses the structured analysis.
type FileAnalyzer struct {
	provider llm.Provider
	tier     config.QualityTier
	model    string
}

// NewFileAnalyzer creates a new FileAnalyzer.
func NewFileAnalyzer(provider llm.Provider, tier config.QualityTier, model string) *FileAnalyzer {
	return &FileAnalyzer{
		provider: provider,
		tier:     tier,
		model:    model,
	}
}

// AnalyzeResult holds both the analysis and token usage from a single file analysis.
type AnalyzeResult struct {
	Analysis     *FileAnalysis
	InputTokens  int
	OutputTokens int
}

// completeWithRetry calls the LLM with exponential backoff on rate limit errors.
func (a *FileAnalyzer) completeWithRetry(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	maxRetries := 5
	backoff := 15 * time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, err := a.provider.Complete(ctx, req)
		if err == nil {
			return resp, nil
		}

		errStr := err.Error()
		isRateLimit := strings.Contains(errStr, "rate_limit") || strings.Contains(errStr, "429") || strings.Contains(errStr, "too many requests")
		isOverloaded := strings.Contains(errStr, "overloaded")

		if !isRateLimit && !isOverloaded {
			return nil, err
		}

		if attempt == maxRetries {
			return nil, fmt.Errorf("rate limited after %d retries: %w", maxRetries, err)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
			backoff = backoff * 2
			if backoff > 2*time.Minute {
				backoff = 2 * time.Minute
			}
		}
	}
	return nil, fmt.Errorf("unreachable")
}

// Analyze sends a file to the LLM and returns the structured analysis.
func (a *FileAnalyzer) Analyze(ctx context.Context, filePath string, content []byte, language string) (*AnalyzeResult, error) {
	contentStr := string(content)
	messages := buildMessages(a.tier, filePath, contentStr, language)

	resp, err := a.completeWithRetry(ctx, llm.CompletionRequest{
		Model:       a.model,
		Messages:    messages,
		MaxTokens:   4096,
		Temperature: 0.1,
		JSONMode:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("llm completion: %w", err)
	}

	analysis, parseErr := parseAnalysis(resp.Content)
	if parseErr != nil {
		// Retry with a simpler fallback prompt.
		fallbackMsgs := buildFallbackMessages(filePath, contentStr)
		fallbackResp, fallbackErr := a.completeWithRetry(ctx, llm.CompletionRequest{
			Model:       a.model,
			Messages:    fallbackMsgs,
			MaxTokens:   1024,
			Temperature: 0.0,
			JSONMode:    true,
		})
		if fallbackErr != nil {
			// Return a minimal analysis with just the file path.
			analysis = &FileAnalysis{
				FilePath: filePath,
				Language: language,
				Summary:  "Analysis failed: could not parse LLM response.",
			}
		} else {
			analysis, _ = parseAnalysis(fallbackResp.Content)
			if analysis == nil {
				analysis = &FileAnalysis{
					FilePath: filePath,
					Language: language,
					Summary:  "Analysis failed: could not parse LLM response.",
				}
			}
			resp.InputTokens += fallbackResp.InputTokens
			resp.OutputTokens += fallbackResp.OutputTokens
		}
	}

	analysis.FilePath = filePath
	analysis.Language = language
	analysis.ContentHash = computeHash(content)

	return &AnalyzeResult{
		Analysis:     analysis,
		InputTokens:  resp.InputTokens,
		OutputTokens: resp.OutputTokens,
	}, nil
}

// parseAnalysis parses an LLM JSON response into a FileAnalysis struct.
func parseAnalysis(raw string) (*FileAnalysis, error) {
	raw = strings.TrimSpace(raw)

	// Strip markdown code fences if present.
	if strings.HasPrefix(raw, "```") {
		lines := strings.Split(raw, "\n")
		if len(lines) >= 2 {
			// Remove first line (```json) and last line (```)
			start := 1
			end := len(lines)
			if strings.TrimSpace(lines[end-1]) == "```" {
				end--
			}
			raw = strings.Join(lines[start:end], "\n")
		}
	}

	var analysis FileAnalysis
	if err := json.Unmarshal([]byte(raw), &analysis); err != nil {
		return nil, fmt.Errorf("json parse: %w", err)
	}
	return &analysis, nil
}

// computeHash computes a SHA-256 hash of the given content.
func computeHash(content []byte) string {
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:])
}
