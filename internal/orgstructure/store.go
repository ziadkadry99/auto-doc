package orgstructure

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ziadkadry99/auto-doc/internal/db"
)

// Store provides CRUD operations for teams, members, and service ownership.
type Store struct {
	db *db.DB
}

// NewStore creates a new orgstructure store.
func NewStore(d *db.DB) *Store {
	return &Store{db: d}
}

// CreateTeam inserts a new team.
func (s *Store) CreateTeam(ctx context.Context, t *Team) error {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	if t.Source == "" {
		t.Source = "manual"
	}
	now := time.Now().UTC()
	t.CreatedAt = now
	t.UpdatedAt = now

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO teams (id, name, display_name, source, source_id, slack_channel, email, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Name, t.DisplayName, t.Source, t.SourceID, t.SlackChannel, t.Email, t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("creating team: %w", err)
	}
	return nil
}

// GetTeam retrieves a team by ID, including its members.
func (s *Store) GetTeam(ctx context.Context, id string) (*Team, error) {
	t := &Team{}
	var sourceID, slackChannel, email sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, display_name, source, source_id, slack_channel, email, created_at, updated_at
		 FROM teams WHERE id = ?`, id,
	).Scan(&t.ID, &t.Name, &t.DisplayName, &t.Source, &sourceID, &slackChannel, &email, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting team: %w", err)
	}
	t.SourceID = sourceID.String
	t.SlackChannel = slackChannel.String
	t.Email = email.String

	members, err := s.ListMembers(ctx, id)
	if err != nil {
		return nil, err
	}
	t.Members = members
	return t, nil
}

// ListTeams returns all teams (without members populated).
func (s *Store) ListTeams(ctx context.Context) ([]Team, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, display_name, source, source_id, slack_channel, email, created_at, updated_at
		 FROM teams ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing teams: %w", err)
	}
	defer rows.Close()

	var teams []Team
	for rows.Next() {
		var t Team
		var sourceID, slackChannel, email sql.NullString
		if err := rows.Scan(&t.ID, &t.Name, &t.DisplayName, &t.Source, &sourceID, &slackChannel, &email, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning team: %w", err)
		}
		t.SourceID = sourceID.String
		t.SlackChannel = slackChannel.String
		t.Email = email.String
		teams = append(teams, t)
	}
	return teams, rows.Err()
}

// UpdateTeam updates a team's fields.
func (s *Store) UpdateTeam(ctx context.Context, t *Team) error {
	t.UpdatedAt = time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`UPDATE teams SET name=?, display_name=?, source=?, source_id=?, slack_channel=?, email=?, updated_at=?
		 WHERE id=?`,
		t.Name, t.DisplayName, t.Source, t.SourceID, t.SlackChannel, t.Email, t.UpdatedAt, t.ID,
	)
	if err != nil {
		return fmt.Errorf("updating team: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// DeleteTeam removes a team by ID (cascades to members and ownership).
func (s *Store) DeleteTeam(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM teams WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("deleting team: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// AddMember adds a member to a team.
func (s *Store) AddMember(ctx context.Context, m *TeamMember) error {
	if m.Role == "" {
		m.Role = "member"
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO team_members (team_id, user_id, role) VALUES (?, ?, ?)
		 ON CONFLICT(team_id, user_id) DO UPDATE SET role=excluded.role`,
		m.TeamID, m.UserID, m.Role,
	)
	if err != nil {
		return fmt.Errorf("adding member: %w", err)
	}
	return nil
}

// RemoveMember removes a member from a team.
func (s *Store) RemoveMember(ctx context.Context, teamID, userID string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM team_members WHERE team_id=? AND user_id=?`, teamID, userID)
	if err != nil {
		return fmt.Errorf("removing member: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// ListMembers returns all members of a team.
func (s *Store) ListMembers(ctx context.Context, teamID string) ([]TeamMember, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT team_id, user_id, role FROM team_members WHERE team_id=? ORDER BY user_id`, teamID)
	if err != nil {
		return nil, fmt.Errorf("listing members: %w", err)
	}
	defer rows.Close()

	var members []TeamMember
	for rows.Next() {
		var m TeamMember
		if err := rows.Scan(&m.TeamID, &m.UserID, &m.Role); err != nil {
			return nil, fmt.Errorf("scanning member: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// SetOwnership sets or updates ownership of a repo by a team.
func (s *Store) SetOwnership(ctx context.Context, o *ServiceOwnership) error {
	if o.Confidence == "" {
		o.Confidence = "auto_detected"
	}
	if o.Source == "" {
		o.Source = "codeowners"
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO service_ownership (team_id, repo_id, confidence, source) VALUES (?, ?, ?, ?)
		 ON CONFLICT(team_id, repo_id) DO UPDATE SET confidence=excluded.confidence, source=excluded.source`,
		o.TeamID, o.RepoID, o.Confidence, o.Source,
	)
	if err != nil {
		return fmt.Errorf("setting ownership: %w", err)
	}
	return nil
}

// GetOwnership returns ownership info for a specific repo.
func (s *Store) GetOwnership(ctx context.Context, repoID string) ([]ServiceOwnership, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT team_id, repo_id, confidence, source FROM service_ownership WHERE repo_id=?`, repoID)
	if err != nil {
		return nil, fmt.Errorf("getting ownership: %w", err)
	}
	defer rows.Close()

	var ownerships []ServiceOwnership
	for rows.Next() {
		var o ServiceOwnership
		if err := rows.Scan(&o.TeamID, &o.RepoID, &o.Confidence, &o.Source); err != nil {
			return nil, fmt.Errorf("scanning ownership: %w", err)
		}
		ownerships = append(ownerships, o)
	}
	return ownerships, rows.Err()
}

// ListOwnerships returns all service ownerships for a team.
func (s *Store) ListOwnerships(ctx context.Context, teamID string) ([]ServiceOwnership, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT team_id, repo_id, confidence, source FROM service_ownership WHERE team_id=?`, teamID)
	if err != nil {
		return nil, fmt.Errorf("listing ownerships: %w", err)
	}
	defer rows.Close()

	var ownerships []ServiceOwnership
	for rows.Next() {
		var o ServiceOwnership
		if err := rows.Scan(&o.TeamID, &o.RepoID, &o.Confidence, &o.Source); err != nil {
			return nil, fmt.Errorf("scanning ownership: %w", err)
		}
		ownerships = append(ownerships, o)
	}
	return ownerships, rows.Err()
}
