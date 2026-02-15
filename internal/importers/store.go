package importers

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/ziadkadry99/auto-doc/internal/db"
)

// Store manages persistence of import sources.
type Store struct {
	db *db.DB
}

// NewStore creates a new importers store.
func NewStore(database *db.DB) *Store {
	return &Store{db: database}
}

// Create adds a new import source configuration.
func (s *Store) Create(ctx context.Context, src ImportSource) (*ImportSource, error) {
	if src.ID == "" {
		src.ID = uuid.New().String()
	}
	if src.Status == "" {
		src.Status = "configured"
	}
	if src.Config == "" {
		src.Config = "{}"
	}
	src.CreatedAt = time.Now().UTC()

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO import_sources (id, type, name, config, status, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		src.ID, src.Type, src.Name, src.Config, src.Status, src.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting import source: %w", err)
	}
	return &src, nil
}

// GetByID retrieves an import source by ID.
func (s *Store) GetByID(ctx context.Context, id string) (*ImportSource, error) {
	var src ImportSource
	var lastImported sql.NullTime

	err := s.db.QueryRowContext(ctx,
		`SELECT id, type, name, config, last_imported, status, created_at
		 FROM import_sources WHERE id = ?`, id,
	).Scan(&src.ID, &src.Type, &src.Name, &src.Config, &lastImported, &src.Status, &src.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting import source: %w", err)
	}
	if lastImported.Valid {
		src.LastImported = &lastImported.Time
	}
	return &src, nil
}

// List returns all configured import sources.
func (s *Store) List(ctx context.Context) ([]ImportSource, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, type, name, config, last_imported, status, created_at
		 FROM import_sources ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing import sources: %w", err)
	}
	defer rows.Close()

	var sources []ImportSource
	for rows.Next() {
		var src ImportSource
		var lastImported sql.NullTime
		if err := rows.Scan(&src.ID, &src.Type, &src.Name, &src.Config, &lastImported, &src.Status, &src.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning import source: %w", err)
		}
		if lastImported.Valid {
			src.LastImported = &lastImported.Time
		}
		sources = append(sources, src)
	}
	return sources, rows.Err()
}

// UpdateStatus updates the status and last_imported timestamp.
func (s *Store) UpdateStatus(ctx context.Context, id, status string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx,
		`UPDATE import_sources SET status = ?, last_imported = ? WHERE id = ?`,
		status, now, id,
	)
	return err
}

// Delete removes an import source.
func (s *Store) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM import_sources WHERE id = ?`, id)
	return err
}
