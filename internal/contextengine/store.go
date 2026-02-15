package contextengine

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/ziadkadry99/auto-doc/internal/db"
)

// Store manages persistence of facts and chat sessions.
type Store struct {
	db *db.DB
}

// NewStore creates a new context engine store.
func NewStore(database *db.DB) *Store {
	return &Store{db: database}
}

// SaveFact inserts or updates a fact. If it already exists (same repo/scope/scope_id/key),
// the old version is superseded and a new version is created.
func (s *Store) SaveFact(ctx context.Context, f Fact) (*Fact, error) {
	if f.ID == "" {
		f.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	f.CreatedAt = now
	f.UpdatedAt = now

	// Check for existing fact with same key.
	var existingID string
	var existingVersion int
	err := s.db.QueryRowContext(ctx,
		`SELECT id, version FROM facts
		 WHERE repo_id = ? AND scope = ? AND scope_id = ? AND key = ? AND superseded_by IS NULL
		 ORDER BY version DESC LIMIT 1`,
		f.RepoID, f.Scope, f.ScopeID, f.Key,
	).Scan(&existingID, &existingVersion)

	if err == nil {
		// Supersede the old fact.
		f.Version = existingVersion + 1
		_, err = s.db.ExecContext(ctx,
			`UPDATE facts SET superseded_by = ?, updated_at = ? WHERE id = ?`,
			f.ID, now, existingID,
		)
		if err != nil {
			return nil, fmt.Errorf("superseding old fact: %w", err)
		}
	} else if err != sql.ErrNoRows {
		return nil, fmt.Errorf("checking existing fact: %w", err)
	} else {
		f.Version = 1
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO facts (id, repo_id, scope, scope_id, key, value, source, provided_by, created_at, updated_at, version)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.RepoID, f.Scope, f.ScopeID, f.Key, f.Value, f.Source, f.ProvidedBy, f.CreatedAt, f.UpdatedAt, f.Version,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting fact: %w", err)
	}

	return &f, nil
}

// GetFact retrieves a fact by ID.
func (s *Store) GetFact(ctx context.Context, id string) (*Fact, error) {
	var f Fact
	var supersededBy sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, repo_id, scope, scope_id, key, value, source, provided_by, created_at, updated_at, version, superseded_by
		 FROM facts WHERE id = ?`, id,
	).Scan(&f.ID, &f.RepoID, &f.Scope, &f.ScopeID, &f.Key, &f.Value, &f.Source, &f.ProvidedBy, &f.CreatedAt, &f.UpdatedAt, &f.Version, &supersededBy)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting fact: %w", err)
	}
	f.SupersededBy = supersededBy.String
	return &f, nil
}

// GetCurrentFacts returns all current (non-superseded) facts matching the filter.
func (s *Store) GetCurrentFacts(ctx context.Context, repoID, scope, scopeID string) ([]Fact, error) {
	query := `SELECT id, repo_id, scope, scope_id, key, value, source, provided_by, created_at, updated_at, version
		 FROM facts WHERE superseded_by IS NULL`
	args := []interface{}{}

	if repoID != "" {
		query += " AND repo_id = ?"
		args = append(args, repoID)
	}
	if scope != "" {
		query += " AND scope = ?"
		args = append(args, scope)
	}
	if scopeID != "" {
		query += " AND scope_id = ?"
		args = append(args, scopeID)
	}

	query += " ORDER BY updated_at DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying facts: %w", err)
	}
	defer rows.Close()

	var facts []Fact
	for rows.Next() {
		var f Fact
		if err := rows.Scan(&f.ID, &f.RepoID, &f.Scope, &f.ScopeID, &f.Key, &f.Value, &f.Source, &f.ProvidedBy, &f.CreatedAt, &f.UpdatedAt, &f.Version); err != nil {
			return nil, fmt.Errorf("scanning fact: %w", err)
		}
		facts = append(facts, f)
	}
	return facts, rows.Err()
}

// GetFactHistory returns all versions of a fact (by repo/scope/scope_id/key).
func (s *Store) GetFactHistory(ctx context.Context, repoID, scope, scopeID, key string) ([]Fact, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, repo_id, scope, scope_id, key, value, source, provided_by, created_at, updated_at, version, superseded_by
		 FROM facts WHERE repo_id = ? AND scope = ? AND scope_id = ? AND key = ?
		 ORDER BY version ASC`,
		repoID, scope, scopeID, key,
	)
	if err != nil {
		return nil, fmt.Errorf("querying fact history: %w", err)
	}
	defer rows.Close()

	var facts []Fact
	for rows.Next() {
		var f Fact
		var supersededBy sql.NullString
		if err := rows.Scan(&f.ID, &f.RepoID, &f.Scope, &f.ScopeID, &f.Key, &f.Value, &f.Source, &f.ProvidedBy, &f.CreatedAt, &f.UpdatedAt, &f.Version, &supersededBy); err != nil {
			return nil, fmt.Errorf("scanning fact: %w", err)
		}
		f.SupersededBy = supersededBy.String
		facts = append(facts, f)
	}
	return facts, rows.Err()
}

// CreateSession creates a new chat session.
func (s *Store) CreateSession(ctx context.Context, userID string) (*Session, error) {
	sess := Session{
		ID:        uuid.New().String(),
		UserID:    userID,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO chat_sessions (id, user_id, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		sess.ID, sess.UserID, sess.CreatedAt, sess.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}
	return &sess, nil
}

// AddMessage adds a message to a chat session.
func (s *Store) AddMessage(ctx context.Context, msg ConversationMessage) (*ConversationMessage, error) {
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	if msg.Metadata == "" {
		msg.Metadata = "{}"
	}
	msg.CreatedAt = time.Now().UTC()

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO chat_messages (id, session_id, role, content, metadata, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.SessionID, msg.Role, msg.Content, msg.Metadata, msg.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("adding message: %w", err)
	}

	// Update session timestamp.
	s.db.ExecContext(ctx, `UPDATE chat_sessions SET updated_at = ? WHERE id = ?`, msg.CreatedAt, msg.SessionID)

	return &msg, nil
}

// GetMessages returns all messages for a session, ordered by creation time.
func (s *Store) GetMessages(ctx context.Context, sessionID string) ([]ConversationMessage, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, session_id, role, content, metadata, created_at
		 FROM chat_messages WHERE session_id = ? ORDER BY created_at ASC`,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying messages: %w", err)
	}
	defer rows.Close()

	var messages []ConversationMessage
	for rows.Next() {
		var m ConversationMessage
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.Metadata, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning message: %w", err)
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

// SearchFacts searches facts by value content (simple LIKE search).
func (s *Store) SearchFacts(ctx context.Context, query string, limit int) ([]Fact, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, repo_id, scope, scope_id, key, value, source, provided_by, created_at, updated_at, version
		 FROM facts WHERE superseded_by IS NULL AND (value LIKE ? OR key LIKE ?)
		 ORDER BY updated_at DESC LIMIT ?`,
		"%"+query+"%", "%"+query+"%", limit,
	)
	if err != nil {
		return nil, fmt.Errorf("searching facts: %w", err)
	}
	defer rows.Close()

	var facts []Fact
	for rows.Next() {
		var f Fact
		if err := rows.Scan(&f.ID, &f.RepoID, &f.Scope, &f.ScopeID, &f.Key, &f.Value, &f.Source, &f.ProvidedBy, &f.CreatedAt, &f.UpdatedAt, &f.Version); err != nil {
			return nil, fmt.Errorf("scanning fact: %w", err)
		}
		facts = append(facts, f)
	}
	return facts, rows.Err()
}

// CountSessions returns the total number of chat sessions.
func (s *Store) CountSessions(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM chat_sessions`).Scan(&count)
	return count, err
}

// FactsToJSON converts facts to a JSON string for LLM context.
func FactsToJSON(facts []Fact) string {
	if len(facts) == 0 {
		return "[]"
	}
	b, err := json.Marshal(facts)
	if err != nil {
		return "[]"
	}
	return string(b)
}
