package contextengine

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/ziadkadry99/auto-doc/internal/db"
)

func setupTestStore(t *testing.T) *Store {
	t.Helper()
	database, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return NewStore(database)
}

func TestSaveFact(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	fact := Fact{
		RepoID:     "my-repo",
		Scope:      "service",
		ScopeID:    "payment-service",
		Key:        "description",
		Value:      "Handles payment processing",
		Source:     "user",
		ProvidedBy: "alice@example.com",
	}

	saved, err := store.SaveFact(ctx, fact)
	if err != nil {
		t.Fatalf("SaveFact: %v", err)
	}
	if saved.ID == "" {
		t.Error("expected non-empty ID")
	}
	if saved.Version != 1 {
		t.Errorf("expected version 1, got %d", saved.Version)
	}
}

func TestFactVersioning(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	// Save initial fact.
	f1 := Fact{
		RepoID: "repo", Scope: "service", ScopeID: "svc", Key: "desc",
		Value: "version 1", Source: "user", ProvidedBy: "alice",
	}
	saved1, err := store.SaveFact(ctx, f1)
	if err != nil {
		t.Fatalf("SaveFact v1: %v", err)
	}

	// Save updated fact with same key.
	f2 := Fact{
		RepoID: "repo", Scope: "service", ScopeID: "svc", Key: "desc",
		Value: "version 2", Source: "user", ProvidedBy: "bob",
	}
	saved2, err := store.SaveFact(ctx, f2)
	if err != nil {
		t.Fatalf("SaveFact v2: %v", err)
	}

	if saved2.Version != 2 {
		t.Errorf("expected version 2, got %d", saved2.Version)
	}

	// Original should be superseded.
	original, err := store.GetFact(ctx, saved1.ID)
	if err != nil {
		t.Fatalf("GetFact: %v", err)
	}
	if original.SupersededBy != saved2.ID {
		t.Errorf("expected superseded_by=%s, got %s", saved2.ID, original.SupersededBy)
	}

	// GetCurrentFacts should only return v2.
	current, err := store.GetCurrentFacts(ctx, "repo", "service", "svc")
	if err != nil {
		t.Fatalf("GetCurrentFacts: %v", err)
	}
	if len(current) != 1 {
		t.Fatalf("expected 1 current fact, got %d", len(current))
	}
	if current[0].Value != "version 2" {
		t.Errorf("expected 'version 2', got %q", current[0].Value)
	}
}

func TestFactHistory(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	for i, val := range []string{"v1", "v2", "v3"} {
		_, err := store.SaveFact(ctx, Fact{
			RepoID: "repo", Scope: "service", ScopeID: "svc", Key: "desc",
			Value: val, Source: "user", ProvidedBy: "user",
		})
		if err != nil {
			t.Fatalf("SaveFact[%d]: %v", i, err)
		}
	}

	history, err := store.GetFactHistory(ctx, "repo", "service", "svc", "desc")
	if err != nil {
		t.Fatalf("GetFactHistory: %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("expected 3 versions, got %d", len(history))
	}
	if history[0].Value != "v1" || history[2].Value != "v3" {
		t.Error("history not in expected order")
	}
}

func TestSearchFacts(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	store.SaveFact(ctx, Fact{
		RepoID: "repo", Scope: "service", ScopeID: "payment", Key: "desc",
		Value: "Handles payment processing and refunds", Source: "user", ProvidedBy: "user",
	})
	store.SaveFact(ctx, Fact{
		RepoID: "repo", Scope: "service", ScopeID: "order", Key: "desc",
		Value: "Manages order lifecycle", Source: "user", ProvidedBy: "user",
	})

	results, err := store.SearchFacts(ctx, "payment", 10)
	if err != nil {
		t.Fatalf("SearchFacts: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ScopeID != "payment" {
		t.Errorf("expected scope_id 'payment', got %q", results[0].ScopeID)
	}
}

func TestSessionsAndMessages(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	sess, err := store.CreateSession(ctx, "alice")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if sess.ID == "" {
		t.Error("expected non-empty session ID")
	}

	_, err = store.AddMessage(ctx, ConversationMessage{
		SessionID: sess.ID, Role: "user", Content: "Hello",
	})
	if err != nil {
		t.Fatalf("AddMessage: %v", err)
	}
	_, err = store.AddMessage(ctx, ConversationMessage{
		SessionID: sess.ID, Role: "assistant", Content: "Hi there!",
	})
	if err != nil {
		t.Fatalf("AddMessage: %v", err)
	}

	messages, err := store.GetMessages(ctx, sess.ID)
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Role != "user" || messages[1].Role != "assistant" {
		t.Error("messages not in expected order")
	}
}

func TestParseExtractionResponse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantN   int // number of facts
	}{
		{
			name: "valid JSON",
			input: `{"facts":[{"scope":"service","scope_id":"svc","key":"desc","value":"test","confidence":"high","explanation":"from input"}],"clarifications":[],"affected_docs":["svc"],"summary":"got it"}`,
			wantN: 1,
		},
		{
			name:  "JSON in markdown code block",
			input: "```json\n{\"facts\":[],\"summary\":\"nothing\"}\n```",
			wantN: 0,
		},
		{
			name:  "invalid JSON returns summary",
			input: "I couldn't understand that",
			wantN: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			update, err := parseExtractionResponse(tt.input)
			if err != nil {
				t.Fatalf("parseExtractionResponse: %v", err)
			}
			if len(update.Facts) != tt.wantN {
				t.Errorf("expected %d facts, got %d", tt.wantN, len(update.Facts))
			}
		})
	}
}

func TestRoutes_ListFacts(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	store.SaveFact(ctx, Fact{
		RepoID: "repo", Scope: "service", ScopeID: "svc", Key: "desc",
		Value: "test value", Source: "user", ProvidedBy: "user",
	})

	// We need a dummy engine (LLM not needed for list).
	engine := &Engine{store: store}

	r := chi.NewRouter()
	RegisterRoutes(r, engine)

	req := httptest.NewRequest("GET", "/api/context/facts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var facts []Fact
	if err := json.Unmarshal(w.Body.Bytes(), &facts); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(facts) != 1 {
		t.Errorf("expected 1 fact, got %d", len(facts))
	}
}

func TestRoutes_GetFact(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	saved, _ := store.SaveFact(ctx, Fact{
		RepoID: "repo", Scope: "service", ScopeID: "svc", Key: "desc",
		Value: "test", Source: "user", ProvidedBy: "user",
	})

	engine := &Engine{store: store}
	r := chi.NewRouter()
	RegisterRoutes(r, engine)

	req := httptest.NewRequest("GET", "/api/context/facts/"+saved.ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRoutes_GetFactNotFound(t *testing.T) {
	store := setupTestStore(t)
	engine := &Engine{store: store}

	r := chi.NewRouter()
	RegisterRoutes(r, engine)

	req := httptest.NewRequest("GET", "/api/context/facts/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestRoutes_CreateSession(t *testing.T) {
	store := setupTestStore(t)
	engine := &Engine{store: store}

	r := chi.NewRouter()
	RegisterRoutes(r, engine)

	body := strings.NewReader(`{"user_id":"alice"}`)
	req := httptest.NewRequest("POST", "/api/context/sessions", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var sess Session
	if err := json.Unmarshal(w.Body.Bytes(), &sess); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sess.ID == "" {
		t.Error("expected non-empty session ID")
	}
}

func TestRoutes_SearchFacts(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	store.SaveFact(ctx, Fact{
		RepoID: "repo", Scope: "service", ScopeID: "payment", Key: "desc",
		Value: "payment processing", Source: "user", ProvidedBy: "user",
	})

	engine := &Engine{store: store}
	r := chi.NewRouter()
	RegisterRoutes(r, engine)

	req := httptest.NewRequest("GET", "/api/context/facts/search?q=payment", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var facts []Fact
	if err := json.Unmarshal(w.Body.Bytes(), &facts); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(facts) != 1 {
		t.Errorf("expected 1 fact, got %d", len(facts))
	}
}
