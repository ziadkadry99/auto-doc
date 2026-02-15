package audit

import "time"

// ActorType identifies who performed an action.
type ActorType string

const (
	ActorUser   ActorType = "user"
	ActorSystem ActorType = "system"
	ActorBot    ActorType = "bot"
)

// Action describes what was done.
type Action string

const (
	ActionContextProvided     Action = "context_provided"
	ActionContextCorrected    Action = "context_corrected"
	ActionQuestionAnswered    Action = "question_answered"
	ActionOwnershipConfirmed  Action = "ownership_confirmed"
	ActionDocUpdated          Action = "doc_updated"
	ActionDocCreated          Action = "doc_created"
	ActionRelationshipAdded   Action = "relationship_added"
	ActionRelationshipRemoved Action = "relationship_removed"
	ActionFactInvalidated     Action = "fact_invalidated"
	ActionOverrideApplied     Action = "override_applied"
	ActionOverrideExpired     Action = "override_expired"
)

// Scope describes the level at which an action applies.
type Scope string

const (
	ScopeOrg      Scope = "org"
	ScopeDomain   Scope = "domain"
	ScopeService  Scope = "service"
	ScopeEndpoint Scope = "endpoint"
	ScopeFlow     Scope = "flow"
	ScopeTopic    Scope = "topic"
)

// Entry is a single audit trail record.
type Entry struct {
	ID               string
	Timestamp        time.Time
	ActorType        ActorType
	ActorID          string
	Action           Action
	Scope            Scope
	ScopeID          string
	Summary          string
	Detail           string
	SourceFact       string
	AffectedEntities []string
	ConversationID   string
	PreviousValue    string
	NewValue         string
}
