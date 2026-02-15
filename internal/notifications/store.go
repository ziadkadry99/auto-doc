package notifications

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ziadkadry99/auto-doc/internal/db"
)

// ListFilter controls which notifications are returned by List.
type ListFilter struct {
	Type      NotificationType
	Severity  Severity
	Delivered *bool
	Since     time.Time
	Until     time.Time
	Limit     int
	Offset    int
}

// Store provides CRUD operations for notifications and preferences.
type Store struct {
	db *db.DB
}

// NewStore creates a Store backed by the given database.
func NewStore(database *db.DB) *Store {
	return &Store{db: database}
}

// Create inserts a new notification. If n.ID is empty a UUID is generated.
func (s *Store) Create(ctx context.Context, n Notification) error {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}

	services, err := json.Marshal(n.AffectedServices)
	if err != nil {
		return fmt.Errorf("marshalling affected services: %w", err)
	}
	teams, err := json.Marshal(n.AffectedTeams)
	if err != nil {
		return fmt.Errorf("marshalling affected teams: %w", err)
	}

	delivered := 0
	if n.Delivered {
		delivered = 1
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO notifications (id, type, severity, title, message, affected_services, affected_teams, delivered)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID, string(n.Type), string(n.Severity), n.Title, n.Message,
		string(services), string(teams), delivered,
	)
	if err != nil {
		return fmt.Errorf("inserting notification: %w", err)
	}
	return nil
}

// GetByID retrieves a single notification.
func (s *Store) GetByID(ctx context.Context, id string) (*Notification, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, type, severity, title, message, affected_services, affected_teams, delivered, created_at
		FROM notifications WHERE id = ?`, id)

	return scanNotification(row)
}

// List returns notifications matching the filter.
func (s *Store) List(ctx context.Context, filter ListFilter) ([]Notification, error) {
	var (
		clauses []string
		args    []any
	)

	if filter.Type != "" {
		clauses = append(clauses, "type = ?")
		args = append(args, string(filter.Type))
	}
	if filter.Severity != "" {
		clauses = append(clauses, "severity = ?")
		args = append(args, string(filter.Severity))
	}
	if filter.Delivered != nil {
		v := 0
		if *filter.Delivered {
			v = 1
		}
		clauses = append(clauses, "delivered = ?")
		args = append(args, v)
	}
	if !filter.Since.IsZero() {
		clauses = append(clauses, "created_at >= ?")
		args = append(args, filter.Since.UTC().Format(time.DateTime))
	}
	if !filter.Until.IsZero() {
		clauses = append(clauses, "created_at <= ?")
		args = append(args, filter.Until.UTC().Format(time.DateTime))
	}

	query := "SELECT id, type, severity, title, message, affected_services, affected_teams, delivered, created_at FROM notifications"
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying notifications: %w", err)
	}
	defer rows.Close()

	var result []Notification
	for rows.Next() {
		n, err := scanRows(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *n)
	}
	return result, rows.Err()
}

// MarkDelivered sets delivered=1 for the given notification.
func (s *Store) MarkDelivered(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, "UPDATE notifications SET delivered = 1 WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("marking notification delivered: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("notification %s not found", id)
	}
	return nil
}

// GetPending returns all undelivered notifications.
func (s *Store) GetPending(ctx context.Context) ([]Notification, error) {
	delivered := false
	return s.List(ctx, ListFilter{Delivered: &delivered})
}

// SetPreference upserts a notification preference.
func (s *Store) SetPreference(ctx context.Context, pref Preference) error {
	var webhookURL sql.NullString
	if pref.WebhookURL != "" {
		webhookURL = sql.NullString{String: pref.WebhookURL, Valid: true}
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO notification_preferences (team_id, channel, severity_filter, digest_frequency, webhook_url)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(team_id, channel) DO UPDATE SET
			severity_filter = excluded.severity_filter,
			digest_frequency = excluded.digest_frequency,
			webhook_url = excluded.webhook_url`,
		pref.TeamID, pref.Channel, string(pref.SeverityFilter),
		string(pref.DigestFrequency), webhookURL,
	)
	if err != nil {
		return fmt.Errorf("upserting preference: %w", err)
	}
	return nil
}

// GetPreferences returns all notification preferences for a team.
func (s *Store) GetPreferences(ctx context.Context, teamID string) ([]Preference, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT team_id, channel, severity_filter, digest_frequency, webhook_url
		FROM notification_preferences WHERE team_id = ?`, teamID)
	if err != nil {
		return nil, fmt.Errorf("querying preferences: %w", err)
	}
	defer rows.Close()

	var prefs []Preference
	for rows.Next() {
		var p Preference
		var webhookURL sql.NullString
		var sevFilter, digestFreq string

		if err := rows.Scan(&p.TeamID, &p.Channel, &sevFilter, &digestFreq, &webhookURL); err != nil {
			return nil, fmt.Errorf("scanning preference: %w", err)
		}
		p.SeverityFilter = Severity(sevFilter)
		p.DigestFrequency = DigestFrequency(digestFreq)
		if webhookURL.Valid {
			p.WebhookURL = webhookURL.String
		}
		prefs = append(prefs, p)
	}
	return prefs, rows.Err()
}

// scanner is implemented by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanInto(sc scanner) (*Notification, error) {
	var (
		n                          Notification
		ntype, severity            string
		servicesJSON, teamsJSON    string
		delivered                  int
		ts                         string
	)

	err := sc.Scan(&n.ID, &ntype, &severity, &n.Title, &n.Message,
		&servicesJSON, &teamsJSON, &delivered, &ts)
	if err != nil {
		return nil, err
	}

	n.Type = NotificationType(ntype)
	n.Severity = Severity(severity)
	n.Delivered = delivered != 0

	if t, parseErr := time.Parse(time.DateTime, ts); parseErr == nil {
		n.CreatedAt = t
	} else if t, parseErr := time.Parse("2006-01-02T15:04:05Z", ts); parseErr == nil {
		n.CreatedAt = t
	}

	if err := json.Unmarshal([]byte(servicesJSON), &n.AffectedServices); err != nil {
		n.AffectedServices = nil
	}
	if err := json.Unmarshal([]byte(teamsJSON), &n.AffectedTeams); err != nil {
		n.AffectedTeams = nil
	}

	return &n, nil
}

func scanNotification(row *sql.Row) (*Notification, error) {
	return scanInto(row)
}

func scanRows(rows *sql.Rows) (*Notification, error) {
	return scanInto(rows)
}
