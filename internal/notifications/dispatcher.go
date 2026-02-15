package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Digest summarises notifications for a team over a time period.
type Digest struct {
	TeamID        string         `json:"team_id"`
	Period        string         `json:"period"`
	Notifications []Notification `json:"notifications"`
	Summary       string         `json:"summary"`
}

// Dispatcher creates notifications and delivers them to webhook subscribers.
type Dispatcher struct {
	store  *Store
	client *http.Client
}

// NewDispatcher creates a Dispatcher backed by the given store.
func NewDispatcher(store *Store) *Dispatcher {
	return &Dispatcher{
		store: store,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Dispatch persists a notification and sends it to matching webhook subscribers.
func (d *Dispatcher) Dispatch(ctx context.Context, n Notification) error {
	if err := d.store.Create(ctx, n); err != nil {
		return fmt.Errorf("creating notification: %w", err)
	}

	// Deliver to webhook subscribers for each affected team.
	for _, teamID := range n.AffectedTeams {
		prefs, err := d.store.GetPreferences(ctx, teamID)
		if err != nil {
			continue
		}
		for _, pref := range prefs {
			if pref.WebhookURL == "" {
				continue
			}
			if !severityMatches(n.Severity, pref.SeverityFilter) {
				continue
			}
			payload, err := json.Marshal(n)
			if err != nil {
				continue
			}
			_ = d.SendWebhook(ctx, pref.WebhookURL, payload)
		}
	}

	return nil
}

// GenerateDigest builds a summary of notifications for a team since the given time.
func (d *Dispatcher) GenerateDigest(ctx context.Context, teamID string, since time.Time) (*Digest, error) {
	all, err := d.store.List(ctx, ListFilter{Since: since})
	if err != nil {
		return nil, fmt.Errorf("listing notifications for digest: %w", err)
	}

	// Filter to notifications affecting this team.
	var matched []Notification
	for _, n := range all {
		for _, t := range n.AffectedTeams {
			if t == teamID {
				matched = append(matched, n)
				break
			}
		}
	}

	period := fmt.Sprintf("%s to %s",
		since.UTC().Format(time.RFC3339),
		time.Now().UTC().Format(time.RFC3339))

	summary := fmt.Sprintf("%d notification(s) for team %s", len(matched), teamID)

	return &Digest{
		TeamID:        teamID,
		Period:        period,
		Notifications: matched,
		Summary:       summary,
	}, nil
}

// SendWebhook POSTs payload to the given URL.
func (d *Dispatcher) SendWebhook(ctx context.Context, url string, payload []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("creating webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}

// severityMatches returns true if the notification severity meets or exceeds the filter threshold.
func severityMatches(actual, filter Severity) bool {
	levels := map[Severity]int{
		SeverityInfo:     0,
		SeverityWarning:  1,
		SeverityCritical: 2,
	}
	return levels[actual] >= levels[filter]
}
