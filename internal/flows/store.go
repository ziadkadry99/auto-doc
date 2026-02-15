package flows

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ziadkadry99/auto-doc/internal/db"
)

// Store provides CRUD operations for flows.
type Store struct {
	db *db.DB
}

// NewStore creates a new flows store.
func NewStore(d *db.DB) *Store {
	return &Store{db: d}
}

// CreateFlow inserts a new flow.
func (s *Store) CreateFlow(ctx context.Context, f *Flow) error {
	if f.ID == "" {
		f.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	f.CreatedAt = now
	f.UpdatedAt = now

	if f.Services == nil {
		f.Services = []string{}
	}
	servicesJSON, err := json.Marshal(f.Services)
	if err != nil {
		return fmt.Errorf("marshaling services: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO flows (id, name, description, narrative, mermaid_diagram, services, entry_point, exit_point, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.Name, f.Description, f.Narrative, f.MermaidDiagram,
		string(servicesJSON), f.EntryPoint, f.ExitPoint, f.CreatedAt, f.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("creating flow: %w", err)
	}
	return nil
}

// GetFlow retrieves a flow by ID.
func (s *Store) GetFlow(ctx context.Context, id string) (*Flow, error) {
	f := &Flow{}
	var servicesJSON string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, description, narrative, mermaid_diagram, services, entry_point, exit_point, created_at, updated_at
		 FROM flows WHERE id = ?`, id,
	).Scan(&f.ID, &f.Name, &f.Description, &f.Narrative, &f.MermaidDiagram,
		&servicesJSON, &f.EntryPoint, &f.ExitPoint, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting flow: %w", err)
	}
	if err := json.Unmarshal([]byte(servicesJSON), &f.Services); err != nil {
		return nil, fmt.Errorf("unmarshaling services: %w", err)
	}
	return f, nil
}

// ListFlows returns all flows.
func (s *Store) ListFlows(ctx context.Context) ([]Flow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, description, narrative, mermaid_diagram, services, entry_point, exit_point, created_at, updated_at
		 FROM flows ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing flows: %w", err)
	}
	defer rows.Close()

	var result []Flow
	for rows.Next() {
		var f Flow
		var servicesJSON string
		if err := rows.Scan(&f.ID, &f.Name, &f.Description, &f.Narrative, &f.MermaidDiagram,
			&servicesJSON, &f.EntryPoint, &f.ExitPoint, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning flow: %w", err)
		}
		if err := json.Unmarshal([]byte(servicesJSON), &f.Services); err != nil {
			return nil, fmt.Errorf("unmarshaling services: %w", err)
		}
		result = append(result, f)
	}
	return result, rows.Err()
}

// UpdateFlow updates a flow's fields.
func (s *Store) UpdateFlow(ctx context.Context, f *Flow) error {
	f.UpdatedAt = time.Now().UTC()
	if f.Services == nil {
		f.Services = []string{}
	}
	servicesJSON, err := json.Marshal(f.Services)
	if err != nil {
		return fmt.Errorf("marshaling services: %w", err)
	}

	res, err := s.db.ExecContext(ctx,
		`UPDATE flows SET name=?, description=?, narrative=?, mermaid_diagram=?, services=?, entry_point=?, exit_point=?, updated_at=?
		 WHERE id=?`,
		f.Name, f.Description, f.Narrative, f.MermaidDiagram,
		string(servicesJSON), f.EntryPoint, f.ExitPoint, f.UpdatedAt, f.ID,
	)
	if err != nil {
		return fmt.Errorf("updating flow: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// DeleteFlow removes a flow by ID.
func (s *Store) DeleteFlow(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM flows WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("deleting flow: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// SearchFlows searches flows by name or description.
func (s *Store) SearchFlows(ctx context.Context, query string) ([]Flow, error) {
	pattern := "%" + query + "%"
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, description, narrative, mermaid_diagram, services, entry_point, exit_point, created_at, updated_at
		 FROM flows WHERE name LIKE ? OR description LIKE ? ORDER BY name`,
		pattern, pattern,
	)
	if err != nil {
		return nil, fmt.Errorf("searching flows: %w", err)
	}
	defer rows.Close()

	var result []Flow
	for rows.Next() {
		var f Flow
		var servicesJSON string
		if err := rows.Scan(&f.ID, &f.Name, &f.Description, &f.Narrative, &f.MermaidDiagram,
			&servicesJSON, &f.EntryPoint, &f.ExitPoint, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning flow: %w", err)
		}
		if err := json.Unmarshal([]byte(servicesJSON), &f.Services); err != nil {
			return nil, fmt.Errorf("unmarshaling services: %w", err)
		}
		result = append(result, f)
	}
	return result, rows.Err()
}
