package notifications

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

func testNotification(id string) Notification {
	return Notification{
		ID:               id,
		Type:             TypeServiceAdded,
		Severity:         SeverityInfo,
		Title:            "New service detected",
		Message:          "Service payments-api was detected in the codebase",
		AffectedServices: []string{"payments-api"},
		AffectedTeams:    []string{"platform"},
	}
}

func TestStoreCreate(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	n := testNotification("n-1")
	if err := store.Create(ctx, n); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := store.GetByID(ctx, "n-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Title != n.Title {
		t.Errorf("Title = %q, want %q", got.Title, n.Title)
	}
	if got.Delivered {
		t.Error("expected Delivered = false")
	}
	if len(got.AffectedServices) != 1 || got.AffectedServices[0] != "payments-api" {
		t.Errorf("AffectedServices = %v, want [payments-api]", got.AffectedServices)
	}
}

func TestStoreCreateAutoID(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	n := testNotification("")
	if err := store.Create(ctx, n); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Should be at least one notification.
	all, err := store.List(ctx, ListFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(all))
	}
	if all[0].ID == "" {
		t.Error("expected generated ID")
	}
}

func TestStoreGetByIDNotFound(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	_, err := store.GetByID(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent ID")
	}
}

func TestStoreListFilters(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	// Create notifications of different types and severities.
	n1 := Notification{
		ID: "f-1", Type: TypeServiceAdded, Severity: SeverityInfo,
		Title: "Added", AffectedServices: []string{"svc-a"}, AffectedTeams: []string{"team-a"},
	}
	n2 := Notification{
		ID: "f-2", Type: TypeDocUpdated, Severity: SeverityWarning,
		Title: "Updated", AffectedServices: []string{"svc-b"}, AffectedTeams: []string{"team-b"},
	}
	n3 := Notification{
		ID: "f-3", Type: TypeStalenessDetected, Severity: SeverityCritical,
		Title: "Stale", AffectedServices: []string{"svc-c"}, AffectedTeams: []string{"team-c"},
	}

	for _, n := range []Notification{n1, n2, n3} {
		if err := store.Create(ctx, n); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	// Filter by type.
	got, err := store.List(ctx, ListFilter{Type: TypeDocUpdated})
	if err != nil {
		t.Fatalf("List by type: %v", err)
	}
	if len(got) != 1 || got[0].ID != "f-2" {
		t.Errorf("filter by type: got %d results", len(got))
	}

	// Filter by severity.
	got, err = store.List(ctx, ListFilter{Severity: SeverityCritical})
	if err != nil {
		t.Fatalf("List by severity: %v", err)
	}
	if len(got) != 1 || got[0].ID != "f-3" {
		t.Errorf("filter by severity: got %d results", len(got))
	}

	// Filter with limit.
	got, err = store.List(ctx, ListFilter{Limit: 2})
	if err != nil {
		t.Fatalf("List with limit: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("filter with limit: got %d results, want 2", len(got))
	}

	// Filter by delivered status.
	delivered := false
	got, err = store.List(ctx, ListFilter{Delivered: &delivered})
	if err != nil {
		t.Fatalf("List by delivered: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("filter by delivered=false: got %d results, want 3", len(got))
	}
}

func TestStoreMarkDelivered(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	n := testNotification("d-1")
	if err := store.Create(ctx, n); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := store.MarkDelivered(ctx, "d-1"); err != nil {
		t.Fatalf("MarkDelivered: %v", err)
	}

	got, err := store.GetByID(ctx, "d-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if !got.Delivered {
		t.Error("expected Delivered = true after MarkDelivered")
	}
}

func TestStoreMarkDeliveredNotFound(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	err := store.MarkDelivered(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent ID")
	}
}

func TestStoreGetPending(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	n1 := testNotification("p-1")
	n2 := testNotification("p-2")
	if err := store.Create(ctx, n1); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := store.Create(ctx, n2); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Mark one delivered.
	if err := store.MarkDelivered(ctx, "p-1"); err != nil {
		t.Fatalf("MarkDelivered: %v", err)
	}

	pending, err := store.GetPending(ctx)
	if err != nil {
		t.Fatalf("GetPending: %v", err)
	}
	if len(pending) != 1 || pending[0].ID != "p-2" {
		t.Errorf("GetPending: got %d results, want 1 (p-2)", len(pending))
	}
}

func TestPreferenceCRUD(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	pref := Preference{
		TeamID:          "team-alpha",
		Channel:         "webhook",
		SeverityFilter:  SeverityWarning,
		DigestFrequency: FreqDaily,
		WebhookURL:      "https://example.com/hook",
	}

	if err := store.SetPreference(ctx, pref); err != nil {
		t.Fatalf("SetPreference: %v", err)
	}

	prefs, err := store.GetPreferences(ctx, "team-alpha")
	if err != nil {
		t.Fatalf("GetPreferences: %v", err)
	}
	if len(prefs) != 1 {
		t.Fatalf("expected 1 preference, got %d", len(prefs))
	}
	if prefs[0].WebhookURL != "https://example.com/hook" {
		t.Errorf("WebhookURL = %q, want %q", prefs[0].WebhookURL, "https://example.com/hook")
	}
	if prefs[0].DigestFrequency != FreqDaily {
		t.Errorf("DigestFrequency = %q, want %q", prefs[0].DigestFrequency, FreqDaily)
	}

	// Update the preference.
	pref.SeverityFilter = SeverityCritical
	if err := store.SetPreference(ctx, pref); err != nil {
		t.Fatalf("SetPreference (update): %v", err)
	}

	prefs, err = store.GetPreferences(ctx, "team-alpha")
	if err != nil {
		t.Fatalf("GetPreferences: %v", err)
	}
	if len(prefs) != 1 {
		t.Fatalf("expected 1 preference after upsert, got %d", len(prefs))
	}
	if prefs[0].SeverityFilter != SeverityCritical {
		t.Errorf("SeverityFilter = %q, want %q", prefs[0].SeverityFilter, SeverityCritical)
	}
}

func TestPreferencesEmpty(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	prefs, err := store.GetPreferences(ctx, "no-such-team")
	if err != nil {
		t.Fatalf("GetPreferences: %v", err)
	}
	if prefs != nil {
		t.Errorf("expected nil for nonexistent team, got %v", prefs)
	}
}

func TestDispatcherWebhook(t *testing.T) {
	store := setupTestStore(t)
	dispatcher := NewDispatcher(store)
	ctx := context.Background()

	// Set up a mock webhook server.
	var received []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		buf.ReadFrom(r.Body)
		received = buf.Bytes()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Set up webhook preference for the team.
	pref := Preference{
		TeamID:          "platform",
		Channel:         "webhook",
		SeverityFilter:  SeverityInfo,
		DigestFrequency: FreqRealtime,
		WebhookURL:      server.URL,
	}
	if err := store.SetPreference(ctx, pref); err != nil {
		t.Fatalf("SetPreference: %v", err)
	}

	// Dispatch a notification.
	n := testNotification("wh-1")
	if err := dispatcher.Dispatch(ctx, n); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	// Verify webhook was called.
	if received == nil {
		t.Fatal("webhook was not called")
	}
	var got Notification
	if err := json.Unmarshal(received, &got); err != nil {
		t.Fatalf("unmarshalling webhook payload: %v", err)
	}
	if got.Title != n.Title {
		t.Errorf("webhook payload Title = %q, want %q", got.Title, n.Title)
	}

	// Verify notification was persisted.
	stored, err := store.GetByID(ctx, "wh-1")
	if err != nil {
		t.Fatalf("GetByID after dispatch: %v", err)
	}
	if stored.Title != n.Title {
		t.Errorf("stored Title = %q, want %q", stored.Title, n.Title)
	}
}

func TestDispatcherSeverityFiltering(t *testing.T) {
	store := setupTestStore(t)
	dispatcher := NewDispatcher(store)
	ctx := context.Background()

	webhookCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Set preference to only receive critical notifications.
	pref := Preference{
		TeamID:          "platform",
		Channel:         "webhook",
		SeverityFilter:  SeverityCritical,
		DigestFrequency: FreqRealtime,
		WebhookURL:      server.URL,
	}
	if err := store.SetPreference(ctx, pref); err != nil {
		t.Fatalf("SetPreference: %v", err)
	}

	// Dispatch an info-level notification — should not trigger webhook.
	n := testNotification("sf-1")
	n.Severity = SeverityInfo
	if err := dispatcher.Dispatch(ctx, n); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if webhookCalled {
		t.Error("webhook should not be called for info severity when filter is critical")
	}

	// Dispatch a critical notification — should trigger webhook.
	n2 := testNotification("sf-2")
	n2.Severity = SeverityCritical
	if err := dispatcher.Dispatch(ctx, n2); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if !webhookCalled {
		t.Error("webhook should be called for critical severity")
	}
}

func TestDigestGeneration(t *testing.T) {
	store := setupTestStore(t)
	dispatcher := NewDispatcher(store)
	ctx := context.Background()

	// Create notifications for different teams.
	n1 := Notification{
		ID: "dg-1", Type: TypeDocUpdated, Severity: SeverityInfo,
		Title: "Doc A updated", AffectedServices: []string{"svc-a"}, AffectedTeams: []string{"team-a"},
	}
	n2 := Notification{
		ID: "dg-2", Type: TypeServiceAdded, Severity: SeverityWarning,
		Title: "New service", AffectedServices: []string{"svc-b"}, AffectedTeams: []string{"team-a", "team-b"},
	}
	n3 := Notification{
		ID: "dg-3", Type: TypeContextChanged, Severity: SeverityInfo,
		Title: "Context changed", AffectedServices: []string{"svc-c"}, AffectedTeams: []string{"team-b"},
	}

	for _, n := range []Notification{n1, n2, n3} {
		if err := store.Create(ctx, n); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	since := time.Now().UTC().Add(-1 * time.Hour)
	digest, err := dispatcher.GenerateDigest(ctx, "team-a", since)
	if err != nil {
		t.Fatalf("GenerateDigest: %v", err)
	}
	if digest.TeamID != "team-a" {
		t.Errorf("TeamID = %q, want team-a", digest.TeamID)
	}
	if len(digest.Notifications) != 2 {
		t.Errorf("expected 2 notifications in digest, got %d", len(digest.Notifications))
	}

	// team-b should have 2 notifications.
	digest2, err := dispatcher.GenerateDigest(ctx, "team-b", since)
	if err != nil {
		t.Fatalf("GenerateDigest: %v", err)
	}
	if len(digest2.Notifications) != 2 {
		t.Errorf("expected 2 notifications for team-b, got %d", len(digest2.Notifications))
	}
}

func TestHTTPHandlers(t *testing.T) {
	store := setupTestStore(t)
	dispatcher := NewDispatcher(store)
	ctx := context.Background()

	r := chi.NewRouter()
	RegisterRoutes(r, store, dispatcher)

	// Create test data.
	n := testNotification("api-1")
	if err := store.Create(ctx, n); err != nil {
		t.Fatalf("Create: %v", err)
	}

	t.Run("GET /api/notifications", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/notifications", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}

		var got []Notification
		if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if len(got) != 1 {
			t.Errorf("expected 1 notification, got %d", len(got))
		}
	})

	t.Run("GET /api/notifications/{id}", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/notifications/api-1", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}

		var got Notification
		if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if got.ID != "api-1" {
			t.Errorf("ID = %q, want api-1", got.ID)
		}
	})

	t.Run("GET /api/notifications/{id} not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/notifications/nonexistent", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
	})

	t.Run("POST /api/notifications/{id}/deliver", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/notifications/api-1/deliver", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}

		// Verify it's marked delivered.
		got, err := store.GetByID(ctx, "api-1")
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if !got.Delivered {
			t.Error("expected Delivered = true")
		}
	})

	t.Run("GET /api/notifications/pending", func(t *testing.T) {
		// api-1 is now delivered; create another undelivered one.
		n2 := testNotification("api-2")
		if err := store.Create(ctx, n2); err != nil {
			t.Fatalf("Create: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/notifications/pending", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}

		var got []Notification
		if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if len(got) != 1 || got[0].ID != "api-2" {
			t.Errorf("expected 1 pending notification (api-2), got %v", got)
		}
	})

	t.Run("PUT /api/notifications/preferences", func(t *testing.T) {
		pref := Preference{
			TeamID:          "team-x",
			Channel:         "dashboard",
			SeverityFilter:  SeverityWarning,
			DigestFrequency: FreqWeekly,
		}
		body, _ := json.Marshal(pref)

		req := httptest.NewRequest(http.MethodPut, "/api/notifications/preferences", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
		}
	})

	t.Run("GET /api/notifications/preferences/{teamID}", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/notifications/preferences/team-x", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}

		var got []Preference
		if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if len(got) != 1 || got[0].TeamID != "team-x" {
			t.Errorf("expected 1 preference for team-x, got %v", got)
		}
	})

	t.Run("GET /api/notifications/digest/{teamID}", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/notifications/digest/platform", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}

		var got Digest
		if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if got.TeamID != "platform" {
			t.Errorf("TeamID = %q, want platform", got.TeamID)
		}
	})

	t.Run("PUT /api/notifications/preferences missing fields", func(t *testing.T) {
		body, _ := json.Marshal(Preference{})
		req := httptest.NewRequest(http.MethodPut, "/api/notifications/preferences", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})
}

func TestSeverityMatches(t *testing.T) {
	tests := []struct {
		actual Severity
		filter Severity
		want   bool
	}{
		{SeverityInfo, SeverityInfo, true},
		{SeverityWarning, SeverityInfo, true},
		{SeverityCritical, SeverityInfo, true},
		{SeverityInfo, SeverityWarning, false},
		{SeverityWarning, SeverityWarning, true},
		{SeverityCritical, SeverityWarning, true},
		{SeverityInfo, SeverityCritical, false},
		{SeverityWarning, SeverityCritical, false},
		{SeverityCritical, SeverityCritical, true},
	}

	for _, tt := range tests {
		got := severityMatches(tt.actual, tt.filter)
		if got != tt.want {
			t.Errorf("severityMatches(%q, %q) = %v, want %v", tt.actual, tt.filter, got, tt.want)
		}
	}
}
