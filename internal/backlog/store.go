package backlog

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/ziadkadry99/auto-doc/internal/db"
)

// Store manages persistence of knowledge backlog questions.
type Store struct {
	db *db.DB
}

// NewStore creates a new backlog store.
func NewStore(database *db.DB) *Store {
	return &Store{db: database}
}

// Create adds a new question to the backlog.
func (s *Store) Create(ctx context.Context, q Question) (*Question, error) {
	if q.ID == "" {
		q.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	q.CreatedAt = now
	q.UpdatedAt = now
	if q.Status == "" {
		q.Status = StatusOpen
	}
	if q.Category == "" {
		q.Category = CategoryGeneral
	}
	if q.Source == "" {
		q.Source = "system"
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO knowledge_questions (id, repo_id, question, category, priority, status, source, source_detail, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		q.ID, q.RepoID, q.Question, q.Category, q.Priority, q.Status, q.Source, q.SourceDetail, q.CreatedAt, q.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting question: %w", err)
	}
	return &q, nil
}

// GetByID retrieves a question by its ID.
func (s *Store) GetByID(ctx context.Context, id string) (*Question, error) {
	var q Question
	var answer, answeredBy, sourceDetail sql.NullString
	var answeredAt sql.NullTime

	err := s.db.QueryRowContext(ctx,
		`SELECT id, repo_id, question, category, priority, status, source, source_detail, answer, answered_by, answered_at, created_at, updated_at
		 FROM knowledge_questions WHERE id = ?`, id,
	).Scan(&q.ID, &q.RepoID, &q.Question, &q.Category, &q.Priority, &q.Status, &q.Source, &sourceDetail, &answer, &answeredBy, &answeredAt, &q.CreatedAt, &q.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting question: %w", err)
	}
	q.Answer = answer.String
	q.AnsweredBy = answeredBy.String
	q.SourceDetail = sourceDetail.String
	if answeredAt.Valid {
		q.AnsweredAt = &answeredAt.Time
	}
	return &q, nil
}

// List returns questions matching the filter.
func (s *Store) List(ctx context.Context, filter ListFilter) ([]Question, error) {
	query := `SELECT id, repo_id, question, category, priority, status, source, source_detail, answer, answered_by, answered_at, created_at, updated_at
		 FROM knowledge_questions WHERE 1=1`
	args := []interface{}{}

	if filter.RepoID != "" {
		query += " AND repo_id = ?"
		args = append(args, filter.RepoID)
	}
	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, filter.Status)
	}
	if filter.Category != "" {
		query += " AND category = ?"
		args = append(args, filter.Category)
	}
	if filter.MinPriority > 0 {
		query += " AND priority >= ?"
		args = append(args, filter.MinPriority)
	}

	query += " ORDER BY priority DESC, created_at ASC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing questions: %w", err)
	}
	defer rows.Close()

	var questions []Question
	for rows.Next() {
		var q Question
		var answer, answeredBy, sourceDetail sql.NullString
		var answeredAt sql.NullTime
		if err := rows.Scan(&q.ID, &q.RepoID, &q.Question, &q.Category, &q.Priority, &q.Status, &q.Source, &sourceDetail, &answer, &answeredBy, &answeredAt, &q.CreatedAt, &q.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning question: %w", err)
		}
		q.Answer = answer.String
		q.AnsweredBy = answeredBy.String
		q.SourceDetail = sourceDetail.String
		if answeredAt.Valid {
			q.AnsweredAt = &answeredAt.Time
		}
		questions = append(questions, q)
	}
	return questions, rows.Err()
}

// Answer records an answer for a question.
func (s *Store) Answer(ctx context.Context, id, answer, answeredBy string) error {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx,
		`UPDATE knowledge_questions SET answer = ?, answered_by = ?, answered_at = ?, status = ?, updated_at = ?
		 WHERE id = ?`,
		answer, answeredBy, now, StatusAnswered, now, id,
	)
	if err != nil {
		return fmt.Errorf("answering question: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("question not found: %s", id)
	}
	return nil
}

// UpdateStatus changes the status of a question.
func (s *Store) UpdateStatus(ctx context.Context, id string, status Status) error {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx,
		`UPDATE knowledge_questions SET status = ?, updated_at = ? WHERE id = ?`,
		status, now, id,
	)
	if err != nil {
		return fmt.Errorf("updating status: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("question not found: %s", id)
	}
	return nil
}

// GetOpenCount returns the number of open questions.
func (s *Store) GetOpenCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM knowledge_questions WHERE status = ?`, StatusOpen,
	).Scan(&count)
	return count, err
}

// GetTopPriority returns the top N highest-priority open questions.
func (s *Store) GetTopPriority(ctx context.Context, n int) ([]Question, error) {
	return s.List(ctx, ListFilter{Status: StatusOpen, Limit: n})
}
