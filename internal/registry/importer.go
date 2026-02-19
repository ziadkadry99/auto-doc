package registry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ziadkadry99/auto-doc/internal/config"
	"github.com/ziadkadry99/auto-doc/internal/flows"
	"github.com/ziadkadry99/auto-doc/internal/indexer"
	"github.com/ziadkadry99/auto-doc/internal/vectordb"
)

// Importer handles importing .autodoc/ artifacts from repositories into the central store.
type Importer struct {
	store     *Store
	vecStore  vectordb.VectorStore
	detector  *flows.Detector
	tier      config.QualityTier
}

// NewImporter creates a new import pipeline.
func NewImporter(store *Store, vecStore vectordb.VectorStore, tier config.QualityTier) *Importer {
	return &Importer{
		store:    store,
		vecStore: vecStore,
		detector: flows.NewDetector(),
		tier:     tier,
	}
}

// ImportRepo imports .autodoc/ artifacts from a repository into the central vector store.
func (imp *Importer) ImportRepo(ctx context.Context, repo *Repository) error {
	// 1. Validate the repo path has .autodoc/analyses.json.
	analysesPath := filepath.Join(repo.LocalPath, ".autodoc", "analyses.json")
	if _, err := os.Stat(analysesPath); os.IsNotExist(err) {
		return fmt.Errorf("no .autodoc/analyses.json found at %s — run `autodoc generate` in that repo first", repo.LocalPath)
	}

	// Update status to indexing.
	repo.Status = "indexing"
	if repo.ID != "" {
		imp.store.Update(ctx, repo)
	}

	// 2. Load analyses.
	analyses, err := indexer.LoadAnalyses(repo.LocalPath)
	if err != nil {
		repo.Status = "error"
		imp.store.Update(ctx, repo)
		return fmt.Errorf("loading analyses from %s: %w", repo.LocalPath, err)
	}

	if len(analyses) == 0 {
		repo.Status = "error"
		imp.store.Update(ctx, repo)
		return fmt.Errorf("no analyses found in %s", repo.LocalPath)
	}

	// 3. Delete existing documents for this repo from the vector store.
	if err := imp.vecStore.DeleteByRepoID(ctx, repo.Name); err != nil {
		// Non-fatal: may not have previous data.
		fmt.Fprintf(os.Stderr, "Warning: could not clean old documents for repo %s: %v\n", repo.Name, err)
	}

	// 4. Re-chunk each analysis with repo_id set and add to vector store.
	var allDocs []vectordb.Document
	for _, analysis := range analyses {
		a := analysis // copy
		docs := indexer.ChunkAnalysisForRepo(&a, imp.tier, repo.Name)
		allDocs = append(allDocs, docs...)
	}

	if len(allDocs) > 0 {
		if err := imp.vecStore.AddDocuments(ctx, allDocs); err != nil {
			repo.Status = "error"
			imp.store.Update(ctx, repo)
			return fmt.Errorf("adding documents to vector store: %w", err)
		}
	}

	// 5. Detect cross-service calls from source files.
	crossCalls := imp.detectCrossServiceCalls(repo.LocalPath)

	// 6. Get the current git commit SHA.
	commitSHA := indexer.GetGitCommitSHA(repo.LocalPath)

	// 7. Generate a summary from the analyses.
	summary := generateRepoSummary(analyses)

	// 8. Update repository record.
	repo.Status = "ready"
	repo.LastIndexedAt = time.Now().UTC().Format(time.RFC3339)
	repo.FileCount = len(analyses)
	repo.LastCommitSHA = commitSHA
	repo.Summary = summary
	if repo.ID != "" {
		if err := imp.store.Update(ctx, repo); err != nil {
			return fmt.Errorf("updating repository record: %w", err)
		}
	}

	_ = crossCalls // used by linker in Milestone 2

	return nil
}

// DetectCrossServiceCalls scans source files in a repo for cross-service communication patterns.
// Exported for use by the linker.
func (imp *Importer) DetectCrossServiceCalls(repoPath string) []flows.CrossServiceCall {
	return imp.detectCrossServiceCalls(repoPath)
}

func (imp *Importer) detectCrossServiceCalls(repoPath string) []flows.CrossServiceCall {
	var allCalls []flows.CrossServiceCall

	_ = filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		// Skip .autodoc, .git, vendor, node_modules directories.
		rel, _ := filepath.Rel(repoPath, path)
		if hasExcludedPrefix(rel) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// Only process likely source files.
		if !isSourceFile(path) {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		calls := imp.detector.DetectPatterns(string(data), rel)
		allCalls = append(allCalls, calls...)
		return nil
	})

	return allCalls
}

// generateRepoSummary builds a one-line summary from the analyses.
func generateRepoSummary(analyses map[string]indexer.FileAnalysis) string {
	if len(analyses) == 0 {
		return ""
	}

	// Use the most common language and count.
	langCount := make(map[string]int)
	for _, a := range analyses {
		if a.Language != "" {
			langCount[a.Language]++
		}
	}

	topLang := ""
	topCount := 0
	for lang, count := range langCount {
		if count > topCount {
			topLang = lang
			topCount = count
		}
	}

	// Known entry-point file patterns (base names).
	entryPoints := map[string]bool{
		"main.go": true, "main.py": true, "app.py": true, "server.py": true,
		"index.ts": true, "index.js": true, "server.js": true, "server.ts": true, "app.ts": true, "app.js": true,
		"Program.cs": true, "Startup.cs": true,
		"Main.java": true, "Application.java": true, "App.java": true,
		"main.rs": true, "lib.rs": true,
	}

	// Find a file that looks like a main entry point for a summary.
	for _, a := range analyses {
		if a.Purpose != "" && entryPoints[filepath.Base(a.FilePath)] {
			return a.Purpose
		}
	}

	// Source code extensions for preferring code files over config files.
	sourceExts := map[string]bool{
		".go": true, ".py": true, ".js": true, ".ts": true, ".tsx": true, ".jsx": true,
		".java": true, ".rs": true, ".rb": true, ".cs": true, ".cpp": true, ".c": true,
		".swift": true, ".kt": true, ".scala": true, ".php": true,
	}

	// Try source files whose name contains "server", "service", "main", or "app".
	for _, a := range analyses {
		if a.Purpose == "" {
			continue
		}
		ext := strings.ToLower(filepath.Ext(a.FilePath))
		if !sourceExts[ext] {
			continue
		}
		base := strings.ToLower(filepath.Base(a.FilePath))
		for _, keyword := range []string{"server", "service", "main", "app"} {
			if strings.Contains(base, keyword) {
				return a.Purpose
			}
		}
	}

	// Try any source code file with a non-empty Purpose.
	for _, a := range analyses {
		if a.Purpose == "" {
			continue
		}
		ext := strings.ToLower(filepath.Ext(a.FilePath))
		if sourceExts[ext] {
			return a.Purpose
		}
	}

	// Last resort: use the first non-empty Purpose from any file.
	for _, a := range analyses {
		if a.Purpose != "" {
			return a.Purpose
		}
	}

	if topLang != "" {
		return fmt.Sprintf("%s project with %d files", topLang, len(analyses))
	}
	return fmt.Sprintf("%d files indexed", len(analyses))
}

func hasExcludedPrefix(path string) bool {
	excluded := []string{".autodoc", ".git", "vendor", "node_modules", ".venv", "__pycache__"}
	for _, prefix := range excluded {
		if len(path) >= len(prefix) && path[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

func isSourceFile(path string) bool {
	ext := filepath.Ext(path)
	sourceExts := map[string]bool{
		".go": true, ".py": true, ".js": true, ".ts": true, ".tsx": true, ".jsx": true,
		".java": true, ".rs": true, ".rb": true, ".cs": true, ".cpp": true, ".c": true,
		".h": true, ".hpp": true, ".swift": true, ".kt": true, ".scala": true,
		".php": true, ".ex": true, ".exs": true, ".erl": true, ".hs": true,
		".proto": true, ".graphql": true, ".gql": true,
	}
	return sourceExts[ext]
}
