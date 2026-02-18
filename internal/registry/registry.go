package registry

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ziadkadry99/auto-doc/internal/db"
)

// Repository represents a registered repository in the central server.
type Repository struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	DisplayName   string    `json:"display_name"`
	SourceType    string    `json:"source_type"` // "local" or "git"
	SourceURL     string    `json:"source_url"`
	LocalPath     string    `json:"local_path"`
	LastCommitSHA string    `json:"last_commit_sha"`
	LastIndexedAt string    `json:"last_indexed_at"`
	Status        string    `json:"status"` // pending, indexing, ready, error
	FileCount     int       `json:"file_count"`
	Summary       string    `json:"summary"`
	CreatedAt     time.Time `json:"created_at"`
}

// ServiceLink represents a discovered dependency between two repos.
type ServiceLink struct {
	ID        string    `json:"id"`
	FromRepo  string    `json:"from_repo"`
	ToRepo    string    `json:"to_repo"`
	LinkType  string    `json:"link_type"` // http, grpc, kafka, amqp
	Reason    string    `json:"reason"`
	Endpoints []string  `json:"endpoints"`
	CreatedAt time.Time `json:"created_at"`
}

// Store provides CRUD operations for the repository registry.
type Store struct {
	db *db.DB
}

// NewStore creates a new registry store.
func NewStore(d *db.DB) *Store {
	return &Store{db: d}
}

// Add inserts a new repository.
func (s *Store) Add(ctx context.Context, repo *Repository) error {
	if repo.ID == "" {
		repo.ID = uuid.NewString()
	}
	if repo.Status == "" {
		repo.Status = "pending"
	}
	repo.CreatedAt = time.Now().UTC()

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO repositories (id, name, display_name, source_type, source_url, local_path, last_commit_sha, last_indexed_at, status, file_count, summary, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		repo.ID, repo.Name, repo.DisplayName, repo.SourceType, repo.SourceURL,
		repo.LocalPath, repo.LastCommitSHA, repo.LastIndexedAt, repo.Status,
		repo.FileCount, repo.Summary, repo.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("adding repository: %w", err)
	}
	return nil
}

// Get retrieves a repository by name.
func (s *Store) Get(ctx context.Context, name string) (*Repository, error) {
	r := &Repository{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, display_name, source_type, source_url, local_path, last_commit_sha, last_indexed_at, status, file_count, summary, created_at
		 FROM repositories WHERE name = ?`, name,
	).Scan(&r.ID, &r.Name, &r.DisplayName, &r.SourceType, &r.SourceURL,
		&r.LocalPath, &r.LastCommitSHA, &r.LastIndexedAt, &r.Status,
		&r.FileCount, &r.Summary, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting repository: %w", err)
	}
	return r, nil
}

// GetByID retrieves a repository by ID.
func (s *Store) GetByID(ctx context.Context, id string) (*Repository, error) {
	r := &Repository{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, display_name, source_type, source_url, local_path, last_commit_sha, last_indexed_at, status, file_count, summary, created_at
		 FROM repositories WHERE id = ?`, id,
	).Scan(&r.ID, &r.Name, &r.DisplayName, &r.SourceType, &r.SourceURL,
		&r.LocalPath, &r.LastCommitSHA, &r.LastIndexedAt, &r.Status,
		&r.FileCount, &r.Summary, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting repository by ID: %w", err)
	}
	return r, nil
}

// List returns all registered repositories.
func (s *Store) List(ctx context.Context) ([]Repository, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, display_name, source_type, source_url, local_path, last_commit_sha, last_indexed_at, status, file_count, summary, created_at
		 FROM repositories ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing repositories: %w", err)
	}
	defer rows.Close()

	var repos []Repository
	for rows.Next() {
		var r Repository
		if err := rows.Scan(&r.ID, &r.Name, &r.DisplayName, &r.SourceType, &r.SourceURL,
			&r.LocalPath, &r.LastCommitSHA, &r.LastIndexedAt, &r.Status,
			&r.FileCount, &r.Summary, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning repository: %w", err)
		}
		repos = append(repos, r)
	}
	return repos, rows.Err()
}

// Update modifies an existing repository record.
func (s *Store) Update(ctx context.Context, repo *Repository) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE repositories SET display_name=?, source_type=?, source_url=?, local_path=?,
		 last_commit_sha=?, last_indexed_at=?, status=?, file_count=?, summary=?
		 WHERE id=?`,
		repo.DisplayName, repo.SourceType, repo.SourceURL, repo.LocalPath,
		repo.LastCommitSHA, repo.LastIndexedAt, repo.Status, repo.FileCount,
		repo.Summary, repo.ID,
	)
	if err != nil {
		return fmt.Errorf("updating repository: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// Remove deletes a repository by name.
func (s *Store) Remove(ctx context.Context, name string) error {
	// Also delete associated service links.
	s.db.ExecContext(ctx, `DELETE FROM service_links WHERE from_repo = ? OR to_repo = ?`, name, name)

	res, err := s.db.ExecContext(ctx, `DELETE FROM repositories WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("removing repository: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// SaveLink inserts or updates a service link between repos.
func (s *Store) SaveLink(ctx context.Context, link *ServiceLink) error {
	if link.ID == "" {
		link.ID = uuid.NewString()
	}
	link.CreatedAt = time.Now().UTC()

	endpointsJSON, err := json.Marshal(link.Endpoints)
	if err != nil {
		return fmt.Errorf("marshaling endpoints: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO service_links (id, from_repo, to_repo, link_type, reason, endpoints, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(from_repo, to_repo, link_type) DO UPDATE SET reason=excluded.reason, endpoints=excluded.endpoints, created_at=excluded.created_at`,
		link.ID, link.FromRepo, link.ToRepo, link.LinkType, link.Reason,
		string(endpointsJSON), link.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("saving service link: %w", err)
	}
	return nil
}

// GetLinks returns all service links, optionally filtered by repo name.
func (s *Store) GetLinks(ctx context.Context, repoName string) ([]ServiceLink, error) {
	var rows *sql.Rows
	var err error

	if repoName != "" {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, from_repo, to_repo, link_type, reason, endpoints, created_at
			 FROM service_links WHERE from_repo = ? OR to_repo = ? ORDER BY from_repo, to_repo`,
			repoName, repoName)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, from_repo, to_repo, link_type, reason, endpoints, created_at
			 FROM service_links ORDER BY from_repo, to_repo`)
	}
	if err != nil {
		return nil, fmt.Errorf("querying service links: %w", err)
	}
	defer rows.Close()

	var links []ServiceLink
	for rows.Next() {
		var l ServiceLink
		var endpointsJSON string
		if err := rows.Scan(&l.ID, &l.FromRepo, &l.ToRepo, &l.LinkType, &l.Reason, &endpointsJSON, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning service link: %w", err)
		}
		if err := json.Unmarshal([]byte(endpointsJSON), &l.Endpoints); err != nil {
			l.Endpoints = nil
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

// DeleteLinks removes all service links for a given repo.
func (s *Store) DeleteLinks(ctx context.Context, repoName string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM service_links WHERE from_repo = ? OR to_repo = ?`, repoName, repoName)
	return err
}
