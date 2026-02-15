package notifications

import "time"

// Severity indicates the importance of a notification.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// NotificationType categorises the change that triggered the notification.
type NotificationType string

const (
	TypeServiceAdded       NotificationType = "service_added"
	TypeServiceRemoved     NotificationType = "service_removed"
	TypeRelationshipChanged NotificationType = "relationship_changed"
	TypeOwnershipChanged   NotificationType = "ownership_changed"
	TypeDocUpdated         NotificationType = "doc_updated"
	TypeContextChanged     NotificationType = "context_changed"
	TypeStalenessDetected  NotificationType = "staleness_detected"
)

// DigestFrequency controls how often digest summaries are sent.
type DigestFrequency string

const (
	FreqRealtime DigestFrequency = "realtime"
	FreqDaily    DigestFrequency = "daily"
	FreqWeekly   DigestFrequency = "weekly"
)

// Notification is a single change notification record.
type Notification struct {
	ID               string           `json:"id"`
	Type             NotificationType `json:"type"`
	Severity         Severity         `json:"severity"`
	Title            string           `json:"title"`
	Message          string           `json:"message"`
	AffectedServices []string         `json:"affected_services"`
	AffectedTeams    []string         `json:"affected_teams"`
	Delivered        bool             `json:"delivered"`
	CreatedAt        time.Time        `json:"created_at"`
}

// Preference stores a team's notification delivery preferences.
type Preference struct {
	TeamID          string          `json:"team_id"`
	Channel         string          `json:"channel"`
	SeverityFilter  Severity        `json:"severity_filter"`
	DigestFrequency DigestFrequency `json:"digest_frequency"`
	WebhookURL      string          `json:"webhook_url,omitempty"`
}
