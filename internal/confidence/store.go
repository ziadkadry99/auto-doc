package confidence

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ziadkadry99/auto-doc/internal/db"
)

// Store provides persistence for confidence metadata.
type Store struct {
	db *db.DB
}

// NewStore creates a new confidence metadata store.
func NewStore(database *db.DB) *Store {
	return &Store{db: database}
}

// Set upserts a confidence metadata record. If ID is empty, a new UUID is generated.
func (s *Store) Set(ctx context.Context, meta Metadata) error {
	if meta.ID == "" {
		meta.ID = uuid.NewString()
	}

	var attributedAt *string
	if meta.AttributedAt != nil {
		t := meta.AttributedAt.UTC().Format(time.DateTime)
		attributedAt = &t
	}

	stale := 0
	if meta.PotentiallyStale {
		stale = 1
	}

	lastVerified := meta.LastVerified
	if lastVerified.IsZero() {
		lastVerified = time.Now().UTC()
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO confidence_metadata (id, entity_type, entity_id, confidence, source, source_detail, attributed_to, attributed_at, last_verified, potentially_stale, stale_reason)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(entity_type, entity_id) DO UPDATE SET
			confidence = excluded.confidence,
			source = excluded.source,
			source_detail = excluded.source_detail,
			attributed_to = excluded.attributed_to,
			attributed_at = excluded.attributed_at,
			last_verified = excluded.last_verified,
			potentially_stale = excluded.potentially_stale,
			stale_reason = excluded.stale_reason`,
		meta.ID,
		string(meta.EntityType),
		meta.EntityID,
		string(meta.Confidence),
		string(meta.Source),
		nullString(meta.SourceDetail),
		nullString(meta.AttributedTo),
		attributedAt,
		lastVerified.UTC().Format(time.DateTime),
		stale,
		nullString(meta.StaleReason),
	)
	return err
}

// Get retrieves confidence metadata for a specific entity.
func (s *Store) Get(ctx context.Context, entityType EntityType, entityID string) (*Metadata, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, entity_type, entity_id, confidence, source, source_detail, attributed_to, attributed_at, last_verified, potentially_stale, stale_reason
		FROM confidence_metadata
		WHERE entity_type = ? AND entity_id = ?`,
		string(entityType), entityID,
	)
	return scanMetadata(row)
}

// List returns confidence metadata matching the given filter.
func (s *Store) List(ctx context.Context, filter ListFilter) ([]Metadata, error) {
	var conditions []string
	var args []interface{}

	if filter.EntityType != "" {
		conditions = append(conditions, "entity_type = ?")
		args = append(args, string(filter.EntityType))
	}
	if filter.Confidence != "" {
		conditions = append(conditions, "confidence = ?")
		args = append(args, string(filter.Confidence))
	}
	if filter.StaleOnly {
		conditions = append(conditions, "potentially_stale = 1")
	}

	query := "SELECT id, entity_type, entity_id, confidence, source, source_detail, attributed_to, attributed_at, last_verified, potentially_stale, stale_reason FROM confidence_metadata"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY last_verified DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Metadata
	for rows.Next() {
		m, err := scanMetadataRows(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, *m)
	}
	return results, rows.Err()
}

// MarkStale marks an entity's metadata as potentially stale.
func (s *Store) MarkStale(ctx context.Context, entityType EntityType, entityID string, reason string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE confidence_metadata SET potentially_stale = 1, stale_reason = ?
		WHERE entity_type = ? AND entity_id = ?`,
		reason, string(entityType), entityID,
	)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("no confidence metadata found for %s/%s", entityType, entityID)
	}
	return nil
}

// MarkVerified resets the staleness flag and updates the last verified timestamp.
func (s *Store) MarkVerified(ctx context.Context, entityType EntityType, entityID string) error {
	now := time.Now().UTC().Format(time.DateTime)
	result, err := s.db.ExecContext(ctx, `
		UPDATE confidence_metadata SET potentially_stale = 0, stale_reason = NULL, last_verified = ?
		WHERE entity_type = ? AND entity_id = ?`,
		now, string(entityType), entityID,
	)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("no confidence metadata found for %s/%s", entityType, entityID)
	}
	return nil
}

// Stats returns aggregate counts of confidence metadata.
func (s *Store) Stats(ctx context.Context) (*Stats, error) {
	stats := &Stats{ByConfidence: make(map[Level]int)}

	rows, err := s.db.QueryContext(ctx, `
		SELECT confidence, COUNT(*) FROM confidence_metadata GROUP BY confidence`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var level string
		var count int
		if err := rows.Scan(&level, &count); err != nil {
			return nil, err
		}
		stats.ByConfidence[Level(level)] = count
		stats.TotalEntities += count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM confidence_metadata WHERE potentially_stale = 1`).Scan(&stats.StaleCount)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

// scanner is implemented by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...interface{}) error
}

func scanMetadata(s scanner) (*Metadata, error) {
	var m Metadata
	var entityType, entityID, confidence, source string
	var sourceDetail, attributedTo, staleReason sql.NullString
	var attributedAt sql.NullString
	var lastVerified string
	var potentiallyStale int

	err := s.Scan(&m.ID, &entityType, &entityID, &confidence, &source,
		&sourceDetail, &attributedTo, &attributedAt, &lastVerified,
		&potentiallyStale, &staleReason)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	m.EntityType = EntityType(entityType)
	m.EntityID = entityID
	m.Confidence = Level(confidence)
	m.Source = Source(source)
	m.SourceDetail = sourceDetail.String
	m.AttributedTo = attributedTo.String
	m.PotentiallyStale = potentiallyStale != 0
	m.StaleReason = staleReason.String

	if attributedAt.Valid {
		t, err := time.Parse(time.DateTime, attributedAt.String)
		if err == nil {
			m.AttributedAt = &t
		}
	}

	t, err := time.Parse(time.DateTime, lastVerified)
	if err == nil {
		m.LastVerified = t
	}

	return &m, nil
}

func scanMetadataRows(rows *sql.Rows) (*Metadata, error) {
	return scanMetadata(rows)
}

func nullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
