package confidence

import "time"

// Level represents the confidence level of a piece of metadata.
type Level string

const (
	LevelAutoDetected  Level = "auto_detected"
	LevelConfirmed     Level = "confirmed"
	LevelHumanProvided Level = "human_provided"
	LevelExternalImport Level = "external_import"
	LevelAIInferred    Level = "ai_inferred"
)

// EntityType identifies what kind of entity the confidence metadata describes.
type EntityType string

const (
	EntityRelationship EntityType = "relationship"
	EntityDescription  EntityType = "description"
	EntityOwnership    EntityType = "ownership"
	EntityEntryPoint   EntityType = "entry_point"
	EntityExitPoint    EntityType = "exit_point"
	EntityFlowStep     EntityType = "flow_step"
)

// Source identifies how a piece of information was obtained.
type Source string

const (
	SourceCodeAnalysis     Source = "code_analysis"
	SourceUserConversation Source = "user_conversation"
	SourceConfluence       Source = "confluence"
	SourceReadme           Source = "readme"
	SourceCodeowners       Source = "codeowners"
	SourceGitHub           Source = "github"
)

// Metadata holds confidence and attribution information for an entity.
type Metadata struct {
	ID               string     `json:"id"`
	EntityType       EntityType `json:"entity_type"`
	EntityID         string     `json:"entity_id"`
	Confidence       Level      `json:"confidence"`
	Source           Source     `json:"source"`
	SourceDetail     string     `json:"source_detail,omitempty"`
	AttributedTo     string     `json:"attributed_to,omitempty"`
	AttributedAt     *time.Time `json:"attributed_at,omitempty"`
	LastVerified     time.Time  `json:"last_verified"`
	PotentiallyStale bool       `json:"potentially_stale"`
	StaleReason      string     `json:"stale_reason,omitempty"`
}

// ListFilter controls which metadata entries are returned by List.
type ListFilter struct {
	EntityType EntityType `json:"entity_type,omitempty"`
	Confidence Level      `json:"confidence,omitempty"`
	StaleOnly  bool       `json:"stale_only,omitempty"`
	Limit      int        `json:"limit,omitempty"`
	Offset     int        `json:"offset,omitempty"`
}

// Stats holds aggregate confidence statistics.
type Stats struct {
	TotalEntities int          `json:"total_entities"`
	ByConfidence  map[Level]int `json:"by_confidence"`
	StaleCount    int          `json:"stale_count"`
}
