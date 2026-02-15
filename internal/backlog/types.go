package backlog

import "time"

// Status represents the lifecycle stage of a knowledge question.
type Status string

const (
	StatusOpen     Status = "open"
	StatusAnswered Status = "answered"
	StatusVerified Status = "verified"
	StatusRetired  Status = "retired"
)

// Category categorizes what kind of knowledge a question seeks.
type Category string

const (
	CategoryGeneral      Category = "general"
	CategoryBusiness     Category = "business_context"
	CategoryOwnership    Category = "ownership"
	CategoryRelationship Category = "relationship"
	CategoryDataFlow     Category = "data_flow"
	CategoryDeployment   Category = "deployment"
	CategorySecurity     Category = "security"
)

// Question represents a gap in the system's knowledge that needs a human answer.
type Question struct {
	ID           string    `json:"id"`
	RepoID       string    `json:"repo_id"`
	Question     string    `json:"question"`
	Category     Category  `json:"category"`
	Priority     int       `json:"priority"` // 0-100, higher = more important
	Status       Status    `json:"status"`
	Source       string    `json:"source"`        // "system", "import", "user"
	SourceDetail string    `json:"source_detail"` // e.g. "detected unknown Kafka topic"
	Answer       string    `json:"answer,omitempty"`
	AnsweredBy   string    `json:"answered_by,omitempty"`
	AnsweredAt   *time.Time `json:"answered_at,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ListFilter controls which questions to return.
type ListFilter struct {
	RepoID   string
	Status   Status
	Category Category
	MinPriority int
	Limit    int
	Offset   int
}
