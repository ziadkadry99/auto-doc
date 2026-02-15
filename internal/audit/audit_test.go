package audit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ziadkadry99/auto-doc/internal/db"
)

func setupStore(t *testing.T) *Store {
	t.Helper()
	database, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return NewStore(database)
}

func TestLogAndGetByID(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	entry := Entry{
		ID:               "test-1",
		ActorType:        ActorUser,
		ActorID:          "alice",
		Action:           ActionDocCreated,
		Scope:            ScopeService,
		ScopeID:          "auth-svc",
		Summary:          "Created auth service docs",
		Detail:           "Initial documentation for auth service",
		SourceFact:       "fact-42",
		AffectedEntities: []string{"auth-svc", "gateway"},
		ConversationID:   "conv-1",
		PreviousValue:    "",
		NewValue:         "new doc content",
	}

	if err := store.Log(ctx, entry); err != nil {
		t.Fatalf("Log: %v", err)
	}

	got, err := store.GetByID(ctx, "test-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	if got.ActorID != "alice" {
		t.Errorf("ActorID = %q, want %q", got.ActorID, "alice")
	}
	if got.Action != ActionDocCreated {
		t.Errorf("Action = %q, want %q", got.Action, ActionDocCreated)
	}
	if got.Scope != ScopeService {
		t.Errorf("Scope = %q, want %q", got.Scope, ScopeService)
	}
	if got.ScopeID != "auth-svc" {
		t.Errorf("ScopeID = %q, want %q", got.ScopeID, "auth-svc")
	}
	if got.SourceFact != "fact-42" {
		t.Errorf("SourceFact = %q, want %q", got.SourceFact, "fact-42")
	}
	if got.ConversationID != "conv-1" {
		t.Errorf("ConversationID = %q, want %q", got.ConversationID, "conv-1")
	}
	if got.NewValue != "new doc content" {
		t.Errorf("NewValue = %q, want %q", got.NewValue, "new doc content")
	}
	if len(got.AffectedEntities) != 2 || got.AffectedEntities[0] != "auth-svc" {
		t.Errorf("AffectedEntities = %v, want [auth-svc gateway]", got.AffectedEntities)
	}
}

func TestLogGeneratesUUID(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	entry := Entry{
		ActorType: ActorSystem,
		ActorID:   "indexer",
		Action:    ActionDocUpdated,
		Scope:     ScopeService,
		ScopeID:   "api-svc",
		Summary:   "Auto-updated docs",
	}

	if err := store.Log(ctx, entry); err != nil {
		t.Fatalf("Log: %v", err)
	}

	// Verify we can find it with a query.
	entries, err := store.Query(ctx, QueryFilter{ActorID: "indexer"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].ID == "" {
		t.Error("expected generated ID, got empty string")
	}
}

func TestQueryFilterByActor(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	for _, actor := range []string{"alice", "bob", "alice"} {
		if err := store.Log(ctx, Entry{
			ActorType: ActorUser,
			ActorID:   actor,
			Action:    ActionContextProvided,
			Scope:     ScopeOrg,
		}); err != nil {
			t.Fatalf("Log: %v", err)
		}
	}

	entries, err := store.Query(ctx, QueryFilter{ActorID: "alice"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries for alice, got %d", len(entries))
	}
}

func TestQueryFilterByScope(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	scopes := []Scope{ScopeService, ScopeEndpoint, ScopeService}
	for _, sc := range scopes {
		if err := store.Log(ctx, Entry{
			ActorType: ActorBot,
			ActorID:   "bot-1",
			Action:    ActionDocUpdated,
			Scope:     sc,
		}); err != nil {
			t.Fatalf("Log: %v", err)
		}
	}

	entries, err := store.Query(ctx, QueryFilter{Scope: ScopeService})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 service-scope entries, got %d", len(entries))
	}
}

func TestQueryFilterByAction(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	actions := []Action{ActionDocCreated, ActionDocUpdated, ActionDocCreated}
	for _, a := range actions {
		if err := store.Log(ctx, Entry{
			ActorType: ActorUser,
			ActorID:   "alice",
			Action:    a,
			Scope:     ScopeService,
		}); err != nil {
			t.Fatalf("Log: %v", err)
		}
	}

	entries, err := store.Query(ctx, QueryFilter{Action: ActionDocCreated})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 doc_created entries, got %d", len(entries))
	}
}

func TestQueryFilterByAffectedService(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	if err := store.Log(ctx, Entry{
		ActorType:        ActorUser,
		ActorID:          "alice",
		Action:           ActionRelationshipAdded,
		Scope:            ScopeService,
		AffectedEntities: []string{"auth-svc", "gateway"},
	}); err != nil {
		t.Fatalf("Log: %v", err)
	}

	if err := store.Log(ctx, Entry{
		ActorType:        ActorUser,
		ActorID:          "bob",
		Action:           ActionRelationshipAdded,
		Scope:            ScopeService,
		AffectedEntities: []string{"billing-svc"},
	}); err != nil {
		t.Fatalf("Log: %v", err)
	}

	entries, err := store.Query(ctx, QueryFilter{AffectedService: "auth-svc"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry affecting auth-svc, got %d", len(entries))
	}
}

func TestQueryLimitOffset(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if err := store.Log(ctx, Entry{
			ActorType: ActorUser,
			ActorID:   "alice",
			Action:    ActionDocUpdated,
			Scope:     ScopeService,
		}); err != nil {
			t.Fatalf("Log: %v", err)
		}
	}

	entries, err := store.Query(ctx, QueryFilter{Limit: 2})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries with limit, got %d", len(entries))
	}

	entries, err = store.Query(ctx, QueryFilter{Limit: 2, Offset: 3})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries with offset, got %d", len(entries))
	}
}

func TestDeleteBefore(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	// Insert 3 entries.
	for i := 0; i < 3; i++ {
		if err := store.Log(ctx, Entry{
			ActorType: ActorSystem,
			ActorID:   "system",
			Action:    ActionDocUpdated,
			Scope:     ScopeService,
		}); err != nil {
			t.Fatalf("Log: %v", err)
		}
	}

	// Delete entries before far in the future (should delete all).
	deleted, err := store.DeleteBefore(ctx, time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("DeleteBefore: %v", err)
	}
	if deleted != 3 {
		t.Errorf("expected 3 deleted, got %d", deleted)
	}

	entries, err := store.Query(ctx, QueryFilter{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 remaining entries, got %d", len(entries))
	}
}

func TestGetByIDNotFound(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	_, err := store.GetByID(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent ID, got nil")
	}
}

// --- HTTP handler tests ---

func setupRouter(t *testing.T) (chi.Router, *Store) {
	t.Helper()
	store := setupStore(t)
	r := chi.NewRouter()
	RegisterRoutes(r, store)
	return r, store
}

func TestHTTPGetByID(t *testing.T) {
	r, store := setupRouter(t)
	ctx := context.Background()

	entry := Entry{
		ID:               "http-1",
		ActorType:        ActorUser,
		ActorID:          "alice",
		Action:           ActionDocCreated,
		Scope:            ScopeService,
		ScopeID:          "auth-svc",
		Summary:          "Created docs",
		AffectedEntities: []string{"auth-svc"},
	}
	if err := store.Log(ctx, entry); err != nil {
		t.Fatalf("Log: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/audit/http-1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var got Entry
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != "http-1" {
		t.Errorf("ID = %q, want %q", got.ID, "http-1")
	}
	if got.ActorID != "alice" {
		t.Errorf("ActorID = %q, want %q", got.ActorID, "alice")
	}
}

func TestHTTPGetByIDNotFound(t *testing.T) {
	r, _ := setupRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/audit/missing", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHTTPQueryAll(t *testing.T) {
	r, store := setupRouter(t)
	ctx := context.Background()

	for _, actor := range []string{"alice", "bob"} {
		if err := store.Log(ctx, Entry{
			ActorType: ActorUser,
			ActorID:   actor,
			Action:    ActionContextProvided,
			Scope:     ScopeOrg,
		}); err != nil {
			t.Fatalf("Log: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/audit", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var entries []Entry
	if err := json.NewDecoder(rec.Body).Decode(&entries); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}

func TestHTTPQueryWithFilter(t *testing.T) {
	r, store := setupRouter(t)
	ctx := context.Background()

	for _, actor := range []string{"alice", "bob", "alice"} {
		if err := store.Log(ctx, Entry{
			ActorType: ActorUser,
			ActorID:   actor,
			Action:    ActionDocUpdated,
			Scope:     ScopeService,
		}); err != nil {
			t.Fatalf("Log: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/audit?actor=alice&limit=10", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var entries []Entry
	if err := json.NewDecoder(rec.Body).Decode(&entries); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries for alice, got %d", len(entries))
	}
}
