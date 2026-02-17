package indexer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ziadkadry99/auto-doc/internal/config"
	"github.com/ziadkadry99/auto-doc/internal/embeddings"
	"github.com/ziadkadry99/auto-doc/internal/llm"
	"github.com/ziadkadry99/auto-doc/internal/vectordb"
	"github.com/ziadkadry99/auto-doc/internal/walker"
)

// Pipeline orchestrates the full indexing workflow: walk -> analyze -> chunk -> embed -> store.
type Pipeline struct {
	llmProvider llm.Provider
	embedder    embeddings.Embedder
	store       vectordb.VectorStore
	cfg         *config.Config
	rootDir     string
	onProgress  ProgressFunc
}

// NewPipeline creates a new Pipeline.
func NewPipeline(
	llmProvider llm.Provider,
	embedder embeddings.Embedder,
	store vectordb.VectorStore,
	cfg *config.Config,
	rootDir string,
) *Pipeline {
	return &Pipeline{
		llmProvider: llmProvider,
		embedder:    embedder,
		store:       store,
		cfg:         cfg,
		rootDir:     rootDir,
	}
}

// SetProgressFunc sets the progress callback.
func (p *Pipeline) SetProgressFunc(fn ProgressFunc) {
	p.onProgress = fn
}

// Run executes the full indexing pipeline.
func (p *Pipeline) Run(ctx context.Context, files []walker.FileInfo) (*PipelineResult, error) {
	start := time.Now()
	result := &PipelineResult{
		Analyses: make(map[string]FileAnalysis),
	}

	// Load or create state.
	state, err := LoadState(p.rootDir)
	if err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}

	// Filter out unchanged files and build a hash lookup.
	var changed []walker.FileInfo
	walkerHashes := make(map[string]string) // relPath -> walker ContentHash
	for _, f := range files {
		if state.IsFileChanged(f.RelPath, f.ContentHash) {
			changed = append(changed, f)
			walkerHashes[f.RelPath] = f.ContentHash
		} else {
			result.FilesSkipped++
		}
	}

	if len(changed) == 0 {
		result.Duration = time.Since(start)
		return result, nil
	}

	// Analyze files.
	concurrency := p.cfg.MaxConcurrency
	if concurrency < 1 {
		concurrency = 4
	}
	analyzer := NewFileAnalyzer(p.llmProvider, p.cfg.Quality, p.cfg.Model)
	batcher := NewBatcher(concurrency, analyzer, p.onProgress)

	batchResult := batcher.ProcessFiles(ctx, changed)
	result.Errors = append(result.Errors, batchResult.Errors...)
	result.TotalInputTokens = batchResult.InputTokens
	result.TotalOutputTokens = batchResult.OutputTokens
	result.FilesFailed = len(batchResult.Errors)

	// Chunk, embed, and store each analysis.
	for _, ar := range batchResult.Results {
		docs := ChunkAnalysis(ar.Analysis, p.cfg.Quality)

		// Delete old documents for this file before adding new ones.
		if err := p.store.DeleteByFilePath(ctx, ar.Analysis.FilePath); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("delete old docs for %s: %w", ar.Analysis.FilePath, err))
			continue
		}

		if err := p.store.AddDocuments(ctx, docs); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("store docs for %s: %w", ar.Analysis.FilePath, err))
			continue
		}

		// Update state using the walker's content hash for consistent comparisons.
		if h, ok := walkerHashes[ar.Analysis.FilePath]; ok {
			state.FileHashes[ar.Analysis.FilePath] = h
		} else {
			state.FileHashes[ar.Analysis.FilePath] = ar.Analysis.ContentHash
		}
		result.Analyses[ar.Analysis.FilePath] = *ar.Analysis
		result.FilesProcessed++
	}

	// Build and index reverse-dependency documents.
	reverseDeps := buildReverseDependencyDocs(result.Analyses)
	if len(reverseDeps) > 0 {
		if err := p.store.AddDocuments(ctx, reverseDeps); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("store reverse-dep docs: %w", err))
		}
	}

	// Persist the vector store.
	autodocDir := fmt.Sprintf("%s/.autodoc", p.rootDir)
	if err := p.store.Persist(ctx, autodocDir); err != nil {
		return result, fmt.Errorf("persist store: %w", err)
	}

	// Save index state.
	state.LastCommitSHA = GetGitCommitSHA(p.rootDir)
	if err := state.SaveState(p.rootDir); err != nil {
		return result, fmt.Errorf("save state: %w", err)
	}

	result.Duration = time.Since(start)
	return result, nil
}

// DryRun estimates cost without making API calls.
func (p *Pipeline) DryRun(ctx context.Context, files []walker.FileInfo) (*CostEstimate, error) {
	state, err := LoadState(p.rootDir)
	if err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}

	var changed []walker.FileInfo
	for _, f := range files {
		if state.IsFileChanged(f.RelPath, f.ContentHash) {
			changed = append(changed, f)
		}
	}

	estimate := &CostEstimate{
		TotalFiles:    len(changed),
		CostBreakdown: make(map[string]float64),
	}

	// Estimate tokens: ~1 token per 4 characters of source code.
	var totalInputTokens int
	for _, f := range changed {
		fileTokens := int(f.Size) / 4
		totalInputTokens += fileTokens
	}

	// Output tokens estimate: ~500 per file for lite, ~1500 for normal, ~3000 for max.
	var outputPerFile int
	switch p.cfg.Quality {
	case config.QualityMax:
		outputPerFile = 3000
	case config.QualityNormal:
		outputPerFile = 1500
	default:
		outputPerFile = 500
	}
	totalOutputTokens := len(changed) * outputPerFile

	estimate.TotalTokensEstimate = totalInputTokens + totalOutputTokens

	// Cost estimation using approximate rates (per 1M tokens).
	inputCostPerM := 3.0   // $3 per 1M input tokens (rough average)
	outputCostPerM := 15.0 // $15 per 1M output tokens (rough average)

	analysisCost := float64(totalInputTokens)/1_000_000*inputCostPerM +
		float64(totalOutputTokens)/1_000_000*outputCostPerM
	estimate.CostBreakdown["analysis"] = analysisCost

	// Embedding cost: ~$0.10 per 1M tokens.
	embeddingTokens := totalInputTokens / 2 // Embeddings are generated for summaries, not full source.
	embeddingCost := float64(embeddingTokens) / 1_000_000 * 0.10
	estimate.CostBreakdown["embeddings"] = embeddingCost

	// Architecture pass (only for Normal and Max).
	if p.cfg.Quality != config.QualityLite && len(changed) > 0 {
		archCost := float64(len(changed)*200)/1_000_000*inputCostPerM +
			2000.0/1_000_000*outputCostPerM
		estimate.CostBreakdown["architecture"] = archCost
		analysisCost += archCost
	}

	estimate.EstimatedCost = analysisCost + embeddingCost

	return estimate, nil
}

// buildReverseDependencyDocs creates documents for dependencies that are used by 2+ files.
// This enables "what depends on X" / blast-radius queries.
func buildReverseDependencyDocs(analyses map[string]FileAnalysis) []vectordb.Document {
	// Build reverse map: dependency name → list of files that depend on it.
	reverseMap := make(map[string][]string)
	for _, analysis := range analyses {
		for _, dep := range analysis.Dependencies {
			reverseMap[dep.Name] = append(reverseMap[dep.Name], analysis.FilePath)
		}
	}

	now := time.Now()
	var docs []vectordb.Document

	for depName, dependents := range reverseMap {
		if len(dependents) < 2 {
			continue
		}

		var parts []string
		parts = append(parts, fmt.Sprintf("Dependency: %s", depName))
		parts = append(parts, fmt.Sprintf("Used by %d files (blast radius):", len(dependents)))
		for _, f := range dependents {
			parts = append(parts, fmt.Sprintf("- %s depends on %s", f, depName))
		}
		parts = append(parts, fmt.Sprintf("\nChanges to %s could affect all %d files listed above.", depName, len(dependents)))

		docs = append(docs, vectordb.Document{
			ID:      fmt.Sprintf("reverse-dep:%s", strings.ToLower(strings.ReplaceAll(depName, "/", "-"))),
			Content: strings.Join(parts, "\n"),
			Metadata: vectordb.DocumentMetadata{
				FilePath:    depName,
				Type:        vectordb.DocTypeFile,
				Symbol:      "reverse-dependency",
				LastUpdated: now,
			},
		})
	}

	return docs
}
