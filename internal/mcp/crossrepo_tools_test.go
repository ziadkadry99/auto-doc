package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/ziadkadry99/auto-doc/internal/contextengine"
	"github.com/ziadkadry99/auto-doc/internal/db"
	"github.com/ziadkadry99/auto-doc/internal/flows"
	"github.com/ziadkadry99/auto-doc/internal/orgstructure"
	"github.com/ziadkadry99/auto-doc/internal/vectordb"
)

// newTestServerWithPhase4 creates a Server with Phase 4 deps backed by an in-memory DB.
func newTestServerWithPhase4(t *testing.T, docs []vectordb.Document) (*Server, *db.DB) {
	t.Helper()

	database, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("opening in-memory db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	store := &mockStore{docs: docs}
	srv := NewServer(store, &mockEmbedder{}, t.TempDir())

	ctxStore := contextengine.NewStore(database)
	flowStore := flows.NewStore(database)
	orgStore := orgstructure.NewStore(database)

	srv.SetPhase4Deps(Phase4Deps{
		CtxStore:  ctxStore,
		FlowStore: flowStore,
		OrgStore:  orgStore,
		// CtxEngine is nil — requires an LLM provider; tested separately.
	})

	return srv, database
}

func TestCrossRepoToolDefinitions(t *testing.T) {
	tests := []struct {
		name     string
		tool     mcp.Tool
		wantName string
	}{
		{"search_across_repos", searchAcrossReposTool, "search_across_repos"},
		{"get_service_context", getServiceContextTool, "get_service_context"},
		{"get_blast_radius", getBlastRadiusTool, "get_blast_radius"},
		{"get_flow", getFlowTool, "get_flow"},
		{"ask_architecture", askArchitectureTool, "ask_architecture"},
		{"get_team_services", getTeamServicesTool, "get_team_services"},
		{"provide_context", provideContextTool, "provide_context"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.tool.Name != tt.wantName {
				t.Errorf("tool name = %q, want %q", tt.tool.Name, tt.wantName)
			}
			if tt.tool.Description == "" {
				t.Error("tool description should not be empty")
			}
		})
	}
}

func TestHandleSearchAcrossRepos(t *testing.T) {
	docs := []vectordb.Document{
		{
			ID:      "1",
			Content: "User service handles authentication.",
			Metadata: vectordb.DocumentMetadata{
				FilePath: "user-service/main.go",
				Type:     vectordb.DocTypeFile,
				RepoID:   "user-service",
			},
		},
		{
			ID:      "2",
			Content: "Order service processes purchases.",
			Metadata: vectordb.DocumentMetadata{
				FilePath: "order-service/main.go",
				Type:     vectordb.DocTypeFile,
				RepoID:   "order-service",
			},
		},
	}
	srv, _ := newTestServerWithPhase4(t, docs)
	ctx := context.Background()

	t.Run("basic search", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{"query": "authentication"}

		result, err := srv.handleSearchAcrossRepos(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %v", result.Content)
		}
	})

	t.Run("missing query", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{}

		result, err := srv.handleSearchAcrossRepos(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected error for missing query")
		}
	})

	t.Run("empty results", func(t *testing.T) {
		emptySrv := NewServer(&mockStore{}, &mockEmbedder{}, t.TempDir())
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{"query": "nonexistent"}

		result, err := emptySrv.handleSearchAcrossRepos(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Error("empty results should not be an error")
		}
	})
}

func TestHandleGetServiceContext(t *testing.T) {
	docs := []vectordb.Document{
		{
			ID:      "1",
			Content: "User service documentation.",
			Metadata: vectordb.DocumentMetadata{
				FilePath: "user-service/README.md",
				Type:     vectordb.DocTypeFile,
			},
		},
	}
	srv, database := newTestServerWithPhase4(t, docs)
	ctx := context.Background()

	// Save a fact for the service.
	ctxStore := contextengine.NewStore(database)
	_, err := ctxStore.SaveFact(ctx, contextengine.Fact{
		Scope:   "service",
		ScopeID: "user-service",
		Key:     "description",
		Value:   "Handles user authentication and profiles",
		Source:  "user",
	})
	if err != nil {
		t.Fatalf("saving fact: %v", err)
	}

	t.Run("with facts", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{"service": "user-service"}

		result, err := srv.handleGetServiceContext(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %v", result.Content)
		}
		text := extractText(result)
		if !strings.Contains(text, "Handles user authentication") {
			t.Errorf("expected fact in output, got: %s", text)
		}
	})

	t.Run("missing service param", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{}

		result, err := srv.handleGetServiceContext(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected error for missing service")
		}
	})
}

func TestHandleGetBlastRadius(t *testing.T) {
	docs := []vectordb.Document{
		{
			ID:      "1",
			Content: "Order service calls user-service for authentication.",
			Metadata: vectordb.DocumentMetadata{
				FilePath: "order-service/auth.go",
				Type:     vectordb.DocTypeFile,
			},
		},
	}
	srv, database := newTestServerWithPhase4(t, docs)
	ctx := context.Background()

	// Create a flow that includes the service.
	flowStore := flows.NewStore(database)
	err := flowStore.CreateFlow(ctx, &flows.Flow{
		Name:        "checkout",
		Description: "User checkout flow",
		Services:    []string{"user-service", "order-service", "payment-service"},
	})
	if err != nil {
		t.Fatalf("creating flow: %v", err)
	}

	t.Run("basic blast radius", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{"service": "user-service"}

		result, err := srv.handleGetBlastRadius(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %v", result.Content)
		}
		text := extractText(result)
		if !strings.Contains(text, "checkout") {
			t.Errorf("expected affected flow 'checkout' in output, got: %s", text)
		}
	})

	t.Run("with endpoint", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{
			"service":  "user-service",
			"endpoint": "/api/auth",
		}

		result, err := srv.handleGetBlastRadius(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %v", result.Content)
		}
	})

	t.Run("missing service", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{}

		result, err := srv.handleGetBlastRadius(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected error for missing service")
		}
	})
}

func TestHandleGetFlow(t *testing.T) {
	srv, database := newTestServerWithPhase4(t, nil)
	ctx := context.Background()

	// Create a test flow.
	flowStore := flows.NewStore(database)
	err := flowStore.CreateFlow(ctx, &flows.Flow{
		Name:           "user-registration",
		Description:    "New user registration flow",
		Narrative:      "A user signs up, the system verifies email, then creates a profile.",
		MermaidDiagram: "sequenceDiagram\n  User->>API: POST /register",
		Services:       []string{"api-gateway", "user-service", "email-service"},
		EntryPoint:     "api-gateway",
		ExitPoint:      "email-service",
	})
	if err != nil {
		t.Fatalf("creating flow: %v", err)
	}

	t.Run("existing flow", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{"flow_name": "user-registration"}

		result, err := srv.handleGetFlow(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %v", result.Content)
		}
		text := extractText(result)
		if !strings.Contains(text, "user-registration") {
			t.Errorf("expected flow name in output, got: %s", text)
		}
		if !strings.Contains(text, "user-service") {
			t.Errorf("expected service in output, got: %s", text)
		}
	})

	t.Run("nonexistent flow", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{"flow_name": "does-not-exist"}

		result, err := srv.handleGetFlow(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected error for nonexistent flow")
		}
	})

	t.Run("missing flow_name", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{}

		result, err := srv.handleGetFlow(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected error for missing flow_name")
		}
	})
}

func TestHandleGetFlow_NoDeps(t *testing.T) {
	srv := NewServer(&mockStore{}, &mockEmbedder{}, t.TempDir())
	ctx := context.Background()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"flow_name": "test"}

	result, err := srv.handleGetFlow(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error when phase4 deps are missing")
	}
	text := extractText(result)
	if !strings.Contains(text, "Phase 4") {
		t.Errorf("error should mention Phase 4, got: %s", text)
	}
}

func TestHandleAskArchitecture_NoDeps(t *testing.T) {
	srv := NewServer(&mockStore{}, &mockEmbedder{}, t.TempDir())
	ctx := context.Background()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"question": "How does auth work?"}

	result, err := srv.handleAskArchitecture(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error when context engine is missing")
	}
	text := extractText(result)
	if !strings.Contains(text, "Phase 4") {
		t.Errorf("error should mention Phase 4, got: %s", text)
	}
}

func TestHandleAskArchitecture_MissingQuestion(t *testing.T) {
	srv := NewServer(&mockStore{}, &mockEmbedder{}, t.TempDir())
	ctx := context.Background()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{}

	result, err := srv.handleAskArchitecture(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for missing question")
	}
}

func TestHandleGetTeamServices(t *testing.T) {
	srv, database := newTestServerWithPhase4(t, nil)
	ctx := context.Background()

	orgStore := orgstructure.NewStore(database)

	// Create a team.
	team := &orgstructure.Team{
		Name:        "platform",
		DisplayName: "Platform Team",
	}
	if err := orgStore.CreateTeam(ctx, team); err != nil {
		t.Fatalf("creating team: %v", err)
	}

	// Set ownership.
	if err := orgStore.SetOwnership(ctx, &orgstructure.ServiceOwnership{
		TeamID:     team.ID,
		RepoID:     "api-gateway",
		Confidence: "confirmed",
		Source:     "manual",
	}); err != nil {
		t.Fatalf("setting ownership: %v", err)
	}
	if err := orgStore.SetOwnership(ctx, &orgstructure.ServiceOwnership{
		TeamID:     team.ID,
		RepoID:     "auth-service",
		Confidence: "auto_detected",
		Source:     "codeowners",
	}); err != nil {
		t.Fatalf("setting ownership: %v", err)
	}

	t.Run("existing team by name", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{"team": "platform"}

		result, err := srv.handleGetTeamServices(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %v", result.Content)
		}
		text := extractText(result)
		if !strings.Contains(text, "api-gateway") {
			t.Errorf("expected api-gateway in output, got: %s", text)
		}
		if !strings.Contains(text, "auth-service") {
			t.Errorf("expected auth-service in output, got: %s", text)
		}
	})

	t.Run("nonexistent team", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{"team": "nonexistent"}

		result, err := srv.handleGetTeamServices(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected error for nonexistent team")
		}
	})

	t.Run("missing team param", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{}

		result, err := srv.handleGetTeamServices(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected error for missing team")
		}
	})
}

func TestHandleGetTeamServices_NoDeps(t *testing.T) {
	srv := NewServer(&mockStore{}, &mockEmbedder{}, t.TempDir())
	ctx := context.Background()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"team": "platform"}

	result, err := srv.handleGetTeamServices(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error when org store is missing")
	}
}

func TestHandleProvideContext(t *testing.T) {
	srv, database := newTestServerWithPhase4(t, nil)
	ctx := context.Background()

	t.Run("save context", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{
			"service": "payment-service",
			"context": "Uses Stripe API for payment processing. Handles webhooks for payment status updates.",
		}

		result, err := srv.handleProvideContext(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %v", result.Content)
		}
		text := extractText(result)
		if !strings.Contains(text, "payment-service") {
			t.Errorf("expected service name in confirmation, got: %s", text)
		}

		// Verify the fact was saved.
		ctxStore := contextengine.NewStore(database)
		facts, err := ctxStore.GetCurrentFacts(ctx, "", "service", "payment-service")
		if err != nil {
			t.Fatalf("getting facts: %v", err)
		}
		if len(facts) == 0 {
			t.Fatal("expected at least one fact to be saved")
		}
		found := false
		for _, f := range facts {
			if strings.Contains(f.Value, "Stripe API") {
				found = true
				break
			}
		}
		if !found {
			t.Error("saved fact should contain 'Stripe API'")
		}
	})

	t.Run("missing service", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{
			"context": "some context",
		}

		result, err := srv.handleProvideContext(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected error for missing service")
		}
	})

	t.Run("missing context", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{
			"service": "test",
		}

		result, err := srv.handleProvideContext(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected error for missing context")
		}
	})
}

func TestHandleProvideContext_NoDeps(t *testing.T) {
	srv := NewServer(&mockStore{}, &mockEmbedder{}, t.TempDir())
	ctx := context.Background()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"service": "test",
		"context": "some context",
	}

	result, err := srv.handleProvideContext(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error when context store is missing")
	}
}

// extractText gets the text content from a CallToolResult.
func extractText(result *mcp.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}
