package orgstructure

import "time"

// Team represents an engineering team in the organization.
type Team struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	DisplayName  string       `json:"display_name"`
	Source       string       `json:"source"`
	SourceID     string       `json:"source_id,omitempty"`
	SlackChannel string       `json:"slack_channel,omitempty"`
	Email        string       `json:"email,omitempty"`
	Members      []TeamMember `json:"members,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

// TeamMember represents a member of a team.
type TeamMember struct {
	TeamID string `json:"team_id"`
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

// ServiceOwnership maps a team to a repository they own.
type ServiceOwnership struct {
	TeamID     string `json:"team_id"`
	RepoID     string `json:"repo_id"`
	Confidence string `json:"confidence"`
	Source     string `json:"source"`
}
