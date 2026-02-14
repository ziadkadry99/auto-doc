package site

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strings"

	"github.com/ziadkadry99/auto-doc/internal/llm"
	"github.com/ziadkadry99/auto-doc/internal/vectordb"
)

// Serve starts a local HTTP file server for the static site.
// If store is non-nil, an /api/search endpoint is available for semantic search.
// If llmProvider is non-nil, search results include LLM-synthesized answers.
func Serve(dir string, port int, open bool, store vectordb.VectorStore, llmProvider llm.Provider, model string) error {
	addr := fmt.Sprintf(":%d", port)
	url := fmt.Sprintf("http://localhost:%d", port)

	if open {
		go openBrowser(url)
	}

	fmt.Printf("Serving documentation at %s\n", url)
	fmt.Println("Press Ctrl+C to stop.")

	mux := http.NewServeMux()

	// API endpoint for semantic search.
	if store != nil {
		mux.HandleFunc("/api/search", func(w http.ResponseWriter, r *http.Request) {
			handleSearch(w, r, store, llmProvider, model)
		})
	}

	// Static files (must be registered after API routes).
	fs := http.FileServer(http.Dir(dir))
	mux.Handle("/", fs)

	return http.ListenAndServe(addr, mux)
}

// searchRequest is the JSON body for the /api/search endpoint.
type searchRequest struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

// searchResponse is the JSON response for the /api/search endpoint.
type searchResponse struct {
	Answer  string               `json:"answer,omitempty"`
	Results []searchResponseItem `json:"results"`
}

// searchResponseItem is one result in the /api/search response.
type searchResponseItem struct {
	FilePath   string  `json:"file_path"`
	Symbol     string  `json:"symbol,omitempty"`
	Type       string  `json:"type"`
	Language   string  `json:"language,omitempty"`
	Similarity float64 `json:"similarity"`
	Content    string  `json:"content"`
	LineStart  int     `json:"line_start,omitempty"`
}

func handleSearch(w http.ResponseWriter, r *http.Request, store vectordb.VectorStore, llmProvider llm.Provider, model string) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req searchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	query := strings.TrimSpace(req.Query)
	if query == "" {
		http.Error(w, `{"error":"query is required"}`, http.StatusBadRequest)
		return
	}

	limit := req.Limit
	if limit <= 0 || limit > 20 {
		limit = 8
	}

	ctx := context.Background()
	results, err := store.Search(ctx, query, limit, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"search failed: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	items := make([]searchResponseItem, len(results))
	for i, r := range results {
		content := r.Document.Content
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		items[i] = searchResponseItem{
			FilePath:   r.Document.Metadata.FilePath,
			Symbol:     r.Document.Metadata.Symbol,
			Type:       string(r.Document.Metadata.Type),
			Language:   r.Document.Metadata.Language,
			Similarity: float64(r.Similarity),
			Content:    content,
			LineStart:  r.Document.Metadata.LineStart,
		}
	}

	resp := searchResponse{Results: items}

	// Synthesize an LLM answer if provider is available.
	if llmProvider != nil && len(results) > 0 {
		answer := synthesizeAnswer(ctx, llmProvider, model, query, results)
		if answer != "" {
			resp.Answer = answer
		}
	}

	json.NewEncoder(w).Encode(resp)
}

// synthesizeAnswer sends the query and search results to the LLM for a coherent answer.
func synthesizeAnswer(ctx context.Context, provider llm.Provider, model string, query string, results []vectordb.SearchResult) string {
	resultsContext := vectordb.FormatResults(results)

	prompt := fmt.Sprintf(`A developer is exploring documentation for this codebase and asked: "%s"

Here are the most relevant documentation excerpts found via semantic search:

%s

Write a thorough answer (3-8 sentences) that:
1. Directly answers the question with specific details — list all items, name all components, include key facts.
2. Provides broader context about how this fits into the project's architecture.
3. References specific file paths in backticks (e.g. `+"`"+`path/to/file.go`+"`"+`).
4. Never cut off a list — always complete it. If there are 4 items, name all 4.

Be factual and grounded in the provided context.`, query, resultsContext)

	resp, err := provider.Complete(ctx, llm.CompletionRequest{
		Model: model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "You are a knowledgeable code documentation assistant. You provide complete, detailed answers about the codebase. Always finish lists and enumerate all items. Reference file paths in backticks. Give architectural context when relevant."},
			{Role: llm.RoleUser, Content: prompt},
		},
		MaxTokens:   1024,
		Temperature: 0.3,
	})
	if err != nil {
		return ""
	}

	return strings.TrimSpace(resp.Content)
}

// openBrowser opens the given URL in the default browser.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
