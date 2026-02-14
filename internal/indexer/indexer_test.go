package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/ziadkadry99/auto-doc/internal/config"
	"github.com/ziadkadry99/auto-doc/internal/llm"
	"github.com/ziadkadry99/auto-doc/internal/vectordb"
	"github.com/ziadkadry99/auto-doc/internal/walker"
)

// --- Mock LLM Provider ---

type mockProvider struct {
	response *llm.CompletionResponse
	err      error
	calls    atomic.Int64
}

func (m *mockProvider) Complete(_ context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	m.calls.Add(1)
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *mockProvider) Name() string { return "mock" }

// --- Mock Vector Store ---

type mockStore struct {
	docs    []vectordb.Document
	deleted []string
}

func (m *mockStore) AddDocuments(_ context.Context, docs []vectordb.Document) error {
	m.docs = append(m.docs, docs...)
	return nil
}

func (m *mockStore) Search(_ context.Context, _ string, _ int, _ *vectordb.SearchFilter) ([]vectordb.SearchResult, error) {
	return nil, nil
}

func (m *mockStore) GetByFilePath(_ context.Context, path string) ([]vectordb.Document, error) {
	var result []vectordb.Document
	for _, d := range m.docs {
		if d.Metadata.FilePath == path {
			result = append(result, d)
		}
	}
	return result, nil
}

func (m *mockStore) DeleteByFilePath(_ context.Context, path string) error {
	m.deleted = append(m.deleted, path)
	var remaining []vectordb.Document
	for _, d := range m.docs {
		if d.Metadata.FilePath != path {
			remaining = append(remaining, d)
		}
	}
	m.docs = remaining
	return nil
}

func (m *mockStore) Persist(_ context.Context, _ string) error { return nil }
func (m *mockStore) Load(_ context.Context, _ string) error    { return nil }
func (m *mockStore) Count() int                                { return len(m.docs) }

// --- Mock Embedder ---

type mockEmbedder struct{}

func (m *mockEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i := range texts {
		result[i] = make([]float32, 128)
	}
	return result, nil
}

func (m *mockEmbedder) Dimensions() int { return 128 }
func (m *mockEmbedder) Name() string    { return "mock" }

// --- Tests ---

func TestParseAnalysis_ValidJSON(t *testing.T) {
	raw := `{
		"summary": "This file implements a REST API handler.",
		"purpose": "Handles HTTP requests for user management.",
		"functions": [
			{
				"name": "HandleCreate",
				"signature": "func HandleCreate(w http.ResponseWriter, r *http.Request)",
				"summary": "Creates a new user",
				"parameters": [{"name": "w", "type": "http.ResponseWriter", "description": "response writer"}],
				"returns": "void",
				"line_start": 10,
				"line_end": 30
			}
		],
		"classes": [
			{
				"name": "UserService",
				"summary": "Manages user CRUD operations",
				"fields": [{"name": "db", "type": "*sql.DB", "description": "database connection"}]
			}
		],
		"dependencies": [{"name": "net/http", "type": "import"}, {"name": "postgres", "type": "database"}],
		"key_logic": ["Uses bcrypt for password hashing"]
	}`

	analysis, err := parseAnalysis(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if analysis.Summary == "" {
		t.Error("expected non-empty summary")
	}
	if len(analysis.Functions) != 1 {
		t.Errorf("expected 1 function, got %d", len(analysis.Functions))
	}
	if analysis.Functions[0].Name != "HandleCreate" {
		t.Errorf("expected function name HandleCreate, got %s", analysis.Functions[0].Name)
	}
	if len(analysis.Classes) != 1 {
		t.Errorf("expected 1 class, got %d", len(analysis.Classes))
	}
	if len(analysis.Dependencies) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(analysis.Dependencies))
	}
	if len(analysis.KeyLogic) != 1 {
		t.Errorf("expected 1 key_logic entry, got %d", len(analysis.KeyLogic))
	}
}

func TestParseAnalysis_WithCodeFences(t *testing.T) {
	raw := "```json\n{\"summary\": \"A helper module.\", \"purpose\": \"Utility functions.\"}\n```"
	analysis, err := parseAnalysis(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if analysis.Summary != "A helper module." {
		t.Errorf("expected 'A helper module.', got '%s'", analysis.Summary)
	}
}

func TestParseAnalysis_MalformedJSON(t *testing.T) {
	raw := `{this is not valid json}`
	_, err := parseAnalysis(raw)
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestBuildMessages_Lite(t *testing.T) {
	msgs := buildMessages(config.QualityLite, "main.go", "package main", "Go")
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != llm.RoleSystem {
		t.Error("first message should be system role")
	}
	if msgs[1].Role != llm.RoleUser {
		t.Error("second message should be user role")
	}
}

func TestBuildMessages_Normal(t *testing.T) {
	msgs := buildMessages(config.QualityNormal, "main.go", "package main", "Go")
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
}

func TestBuildMessages_Max(t *testing.T) {
	msgs := buildMessages(config.QualityMax, "main.go", "package main", "Go")
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
}

func TestChunkAnalysis_FileOnly_Lite(t *testing.T) {
	analysis := &FileAnalysis{
		FilePath:    "main.go",
		Language:    "Go",
		Summary:     "Main entry point.",
		Purpose:     "Starts the application.",
		ContentHash: "abc123",
		Functions: []FunctionDoc{
			{Name: "main", Summary: "Entry point"},
		},
	}

	docs := ChunkAnalysis(analysis, config.QualityLite)
	if len(docs) != 1 {
		t.Errorf("lite tier should produce 1 doc (file only), got %d", len(docs))
	}
	if docs[0].Metadata.Type != vectordb.DocTypeFile {
		t.Errorf("expected file type, got %s", docs[0].Metadata.Type)
	}
}

func TestChunkAnalysis_Normal(t *testing.T) {
	analysis := &FileAnalysis{
		FilePath:    "handler.go",
		Language:    "Go",
		Summary:     "HTTP handler.",
		ContentHash: "def456",
		Functions: []FunctionDoc{
			{Name: "HandleGet", Summary: "Handles GET requests"},
			{Name: "HandlePost", Summary: "Handles POST requests"},
		},
		Classes: []ClassDoc{
			{Name: "Handler", Summary: "HTTP handler struct"},
		},
	}

	docs := ChunkAnalysis(analysis, config.QualityNormal)
	// 1 file + 2 functions + 1 class = 4
	if len(docs) != 4 {
		t.Errorf("expected 4 docs, got %d", len(docs))
	}

	typeCount := map[vectordb.DocumentType]int{}
	for _, d := range docs {
		typeCount[d.Metadata.Type]++
	}
	if typeCount[vectordb.DocTypeFile] != 1 {
		t.Error("expected 1 file doc")
	}
	if typeCount[vectordb.DocTypeFunction] != 2 {
		t.Error("expected 2 function docs")
	}
	if typeCount[vectordb.DocTypeClass] != 1 {
		t.Error("expected 1 class doc")
	}
}

func TestSplitLargeFile(t *testing.T) {
	// Create content that exceeds max tokens.
	var lines []string
	for i := 0; i < 1000; i++ {
		lines = append(lines, fmt.Sprintf("line %d: some code content here that fills the file", i))
	}
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}

	// Use a small max tokens so content will be split.
	chunks := SplitLargeFile(content, 500)
	if len(chunks) < 2 {
		t.Errorf("expected multiple chunks, got %d", len(chunks))
	}

	// All content should be preserved.
	rejoined := ""
	for i, c := range chunks {
		if i > 0 {
			rejoined += "\n"
		}
		rejoined += c
	}
	if len(rejoined) < len(content)-10 {
		t.Error("chunks lost significant content")
	}
}

func TestSplitLargeFile_Small(t *testing.T) {
	chunks := SplitLargeFile("small file", 1000)
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk for small file, got %d", len(chunks))
	}
}

func TestState_LoadSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()

	state := &IndexState{
		LastCommitSHA: "abc123",
		FileHashes:    map[string]string{"main.go": "hash1", "lib.go": "hash2"},
	}

	if err := state.SaveState(dir); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadState(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if loaded.LastCommitSHA != "abc123" {
		t.Errorf("expected commit sha abc123, got %s", loaded.LastCommitSHA)
	}
	if len(loaded.FileHashes) != 2 {
		t.Errorf("expected 2 file hashes, got %d", len(loaded.FileHashes))
	}
	if loaded.FileHashes["main.go"] != "hash1" {
		t.Errorf("expected hash1 for main.go, got %s", loaded.FileHashes["main.go"])
	}
}

func TestState_LoadNonexistent(t *testing.T) {
	dir := t.TempDir()
	state, err := LoadState(dir)
	if err != nil {
		t.Fatalf("expected no error for nonexistent state, got: %v", err)
	}
	if state.FileHashes == nil {
		t.Error("expected initialized FileHashes map")
	}
}

func TestState_IsFileChanged(t *testing.T) {
	state := &IndexState{
		FileHashes: map[string]string{"main.go": "hash1"},
	}

	if !state.IsFileChanged("new.go", "whatever") {
		t.Error("new file should be reported as changed")
	}
	if !state.IsFileChanged("main.go", "hash2") {
		t.Error("file with different hash should be reported as changed")
	}
	if state.IsFileChanged("main.go", "hash1") {
		t.Error("file with same hash should not be reported as changed")
	}
}

func TestAnalyzer_Analyze(t *testing.T) {
	analysisJSON, _ := json.Marshal(FileAnalysis{
		Summary:  "A test file.",
		Purpose:  "Testing.",
		KeyLogic: []string{"Uses mocks"},
	})

	provider := &mockProvider{
		response: &llm.CompletionResponse{
			Content:      string(analysisJSON),
			InputTokens:  100,
			OutputTokens: 50,
		},
	}

	analyzer := NewFileAnalyzer(provider, config.QualityLite, "test-model")
	result, err := analyzer.Analyze(context.Background(), "test.go", []byte("package test"), "Go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Analysis.Summary != "A test file." {
		t.Errorf("expected 'A test file.', got '%s'", result.Analysis.Summary)
	}
	if result.InputTokens != 100 {
		t.Errorf("expected 100 input tokens, got %d", result.InputTokens)
	}
	if result.Analysis.ContentHash == "" {
		t.Error("expected content hash to be set")
	}
}

func TestAnalyzer_FallbackOnBadJSON(t *testing.T) {
	callCount := 0
	provider := &mockProvider{}
	provider.response = &llm.CompletionResponse{
		Content:      "not json at all",
		InputTokens:  100,
		OutputTokens: 50,
	}
	// Override Complete to return different responses.
	customProvider := &sequentialProvider{
		responses: []*llm.CompletionResponse{
			{Content: "not json at all", InputTokens: 100, OutputTokens: 50},
			{Content: `{"summary": "Fallback summary.", "purpose": "Fallback."}`, InputTokens: 50, OutputTokens: 25},
		},
	}
	_ = callCount

	analyzer := NewFileAnalyzer(customProvider, config.QualityNormal, "test-model")
	result, err := analyzer.Analyze(context.Background(), "test.go", []byte("package test"), "Go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Analysis.Summary != "Fallback summary." {
		t.Errorf("expected fallback summary, got '%s'", result.Analysis.Summary)
	}
	if customProvider.callIdx != 2 {
		t.Errorf("expected 2 LLM calls (original + fallback), got %d", customProvider.callIdx)
	}
}

// sequentialProvider returns different responses on successive calls.
type sequentialProvider struct {
	responses []*llm.CompletionResponse
	callIdx   int
}

func (s *sequentialProvider) Complete(_ context.Context, _ llm.CompletionRequest) (*llm.CompletionResponse, error) {
	if s.callIdx >= len(s.responses) {
		return nil, fmt.Errorf("no more mock responses")
	}
	resp := s.responses[s.callIdx]
	s.callIdx++
	return resp, nil
}

func (s *sequentialProvider) Name() string { return "sequential-mock" }

func TestBatcher_ProcessFiles(t *testing.T) {
	// Create temp files.
	dir := t.TempDir()
	for i := 0; i < 3; i++ {
		path := filepath.Join(dir, fmt.Sprintf("file%d.go", i))
		os.WriteFile(path, []byte(fmt.Sprintf("package f%d", i)), 0o644)
	}

	analysisJSON := `{"summary": "A file.", "purpose": "Testing."}`
	provider := &mockProvider{
		response: &llm.CompletionResponse{
			Content:      analysisJSON,
			InputTokens:  100,
			OutputTokens: 50,
		},
	}

	analyzer := NewFileAnalyzer(provider, config.QualityLite, "test-model")
	var progressCalls atomic.Int64
	batcher := NewBatcher(2, analyzer, func(processed, total int, file string) {
		progressCalls.Add(1)
	})

	files := make([]walker.FileInfo, 3)
	for i := 0; i < 3; i++ {
		files[i] = walker.FileInfo{
			Path:     filepath.Join(dir, fmt.Sprintf("file%d.go", i)),
			RelPath:  fmt.Sprintf("file%d.go", i),
			Language: "Go",
		}
	}

	result := batcher.ProcessFiles(context.Background(), files)
	if len(result.Results) != 3 {
		t.Errorf("expected 3 results, got %d", len(result.Results))
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d: %v", len(result.Errors), result.Errors)
	}
	if progressCalls.Load() != 3 {
		t.Errorf("expected 3 progress calls, got %d", progressCalls.Load())
	}
}

func TestPipeline_Run(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "main.go")
	os.WriteFile(filePath, []byte("package main"), 0o644)

	analysisJSON := `{"summary": "Main entry.", "purpose": "Entry point."}`
	provider := &mockProvider{
		response: &llm.CompletionResponse{
			Content:      analysisJSON,
			InputTokens:  100,
			OutputTokens: 50,
		},
	}

	store := &mockStore{}
	embedder := &mockEmbedder{}
	cfg := &config.Config{
		Quality:        config.QualityLite,
		Model:          "test-model",
		MaxConcurrency: 2,
	}

	pipeline := NewPipeline(provider, embedder, store, cfg, dir)
	files := []walker.FileInfo{
		{
			Path:        filePath,
			RelPath:     "main.go",
			Language:    "Go",
			ContentHash: "somehash",
		},
	}

	result, err := pipeline.Run(context.Background(), files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FilesProcessed != 1 {
		t.Errorf("expected 1 file processed, got %d", result.FilesProcessed)
	}
	if result.FilesSkipped != 0 {
		t.Errorf("expected 0 files skipped, got %d", result.FilesSkipped)
	}

	// Run again — should skip the file.
	result2, err := pipeline.Run(context.Background(), files)
	if err != nil {
		t.Fatalf("unexpected error on re-run: %v", err)
	}
	if result2.FilesSkipped != 1 {
		t.Errorf("expected 1 file skipped on re-run, got %d", result2.FilesSkipped)
	}
	if result2.FilesProcessed != 0 {
		t.Errorf("expected 0 files processed on re-run, got %d", result2.FilesProcessed)
	}
}

func TestPipeline_DryRun(t *testing.T) {
	dir := t.TempDir()
	provider := &mockProvider{}
	store := &mockStore{}
	embedder := &mockEmbedder{}
	cfg := &config.Config{
		Quality:        config.QualityNormal,
		Model:          "test-model",
		MaxConcurrency: 2,
	}

	pipeline := NewPipeline(provider, embedder, store, cfg, dir)
	files := []walker.FileInfo{
		{RelPath: "a.go", Size: 4000, ContentHash: "h1"},
		{RelPath: "b.go", Size: 2000, ContentHash: "h2"},
	}

	estimate, err := pipeline.DryRun(context.Background(), files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if estimate.TotalFiles != 2 {
		t.Errorf("expected 2 total files, got %d", estimate.TotalFiles)
	}
	if estimate.TotalTokensEstimate <= 0 {
		t.Error("expected positive token estimate")
	}
	if estimate.EstimatedCost <= 0 {
		t.Error("expected positive cost estimate")
	}
	if _, ok := estimate.CostBreakdown["analysis"]; !ok {
		t.Error("expected analysis cost in breakdown")
	}
	if _, ok := estimate.CostBreakdown["embeddings"]; !ok {
		t.Error("expected embeddings cost in breakdown")
	}
}

func TestComputeHash(t *testing.T) {
	h1 := computeHash([]byte("hello"))
	h2 := computeHash([]byte("hello"))
	h3 := computeHash([]byte("world"))

	if h1 != h2 {
		t.Error("same content should produce same hash")
	}
	if h1 == h3 {
		t.Error("different content should produce different hash")
	}
	if len(h1) != 64 {
		t.Errorf("expected 64-char hex hash, got %d chars", len(h1))
	}
}
