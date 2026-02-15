package audit

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

// Store provides CRUD operations for audit entries.
type Store struct {
	db *db.DB
}

// NewStore creates a Store backed by the given database.
func NewStore(database *db.DB) *Store {
	return &Store{db: database}
}

// Log inserts a new audit entry. If entry.ID is empty a UUID is generated.
func (s *Store) Log(ctx context.Context, entry Entry) error {
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}

	affected, err := json.Marshal(entry.AffectedEntities)
	if err != nil {
		return fmt.Errorf("marshalling affected entities: %w", err)
	}

	var sourceFact, conversationID, previousValue, newValue sql.NullString
	if entry.SourceFact != "" {
		sourceFact = sql.NullString{String: entry.SourceFact, Valid: true}
	}
	if entry.ConversationID != "" {
		conversationID = sql.NullString{String: entry.ConversationID, Valid: true}
	}
	if entry.PreviousValue != "" {
		previousValue = sql.NullString{String: entry.PreviousValue, Valid: true}
	}
	if entry.NewValue != "" {
		newValue = sql.NullString{String: entry.NewValue, Valid: true}
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO audit_entries (
			id, actor_type, actor_id, action, scope, scope_id,
			summary, detail, source_fact, affected_entities,
			conversation_id, previous_value, new_value
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.ID,
		string(entry.ActorType),
		entry.ActorID,
		string(entry.Action),
		string(entry.Scope),
		entry.ScopeID,
		entry.Summary,
		entry.Detail,
		sourceFact,
		string(affected),
		conversationID,
		previousValue,
		newValue,
	)
	if err != nil {
		return fmt.Errorf("inserting audit entry: %w", err)
	}
	return nil
}

// GetByID retrieves a single audit entry.
func (s *Store) GetByID(ctx context.Context, id string) (*Entry, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, timestamp, actor_type, actor_id, action, scope, scope_id,
			   summary, detail, source_fact, affected_entities,
			   conversation_id, previous_value, new_value
		FROM audit_entries WHERE id = ?`, id)

	return scanEntry(row)
}

// QueryFilter controls which audit entries are returned by Query.
type QueryFilter struct {
	ActorID         string
	Scope           Scope
	ScopeID         string
	Action          Action
	Since           *time.Time
	Until           *time.Time
	AffectedService string
	Limit           int
	Offset          int
}

// Query returns audit entries matching the filter.
func (s *Store) Query(ctx context.Context, filter QueryFilter) ([]Entry, error) {
	var (
		clauses []string
		args    []any
	)

	if filter.ActorID != "" {
		clauses = append(clauses, "actor_id = ?")
		args = append(args, filter.ActorID)
	}
	if filter.Scope != "" {
		clauses = append(clauses, "scope = ?")
		args = append(args, string(filter.Scope))
	}
	if filter.ScopeID != "" {
		clauses = append(clauses, "scope_id = ?")
		args = append(args, filter.ScopeID)
	}
	if filter.Action != "" {
		clauses = append(clauses, "action = ?")
		args = append(args, string(filter.Action))
	}
	if filter.Since != nil {
		clauses = append(clauses, "timestamp >= ?")
		args = append(args, filter.Since.UTC().Format(time.DateTime))
	}
	if filter.Until != nil {
		clauses = append(clauses, "timestamp <= ?")
		args = append(args, filter.Until.UTC().Format(time.DateTime))
	}
	if filter.AffectedService != "" {
		// JSON array stored as text; use LIKE for containment check.
		clauses = append(clauses, "affected_entities LIKE ?")
		args = append(args, "%"+filter.AffectedService+"%")
	}

	query := "SELECT id, timestamp, actor_type, actor_id, action, scope, scope_id, summary, detail, source_fact, affected_entities, conversation_id, previous_value, new_value FROM audit_entries"
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY timestamp DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying audit entries: %w", err)
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		e, err := scanRows(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, *e)
	}
	return entries, rows.Err()
}

// DeleteBefore removes all audit entries older than the given time.
// Returns the number of deleted rows.
func (s *Store) DeleteBefore(ctx context.Context, before time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		"DELETE FROM audit_entries WHERE timestamp < ?",
		before.UTC().Format(time.DateTime),
	)
	if err != nil {
		return 0, fmt.Errorf("deleting old audit entries: %w", err)
	}
	return res.RowsAffected()
}

// scanner is implemented by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanInto(sc scanner) (*Entry, error) {
	var (
		e                                                    Entry
		actorType, action, scope                             string
		ts                                                   string
		affectedJSON                                         string
		sourceFact, conversationID, previousValue, newValue sql.NullString
	)

	err := sc.Scan(
		&e.ID, &ts, &actorType, &e.ActorID, &action, &scope, &e.ScopeID,
		&e.Summary, &e.Detail, &sourceFact, &affectedJSON,
		&conversationID, &previousValue, &newValue,
	)
	if err != nil {
		return nil, err
	}

	e.ActorType = ActorType(actorType)
	e.Action = Action(action)
	e.Scope = Scope(scope)

	if t, parseErr := time.Parse(time.DateTime, ts); parseErr == nil {
		e.Timestamp = t
	} else if t, parseErr := time.Parse("2006-01-02T15:04:05Z", ts); parseErr == nil {
		e.Timestamp = t
	}

	if sourceFact.Valid {
		e.SourceFact = sourceFact.String
	}
	if conversationID.Valid {
		e.ConversationID = conversationID.String
	}
	if previousValue.Valid {
		e.PreviousValue = previousValue.String
	}
	if newValue.Valid {
		e.NewValue = newValue.String
	}

	if err := json.Unmarshal([]byte(affectedJSON), &e.AffectedEntities); err != nil {
		e.AffectedEntities = nil
	}

	return &e, nil
}

func scanEntry(row *sql.Row) (*Entry, error) {
	return scanInto(row)
}

func scanRows(rows *sql.Rows) (*Entry, error) {
	return scanInto(rows)
}
