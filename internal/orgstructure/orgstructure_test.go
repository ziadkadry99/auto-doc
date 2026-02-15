package orgstructure

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/ziadkadry99/auto-doc/internal/db"
)

func setupTestStore(t *testing.T) *Store {
	t.Helper()
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return NewStore(d)
}

// --- Store CRUD tests ---

func TestCreateAndGetTeam(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	team := &Team{Name: "backend", DisplayName: "Backend Team", Source: "manual"}
	if err := store.CreateTeam(ctx, team); err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}
	if team.ID == "" {
		t.Fatal("expected team ID to be set")
	}

	got, err := store.GetTeam(ctx, team.ID)
	if err != nil {
		t.Fatalf("GetTeam: %v", err)
	}
	if got.Name != "backend" {
		t.Errorf("got name %q, want %q", got.Name, "backend")
	}
	if got.DisplayName != "Backend Team" {
		t.Errorf("got display_name %q, want %q", got.DisplayName, "Backend Team")
	}
}

func TestListTeams(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	store.CreateTeam(ctx, &Team{Name: "alpha"})
	store.CreateTeam(ctx, &Team{Name: "beta"})

	teams, err := store.ListTeams(ctx)
	if err != nil {
		t.Fatalf("ListTeams: %v", err)
	}
	if len(teams) != 2 {
		t.Fatalf("got %d teams, want 2", len(teams))
	}
}

func TestUpdateTeam(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	team := &Team{Name: "frontend"}
	store.CreateTeam(ctx, team)

	team.DisplayName = "Frontend Engineers"
	if err := store.UpdateTeam(ctx, team); err != nil {
		t.Fatalf("UpdateTeam: %v", err)
	}

	got, _ := store.GetTeam(ctx, team.ID)
	if got.DisplayName != "Frontend Engineers" {
		t.Errorf("got display_name %q, want %q", got.DisplayName, "Frontend Engineers")
	}
}

func TestDeleteTeam(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	team := &Team{Name: "temp"}
	store.CreateTeam(ctx, team)

	if err := store.DeleteTeam(ctx, team.ID); err != nil {
		t.Fatalf("DeleteTeam: %v", err)
	}

	_, err := store.GetTeam(ctx, team.ID)
	if err == nil {
		t.Fatal("expected error after deleting team")
	}
}

func TestAddAndRemoveMember(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	team := &Team{Name: "devops"}
	store.CreateTeam(ctx, team)

	m := &TeamMember{TeamID: team.ID, UserID: "user-1", Role: "lead"}
	if err := store.AddMember(ctx, m); err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	members, err := store.ListMembers(ctx, team.ID)
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("got %d members, want 1", len(members))
	}
	if members[0].Role != "lead" {
		t.Errorf("got role %q, want %q", members[0].Role, "lead")
	}

	if err := store.RemoveMember(ctx, team.ID, "user-1"); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}
	members, _ = store.ListMembers(ctx, team.ID)
	if len(members) != 0 {
		t.Fatalf("got %d members after remove, want 0", len(members))
	}
}

func TestServiceOwnership(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	team := &Team{Name: "platform"}
	store.CreateTeam(ctx, team)

	o := &ServiceOwnership{TeamID: team.ID, RepoID: "my-service", Confidence: "confirmed", Source: "manual"}
	if err := store.SetOwnership(ctx, o); err != nil {
		t.Fatalf("SetOwnership: %v", err)
	}

	ownerships, err := store.GetOwnership(ctx, "my-service")
	if err != nil {
		t.Fatalf("GetOwnership: %v", err)
	}
	if len(ownerships) != 1 {
		t.Fatalf("got %d ownerships, want 1", len(ownerships))
	}
	if ownerships[0].Confidence != "confirmed" {
		t.Errorf("got confidence %q, want %q", ownerships[0].Confidence, "confirmed")
	}

	byTeam, err := store.ListOwnerships(ctx, team.ID)
	if err != nil {
		t.Fatalf("ListOwnerships: %v", err)
	}
	if len(byTeam) != 1 {
		t.Fatalf("got %d ownerships by team, want 1", len(byTeam))
	}
}

// --- CODEOWNERS parsing tests ---

func TestParseCodeowners(t *testing.T) {
	content := `# Global owners
* @global-team

# Backend
/src/backend/ @backend-team
/src/api/ @api-team @backend-team

# Frontend
*.js @frontend-team
*.css @frontend-team

# DevOps
/infra/ @devops
Dockerfile @devops
`
	rules, err := ParseCodeowners(content)
	if err != nil {
		t.Fatalf("ParseCodeowners: %v", err)
	}
	if len(rules) != 7 {
		t.Fatalf("got %d rules, want 7", len(rules))
	}

	// Check that only the first owner is used per rule (index 2 = /src/api/ line).
	if rules[2].Owner != "@api-team" {
		t.Errorf("rule 2 owner = %q, want %q", rules[2].Owner, "@api-team")
	}
}

func TestParseCodeownersEmpty(t *testing.T) {
	rules, err := ParseCodeowners("")
	if err != nil {
		t.Fatalf("ParseCodeowners: %v", err)
	}
	if len(rules) != 0 {
		t.Fatalf("got %d rules, want 0", len(rules))
	}
}

func TestMatchFileLastRuleWins(t *testing.T) {
	rules := []CodeownersRule{
		{Pattern: "*", Owner: "@global"},
		{Pattern: "*.go", Owner: "@go-team"},
		{Pattern: "/src/backend/", Owner: "@backend"},
	}

	tests := []struct {
		path string
		want string
	}{
		{"README.md", "@global"},
		{"main.go", "@go-team"},
		{"src/backend/server.go", "@backend"},
		{"src/frontend/app.js", "@global"},
	}

	for _, tt := range tests {
		got := MatchFile(rules, tt.path)
		if got != tt.want {
			t.Errorf("MatchFile(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestMatchFileDirectoryPattern(t *testing.T) {
	rules := []CodeownersRule{
		{Pattern: "docs/", Owner: "@docs-team"},
	}

	if got := MatchFile(rules, "docs/guide.md"); got != "@docs-team" {
		t.Errorf("got %q, want @docs-team", got)
	}
	if got := MatchFile(rules, "src/main.go"); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestMatchFileWildcardExtension(t *testing.T) {
	rules := []CodeownersRule{
		{Pattern: "*.proto", Owner: "@proto-team"},
	}
	if got := MatchFile(rules, "api/service.proto"); got != "@proto-team" {
		t.Errorf("got %q, want @proto-team", got)
	}
	if got := MatchFile(rules, "api/service.go"); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// --- HTTP handler tests ---

func setupTestRouter(t *testing.T) (chi.Router, *Store) {
	t.Helper()
	store := setupTestStore(t)
	r := chi.NewRouter()
	RegisterRoutes(r, store)
	return r, store
}

func TestHTTPListTeamsEmpty(t *testing.T) {
	r, _ := setupTestRouter(t)

	req := httptest.NewRequest("GET", "/api/teams", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var teams []Team
	json.NewDecoder(w.Body).Decode(&teams)
	if len(teams) != 0 {
		t.Fatalf("got %d teams, want 0", len(teams))
	}
}

func TestHTTPCreateAndGetTeam(t *testing.T) {
	r, _ := setupTestRouter(t)

	// Create
	body, _ := json.Marshal(Team{Name: "test-team", DisplayName: "Test Team"})
	req := httptest.NewRequest("POST", "/api/teams", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d, body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
	var created Team
	json.NewDecoder(w.Body).Decode(&created)
	if created.ID == "" {
		t.Fatal("expected ID in response")
	}

	// Get
	req = httptest.NewRequest("GET", "/api/teams/"+created.ID, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d", w.Code, http.StatusOK)
	}
	var got Team
	json.NewDecoder(w.Body).Decode(&got)
	if got.Name != "test-team" {
		t.Errorf("got name %q, want %q", got.Name, "test-team")
	}
}

func TestHTTPUpdateTeam(t *testing.T) {
	r, store := setupTestRouter(t)
	ctx := context.Background()

	team := &Team{Name: "old-name"}
	store.CreateTeam(ctx, team)

	body, _ := json.Marshal(Team{Name: "new-name", DisplayName: "New Name"})
	req := httptest.NewRequest("PUT", "/api/teams/"+team.ID, bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("update status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestHTTPListTeamServices(t *testing.T) {
	r, store := setupTestRouter(t)
	ctx := context.Background()

	team := &Team{Name: "svc-team"}
	store.CreateTeam(ctx, team)
	store.SetOwnership(ctx, &ServiceOwnership{TeamID: team.ID, RepoID: "svc-a"})

	req := httptest.NewRequest("GET", "/api/teams/"+team.ID+"/services", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var ownerships []ServiceOwnership
	json.NewDecoder(w.Body).Decode(&ownerships)
	if len(ownerships) != 1 {
		t.Fatalf("got %d ownerships, want 1", len(ownerships))
	}
}

func TestHTTPGetOwnership(t *testing.T) {
	r, store := setupTestRouter(t)
	ctx := context.Background()

	team := &Team{Name: "own-team"}
	store.CreateTeam(ctx, team)
	store.SetOwnership(ctx, &ServiceOwnership{TeamID: team.ID, RepoID: "repo-x"})

	req := httptest.NewRequest("GET", "/api/ownership/repo-x", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var ownerships []ServiceOwnership
	json.NewDecoder(w.Body).Decode(&ownerships)
	if len(ownerships) != 1 {
		t.Fatalf("got %d ownerships, want 1", len(ownerships))
	}
}
