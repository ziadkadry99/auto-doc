package importers

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

// --- README Parser Tests ---

func TestParseReadme(t *testing.T) {
	content := `# My Project

This is a cool project.

## Installation

Run npm install.

## Usage

Use it like this.

### Advanced Usage

More advanced stuff.
`
	sections := ParseReadme(content)
	if len(sections) != 4 {
		t.Fatalf("expected 4 sections, got %d", len(sections))
	}
	if sections[0].Heading != "My Project" {
		t.Errorf("expected 'My Project', got %q", sections[0].Heading)
	}
	if sections[0].Level != 1 {
		t.Errorf("expected level 1, got %d", sections[0].Level)
	}
	if !strings.Contains(sections[0].Content, "cool project") {
		t.Errorf("expected content to contain 'cool project', got %q", sections[0].Content)
	}
	if sections[3].Heading != "Advanced Usage" {
		t.Errorf("expected 'Advanced Usage', got %q", sections[3].Heading)
	}
	if sections[3].Level != 3 {
		t.Errorf("expected level 3, got %d", sections[3].Level)
	}
}

func TestExtractReadmeDescription(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "description section",
			content: "# Title\n\nSome intro.\n\n## Description\n\nThe real description.\n\n## Other\n\nStuff.",
			want:    "The real description.",
		},
		{
			name:    "first section content",
			content: "# My App\n\nThis is my app.\n\n## Install\n\nRun install.",
			want:    "This is my app.",
		},
		{
			name:    "no headings",
			content: "Just a paragraph of text.",
			want:    "Just a paragraph of text.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractReadmeDescription(tt.content)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// --- ADR Parser Tests ---

func TestParseADRFile(t *testing.T) {
	content := `# 1. Use PostgreSQL for Data Storage

## Status

Accepted

## Context

We need a reliable database for our application.

## Decision

We will use PostgreSQL as our primary database.

## Consequences

- Need to manage database migrations
- Team needs PostgreSQL knowledge
`
	adr := ParseADRFile(content, "docs/adr/0001-use-postgresql.md")

	if adr.Number != 1 {
		t.Errorf("expected number 1, got %d", adr.Number)
	}
	if adr.Status != "accepted" {
		t.Errorf("expected status 'accepted', got %q", adr.Status)
	}
	if !strings.Contains(adr.Context, "reliable database") {
		t.Errorf("expected context to contain 'reliable database', got %q", adr.Context)
	}
	if !strings.Contains(adr.Decision, "PostgreSQL") {
		t.Errorf("expected decision to contain 'PostgreSQL', got %q", adr.Decision)
	}
	if !strings.Contains(adr.Consequences, "migrations") {
		t.Errorf("expected consequences to contain 'migrations', got %q", adr.Consequences)
	}
}

func TestParseADRFile_TitleFromHeading(t *testing.T) {
	content := `# Use Redis for Caching

## Status

Proposed
`
	adr := ParseADRFile(content, "docs/adr/0005-use-redis.md")
	if adr.Title != "Use Redis for Caching" {
		t.Errorf("expected title from heading, got %q", adr.Title)
	}
}

// --- OpenAPI Parser Tests ---

func TestParseOpenAPI_JSON(t *testing.T) {
	spec := `{
  "openapi": "3.0.0",
  "info": {"title": "Test API", "version": "1.0"},
  "paths": {
    "/users": {
      "get": {
        "summary": "List users",
        "parameters": [{"name": "limit", "in": "query"}],
        "responses": {"200": {"description": "Success"}}
      },
      "post": {
        "summary": "Create user",
        "responses": {"201": {"description": "Created"}}
      }
    },
    "/users/{id}": {
      "get": {
        "summary": "Get user",
        "description": "Get a single user by ID"
      }
    }
  }
}`

	endpoints, err := ParseOpenAPI(spec)
	if err != nil {
		t.Fatalf("ParseOpenAPI: %v", err)
	}

	if len(endpoints) != 3 {
		t.Fatalf("expected 3 endpoints, got %d", len(endpoints))
	}

	// Check first endpoint.
	if endpoints[0].Path != "/users" || endpoints[0].Method != "GET" {
		t.Errorf("unexpected first endpoint: %s %s", endpoints[0].Method, endpoints[0].Path)
	}
	if endpoints[0].Summary != "List users" {
		t.Errorf("unexpected summary: %q", endpoints[0].Summary)
	}
	if len(endpoints[0].Parameters) != 1 {
		t.Errorf("expected 1 parameter, got %d", len(endpoints[0].Parameters))
	}
}

func TestParseOpenAPI_YAML(t *testing.T) {
	spec := `openapi: "3.0.0"
info:
  title: Test API
  version: "1.0"
paths:
  /health:
    get:
      summary: Health check
      responses:
        "200":
          description: OK
`
	endpoints, err := ParseOpenAPI(spec)
	if err != nil {
		t.Fatalf("ParseOpenAPI: %v", err)
	}
	if len(endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(endpoints))
	}
	if endpoints[0].Summary != "Health check" {
		t.Errorf("unexpected summary: %q", endpoints[0].Summary)
	}
}

func TestFormatEndpointsMarkdown(t *testing.T) {
	endpoints := []OpenAPIEndpoint{
		{Path: "/users", Method: "GET", Summary: "List users"},
		{Path: "/users", Method: "POST", Summary: "Create user"},
	}

	md := FormatEndpointsMarkdown(endpoints)
	if !strings.Contains(md, "| GET | `/users` | List users |") {
		t.Errorf("unexpected markdown output: %s", md)
	}
}

// --- HTML to Plain Text ---

func TestHtmlToPlainText(t *testing.T) {
	html := `<h1>Title</h1><p>First paragraph.</p><p>Second paragraph.</p><ul><li>Item 1</li><li>Item 2</li></ul>`
	text := htmlToPlainText(html)
	if !strings.Contains(text, "First paragraph.") {
		t.Errorf("expected 'First paragraph.' in output: %q", text)
	}
	if strings.Contains(text, "<") {
		t.Errorf("HTML tags should be stripped: %q", text)
	}
}

// --- Import Source Store Tests ---

func TestStoreCreateAndList(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	src := ImportSource{
		Type:   SourceConfluence,
		Name:   "Architecture Space",
		Config: `{"space_key":"ARCH"}`,
	}

	created, err := store.Create(ctx, src)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == "" {
		t.Error("expected non-empty ID")
	}

	sources, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}
	if sources[0].Name != "Architecture Space" {
		t.Errorf("unexpected name: %q", sources[0].Name)
	}
}

func TestStoreGetByID(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	created, _ := store.Create(ctx, ImportSource{Type: SourceReadme, Name: "Main README"})

	fetched, err := store.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if fetched.Name != "Main README" {
		t.Errorf("unexpected name: %q", fetched.Name)
	}
}

func TestStoreDelete(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	created, _ := store.Create(ctx, ImportSource{Type: SourceADR, Name: "ADRs"})
	store.Delete(ctx, created.ID)

	fetched, _ := store.GetByID(ctx, created.ID)
	if fetched != nil {
		t.Error("expected nil after delete")
	}
}

// --- HTTP Route Tests ---

func TestRoute_ListSources(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	store.Create(ctx, ImportSource{Type: SourceConfluence, Name: "Test"})

	r := chi.NewRouter()
	RegisterRoutes(r, store)

	req := httptest.NewRequest("GET", "/api/imports/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var sources []ImportSource
	json.Unmarshal(w.Body.Bytes(), &sources)
	if len(sources) != 1 {
		t.Errorf("expected 1 source, got %d", len(sources))
	}
}

func TestRoute_CreateSource(t *testing.T) {
	store := setupTestStore(t)

	r := chi.NewRouter()
	RegisterRoutes(r, store)

	body := `{"type":"confluence","name":"ARCH Space","config":"{\"space_key\":\"ARCH\"}"}`
	req := httptest.NewRequest("POST", "/api/imports/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRoute_GetNotFound(t *testing.T) {
	store := setupTestStore(t)

	r := chi.NewRouter()
	RegisterRoutes(r, store)

	req := httptest.NewRequest("GET", "/api/imports/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
