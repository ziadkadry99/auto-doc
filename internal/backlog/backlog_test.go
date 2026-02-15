package backlog

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

func TestCreateAndGet(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	q := Question{
		RepoID:       "my-repo",
		Question:     "What is the purpose of the payment-service?",
		Category:     CategoryBusiness,
		Priority:     80,
		Source:       "system",
		SourceDetail: "detected unknown service",
	}

	created, err := store.Create(ctx, q)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == "" {
		t.Error("expected non-empty ID")
	}
	if created.Status != StatusOpen {
		t.Errorf("expected status open, got %s", created.Status)
	}

	fetched, err := store.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if fetched.Question != q.Question {
		t.Errorf("expected %q, got %q", q.Question, fetched.Question)
	}
}

func TestAnswer(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	created, _ := store.Create(ctx, Question{
		Question: "What does service X do?",
		Priority: 50,
	})

	err := store.Answer(ctx, created.ID, "It handles authentication", "alice@example.com")
	if err != nil {
		t.Fatalf("Answer: %v", err)
	}

	fetched, _ := store.GetByID(ctx, created.ID)
	if fetched.Status != StatusAnswered {
		t.Errorf("expected status answered, got %s", fetched.Status)
	}
	if fetched.Answer != "It handles authentication" {
		t.Errorf("unexpected answer: %q", fetched.Answer)
	}
	if fetched.AnsweredBy != "alice@example.com" {
		t.Errorf("unexpected answered_by: %q", fetched.AnsweredBy)
	}
	if fetched.AnsweredAt == nil {
		t.Error("expected non-nil answered_at")
	}
}

func TestListWithFilters(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	store.Create(ctx, Question{Question: "Q1", Priority: 90, Category: CategoryBusiness})
	store.Create(ctx, Question{Question: "Q2", Priority: 30, Category: CategoryOwnership})
	store.Create(ctx, Question{Question: "Q3", Priority: 70, Category: CategoryBusiness})

	// List all.
	all, _ := store.List(ctx, ListFilter{})
	if len(all) != 3 {
		t.Fatalf("expected 3, got %d", len(all))
	}
	// Should be ordered by priority DESC.
	if all[0].Priority != 90 {
		t.Errorf("expected first priority=90, got %d", all[0].Priority)
	}

	// Filter by category.
	biz, _ := store.List(ctx, ListFilter{Category: CategoryBusiness})
	if len(biz) != 2 {
		t.Errorf("expected 2 business questions, got %d", len(biz))
	}

	// Filter by min priority.
	high, _ := store.List(ctx, ListFilter{MinPriority: 50})
	if len(high) != 2 {
		t.Errorf("expected 2 high-priority, got %d", len(high))
	}

	// Limit.
	limited, _ := store.List(ctx, ListFilter{Limit: 1})
	if len(limited) != 1 {
		t.Errorf("expected 1, got %d", len(limited))
	}
}

func TestUpdateStatus(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	created, _ := store.Create(ctx, Question{Question: "Q1", Priority: 50})
	err := store.UpdateStatus(ctx, created.ID, StatusRetired)
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	fetched, _ := store.GetByID(ctx, created.ID)
	if fetched.Status != StatusRetired {
		t.Errorf("expected retired, got %s", fetched.Status)
	}
}

func TestGetOpenCount(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	store.Create(ctx, Question{Question: "Q1", Priority: 50})
	store.Create(ctx, Question{Question: "Q2", Priority: 50})

	count, err := store.GetOpenCount(ctx)
	if err != nil {
		t.Fatalf("GetOpenCount: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}

	// Answer one.
	all, _ := store.List(ctx, ListFilter{})
	store.Answer(ctx, all[0].ID, "ans", "user")

	count, _ = store.GetOpenCount(ctx)
	if count != 1 {
		t.Errorf("expected 1 after answer, got %d", count)
	}
}

func TestGetTopPriority(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	store.Create(ctx, Question{Question: "Low", Priority: 10})
	store.Create(ctx, Question{Question: "High", Priority: 90})
	store.Create(ctx, Question{Question: "Med", Priority: 50})

	top, err := store.GetTopPriority(ctx, 2)
	if err != nil {
		t.Fatalf("GetTopPriority: %v", err)
	}
	if len(top) != 2 {
		t.Fatalf("expected 2, got %d", len(top))
	}
	if top[0].Priority != 90 {
		t.Errorf("expected highest first, got priority %d", top[0].Priority)
	}
}

// HTTP handler tests

func TestRoute_ListQuestions(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	store.Create(ctx, Question{Question: "Q1", Priority: 80})
	store.Create(ctx, Question{Question: "Q2", Priority: 40})

	r := chi.NewRouter()
	RegisterRoutes(r, store)

	req := httptest.NewRequest("GET", "/api/backlog/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var questions []Question
	json.Unmarshal(w.Body.Bytes(), &questions)
	if len(questions) != 2 {
		t.Errorf("expected 2, got %d", len(questions))
	}
}

func TestRoute_CreateQuestion(t *testing.T) {
	store := setupTestStore(t)

	r := chi.NewRouter()
	RegisterRoutes(r, store)

	body := `{"question":"What does service X do?","priority":70,"category":"business_context"}`
	req := httptest.NewRequest("POST", "/api/backlog/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var q Question
	json.Unmarshal(w.Body.Bytes(), &q)
	if q.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestRoute_AnswerQuestion(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	created, _ := store.Create(ctx, Question{Question: "Q?", Priority: 50})

	r := chi.NewRouter()
	RegisterRoutes(r, store)

	body := `{"answer":"The answer","answered_by":"alice"}`
	req := httptest.NewRequest("POST", "/api/backlog/"+created.ID+"/answer", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRoute_Stats(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	store.Create(ctx, Question{Question: "Q1", Priority: 50})

	r := chi.NewRouter()
	RegisterRoutes(r, store)

	req := httptest.NewRequest("GET", "/api/backlog/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var stats map[string]int
	json.Unmarshal(w.Body.Bytes(), &stats)
	if stats["open_count"] != 1 {
		t.Errorf("expected open_count=1, got %d", stats["open_count"])
	}
}

func TestRoute_GetNotFound(t *testing.T) {
	store := setupTestStore(t)

	r := chi.NewRouter()
	RegisterRoutes(r, store)

	req := httptest.NewRequest("GET", "/api/backlog/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
