package confidence

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

func TestSetAndGet(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	meta := Metadata{
		EntityType:   EntityRelationship,
		EntityID:     "svc-a->svc-b",
		Confidence:   LevelAutoDetected,
		Source:       SourceCodeAnalysis,
		SourceDetail: "found import in main.go",
		LastVerified: now,
	}

	if err := store.Set(ctx, meta); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := store.Get(ctx, EntityRelationship, "svc-a->svc-b")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if got.Confidence != LevelAutoDetected {
		t.Errorf("Confidence = %q, want %q", got.Confidence, LevelAutoDetected)
	}
	if got.Source != SourceCodeAnalysis {
		t.Errorf("Source = %q, want %q", got.Source, SourceCodeAnalysis)
	}
	if got.SourceDetail != "found import in main.go" {
		t.Errorf("SourceDetail = %q, want %q", got.SourceDetail, "found import in main.go")
	}
	if got.ID == "" {
		t.Error("ID should be auto-generated")
	}
}

func TestSetUpsert(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	meta := Metadata{
		EntityType: EntityDescription,
		EntityID:   "svc-auth",
		Confidence: LevelAutoDetected,
		Source:     SourceCodeAnalysis,
	}
	if err := store.Set(ctx, meta); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Update the same entity with different confidence.
	meta.Confidence = LevelConfirmed
	meta.Source = SourceUserConversation
	if err := store.Set(ctx, meta); err != nil {
		t.Fatalf("Set upsert: %v", err)
	}

	got, err := store.Get(ctx, EntityDescription, "svc-auth")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Confidence != LevelConfirmed {
		t.Errorf("Confidence after upsert = %q, want %q", got.Confidence, LevelConfirmed)
	}
}

func TestGetNotFound(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	got, err := store.Get(ctx, EntityRelationship, "nonexistent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for nonexistent entity, got %+v", got)
	}
}

func TestList(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	entries := []Metadata{
		{EntityType: EntityRelationship, EntityID: "a->b", Confidence: LevelAutoDetected, Source: SourceCodeAnalysis},
		{EntityType: EntityRelationship, EntityID: "b->c", Confidence: LevelConfirmed, Source: SourceUserConversation},
		{EntityType: EntityDescription, EntityID: "svc-a", Confidence: LevelAutoDetected, Source: SourceCodeAnalysis},
	}
	for _, e := range entries {
		if err := store.Set(ctx, e); err != nil {
			t.Fatalf("Set: %v", err)
		}
	}

	// List all.
	all, err := store.List(ctx, ListFilter{})
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("List all: got %d, want 3", len(all))
	}

	// Filter by entity type.
	rels, err := store.List(ctx, ListFilter{EntityType: EntityRelationship})
	if err != nil {
		t.Fatalf("List relationships: %v", err)
	}
	if len(rels) != 2 {
		t.Errorf("List relationships: got %d, want 2", len(rels))
	}

	// Filter by confidence.
	confirmed, err := store.List(ctx, ListFilter{Confidence: LevelConfirmed})
	if err != nil {
		t.Fatalf("List confirmed: %v", err)
	}
	if len(confirmed) != 1 {
		t.Errorf("List confirmed: got %d, want 1", len(confirmed))
	}

	// Test limit and offset.
	limited, err := store.List(ctx, ListFilter{Limit: 1})
	if err != nil {
		t.Fatalf("List limited: %v", err)
	}
	if len(limited) != 1 {
		t.Errorf("List limited: got %d, want 1", len(limited))
	}
}

func TestMarkStale(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	meta := Metadata{
		EntityType: EntityRelationship,
		EntityID:   "a->b",
		Confidence: LevelAutoDetected,
		Source:     SourceCodeAnalysis,
	}
	if err := store.Set(ctx, meta); err != nil {
		t.Fatalf("Set: %v", err)
	}

	if err := store.MarkStale(ctx, EntityRelationship, "a->b", "code changed"); err != nil {
		t.Fatalf("MarkStale: %v", err)
	}

	got, err := store.Get(ctx, EntityRelationship, "a->b")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.PotentiallyStale {
		t.Error("expected PotentiallyStale to be true")
	}
	if got.StaleReason != "code changed" {
		t.Errorf("StaleReason = %q, want %q", got.StaleReason, "code changed")
	}

	// List stale only.
	stale, err := store.List(ctx, ListFilter{StaleOnly: true})
	if err != nil {
		t.Fatalf("List stale: %v", err)
	}
	if len(stale) != 1 {
		t.Errorf("List stale: got %d, want 1", len(stale))
	}
}

func TestMarkStaleNotFound(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	err := store.MarkStale(ctx, EntityRelationship, "nonexistent", "test")
	if err == nil {
		t.Error("expected error for nonexistent entity")
	}
}

func TestMarkVerified(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	meta := Metadata{
		EntityType:       EntityRelationship,
		EntityID:         "a->b",
		Confidence:       LevelAutoDetected,
		Source:           SourceCodeAnalysis,
		PotentiallyStale: true,
		StaleReason:      "code changed",
	}
	if err := store.Set(ctx, meta); err != nil {
		t.Fatalf("Set: %v", err)
	}

	if err := store.MarkVerified(ctx, EntityRelationship, "a->b"); err != nil {
		t.Fatalf("MarkVerified: %v", err)
	}

	got, err := store.Get(ctx, EntityRelationship, "a->b")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.PotentiallyStale {
		t.Error("expected PotentiallyStale to be false after MarkVerified")
	}
	if got.StaleReason != "" {
		t.Errorf("StaleReason = %q, want empty", got.StaleReason)
	}
}

func TestMarkVerifiedNotFound(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	err := store.MarkVerified(ctx, EntityRelationship, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent entity")
	}
}

func TestStats(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	entries := []Metadata{
		{EntityType: EntityRelationship, EntityID: "a->b", Confidence: LevelAutoDetected, Source: SourceCodeAnalysis},
		{EntityType: EntityRelationship, EntityID: "b->c", Confidence: LevelAutoDetected, Source: SourceCodeAnalysis, PotentiallyStale: true, StaleReason: "old"},
		{EntityType: EntityDescription, EntityID: "svc-a", Confidence: LevelConfirmed, Source: SourceUserConversation},
	}
	for _, e := range entries {
		if err := store.Set(ctx, e); err != nil {
			t.Fatalf("Set: %v", err)
		}
	}

	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.TotalEntities != 3 {
		t.Errorf("TotalEntities = %d, want 3", stats.TotalEntities)
	}
	if stats.ByConfidence[LevelAutoDetected] != 2 {
		t.Errorf("ByConfidence[auto_detected] = %d, want 2", stats.ByConfidence[LevelAutoDetected])
	}
	if stats.ByConfidence[LevelConfirmed] != 1 {
		t.Errorf("ByConfidence[confirmed] = %d, want 1", stats.ByConfidence[LevelConfirmed])
	}
	if stats.StaleCount != 1 {
		t.Errorf("StaleCount = %d, want 1", stats.StaleCount)
	}
}

func TestCheckStaleness(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name      string
		meta      Metadata
		changed   time.Time
		wantStale bool
		wantMsg   string
	}{
		{
			name: "code changed after verification",
			meta: Metadata{
				LastVerified: now.Add(-2 * time.Hour),
				Source:       SourceCodeAnalysis,
			},
			changed:   now.Add(-1 * time.Hour),
			wantStale: true,
			wantMsg:   "code changed after last verification",
		},
		{
			name: "code not changed",
			meta: Metadata{
				LastVerified: now,
				Source:       SourceCodeAnalysis,
			},
			changed:   now.Add(-1 * time.Hour),
			wantStale: false,
		},
		{
			name: "external source old verification",
			meta: Metadata{
				LastVerified: now.Add(-7 * 30 * 24 * time.Hour), // 7 months ago
				Source:       SourceConfluence,
			},
			changed:   time.Time{}, // zero
			wantStale: true,
			wantMsg:   "external source not re-verified for over 6 months",
		},
		{
			name: "external source recent verification",
			meta: Metadata{
				LastVerified: now.Add(-1 * 30 * 24 * time.Hour), // 1 month ago
				Source:       SourceConfluence,
			},
			changed:   time.Time{},
			wantStale: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stale, msg := CheckStaleness(tt.meta, tt.changed)
			if stale != tt.wantStale {
				t.Errorf("stale = %v, want %v", stale, tt.wantStale)
			}
			if tt.wantStale && msg != tt.wantMsg {
				t.Errorf("msg = %q, want %q", msg, tt.wantMsg)
			}
		})
	}
}

// HTTP handler tests.

func setupTestRouter(t *testing.T) (chi.Router, *Store) {
	t.Helper()
	store := setupTestStore(t)
	r := chi.NewRouter()
	RegisterRoutes(r, store)
	return r, store
}

func TestHTTPGetNotFound(t *testing.T) {
	r, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/confidence/relationship/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHTTPSetAndGet(t *testing.T) {
	r, _ := setupTestRouter(t)

	meta := Metadata{
		EntityType: EntityRelationship,
		EntityID:   "a->b",
		Confidence: LevelAutoDetected,
		Source:     SourceCodeAnalysis,
	}
	body, _ := json.Marshal(meta)

	// PUT
	req := httptest.NewRequest(http.MethodPut, "/api/confidence", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	// GET
	req = httptest.NewRequest(http.MethodGet, "/api/confidence/relationship/a-%3Eb", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var got Metadata
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Confidence != LevelAutoDetected {
		t.Errorf("Confidence = %q, want %q", got.Confidence, LevelAutoDetected)
	}
}

func TestHTTPList(t *testing.T) {
	r, store := setupTestRouter(t)
	ctx := context.Background()

	entries := []Metadata{
		{EntityType: EntityRelationship, EntityID: "a->b", Confidence: LevelAutoDetected, Source: SourceCodeAnalysis},
		{EntityType: EntityDescription, EntityID: "svc-a", Confidence: LevelConfirmed, Source: SourceUserConversation},
	}
	for _, e := range entries {
		store.Set(ctx, e)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/confidence?entity_type=relationship", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var results []Metadata
	if err := json.NewDecoder(w.Body).Decode(&results); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("got %d results, want 1", len(results))
	}
}

func TestHTTPStats(t *testing.T) {
	r, store := setupTestRouter(t)
	ctx := context.Background()

	store.Set(ctx, Metadata{EntityType: EntityRelationship, EntityID: "a->b", Confidence: LevelAutoDetected, Source: SourceCodeAnalysis})

	req := httptest.NewRequest(http.MethodGet, "/api/confidence/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var stats Stats
	if err := json.NewDecoder(w.Body).Decode(&stats); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if stats.TotalEntities != 1 {
		t.Errorf("TotalEntities = %d, want 1", stats.TotalEntities)
	}
}

func TestHTTPSetBadRequest(t *testing.T) {
	r, _ := setupTestRouter(t)

	// Missing required fields.
	body := `{"entity_type":"relationship"}`
	req := httptest.NewRequest(http.MethodPut, "/api/confidence", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	// Invalid JSON.
	req = httptest.NewRequest(http.MethodPut, "/api/confidence", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d for invalid JSON", w.Code, http.StatusBadRequest)
	}
}
