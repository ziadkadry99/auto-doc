package contextengine

import "time"

// Fact represents a single piece of structured business knowledge.
type Fact struct {
	ID           string    `json:"id"`
	RepoID       string    `json:"repo_id"`
	Scope        string    `json:"scope"`    // "service", "endpoint", "flow", "org", "domain"
	ScopeID      string    `json:"scope_id"` // e.g. service name or endpoint path
	Key          string    `json:"key"`      // e.g. "description", "purpose", "owner"
	Value        string    `json:"value"`
	Source       string    `json:"source"` // "user", "system", "import"
	ProvidedBy   string    `json:"provided_by"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Version      int       `json:"version"`
	SupersededBy string    `json:"superseded_by,omitempty"`
}

// ExtractedFact is a fact parsed from user input by the LLM.
type ExtractedFact struct {
	Scope       string `json:"scope"`
	ScopeID     string `json:"scope_id"`
	Key         string `json:"key"`
	Value       string `json:"value"`
	Confidence  string `json:"confidence"` // "high", "medium", "low"
	Explanation string `json:"explanation"`
}

// ContextUpdate represents the result of processing user input.
type ContextUpdate struct {
	Facts           []ExtractedFact  `json:"facts"`
	Clarifications  []Clarification  `json:"clarifications,omitempty"`
	AffectedDocs    []string         `json:"affected_docs,omitempty"`
	Summary         string           `json:"summary"`
}

// Clarification is a follow-up question the engine needs answered.
type Clarification struct {
	Question string   `json:"question"`
	Options  []string `json:"options,omitempty"`
	Context  string   `json:"context"`
}

// Correction represents a user correcting a previously stored fact.
type Correction struct {
	OriginalFactID string `json:"original_fact_id"`
	NewValue       string `json:"new_value"`
	Reason         string `json:"reason"`
}

// ConversationMessage is a single message in a context conversation.
type ConversationMessage struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Role      string    `json:"role"` // "user", "assistant", "system"
	Content   string    `json:"content"`
	Metadata  string    `json:"metadata"`
	CreatedAt time.Time `json:"created_at"`
}

// Session represents a chat session.
type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
