package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

// DB wraps a sql.DB with autodoc-specific helpers.
type DB struct {
	*sql.DB
	mu   sync.RWMutex
	path string
}

// Open creates or opens a SQLite database at the given path.
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("creating database directory: %w", err)
	}

	sqlDB, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	d := &DB{DB: sqlDB, path: path}
	if err := d.migrate(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return d, nil
}

// OpenMemory creates an in-memory SQLite database (useful for testing).
func OpenMemory() (*DB, error) {
	sqlDB, err := sql.Open("sqlite", ":memory:?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("opening in-memory database: %w", err)
	}

	d := &DB{DB: sqlDB, path: ":memory:"}
	if err := d.migrate(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return d, nil
}

// migrate runs all schema migrations.
func (d *DB) migrate() error {
	_, err := d.Exec(schema)
	return err
}

// schema contains the full database schema. New tables are added here.
const schema = `
CREATE TABLE IF NOT EXISTS audit_entries (
    id TEXT PRIMARY KEY,
    timestamp DATETIME NOT NULL DEFAULT (datetime('now')),
    actor_type TEXT NOT NULL CHECK(actor_type IN ('user','system','bot')),
    actor_id TEXT NOT NULL,
    action TEXT NOT NULL,
    scope TEXT NOT NULL,
    scope_id TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    detail TEXT NOT NULL DEFAULT '',
    source_fact TEXT,
    affected_entities TEXT NOT NULL DEFAULT '[]',
    conversation_id TEXT,
    previous_value TEXT,
    new_value TEXT
);

CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_entries(timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_actor ON audit_entries(actor_id);
CREATE INDEX IF NOT EXISTS idx_audit_scope ON audit_entries(scope, scope_id);
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_entries(action);

CREATE TABLE IF NOT EXISTS confidence_metadata (
    id TEXT PRIMARY KEY,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    confidence TEXT NOT NULL CHECK(confidence IN ('auto_detected','confirmed','human_provided','external_import','ai_inferred')),
    source TEXT NOT NULL,
    source_detail TEXT,
    attributed_to TEXT,
    attributed_at DATETIME,
    last_verified DATETIME NOT NULL DEFAULT (datetime('now')),
    potentially_stale INTEGER NOT NULL DEFAULT 0,
    stale_reason TEXT,
    UNIQUE(entity_type, entity_id)
);

CREATE INDEX IF NOT EXISTS idx_confidence_entity ON confidence_metadata(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_confidence_stale ON confidence_metadata(potentially_stale);

CREATE TABLE IF NOT EXISTS facts (
    id TEXT PRIMARY KEY,
    repo_id TEXT NOT NULL DEFAULT '',
    scope TEXT NOT NULL,
    scope_id TEXT NOT NULL DEFAULT '',
    key TEXT NOT NULL,
    value TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT 'user',
    provided_by TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
    version INTEGER NOT NULL DEFAULT 1,
    superseded_by TEXT,
    UNIQUE(repo_id, scope, scope_id, key, version)
);

CREATE INDEX IF NOT EXISTS idx_facts_scope ON facts(scope, scope_id);
CREATE INDEX IF NOT EXISTS idx_facts_repo ON facts(repo_id);
CREATE INDEX IF NOT EXISTS idx_facts_key ON facts(key);

CREATE TABLE IF NOT EXISTS knowledge_questions (
    id TEXT PRIMARY KEY,
    repo_id TEXT NOT NULL DEFAULT '',
    question TEXT NOT NULL,
    category TEXT NOT NULL DEFAULT 'general',
    priority INTEGER NOT NULL DEFAULT 50,
    status TEXT NOT NULL DEFAULT 'open' CHECK(status IN ('open','answered','verified','retired')),
    source TEXT NOT NULL DEFAULT 'system',
    source_detail TEXT,
    answer TEXT,
    answered_by TEXT,
    answered_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_questions_status ON knowledge_questions(status);
CREATE INDEX IF NOT EXISTS idx_questions_priority ON knowledge_questions(priority DESC);
CREATE INDEX IF NOT EXISTS idx_questions_repo ON knowledge_questions(repo_id);

CREATE TABLE IF NOT EXISTS teams (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT 'manual',
    source_id TEXT,
    slack_channel TEXT,
    email TEXT,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS team_members (
    team_id TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'member',
    PRIMARY KEY(team_id, user_id)
);

CREATE TABLE IF NOT EXISTS service_ownership (
    team_id TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    repo_id TEXT NOT NULL,
    confidence TEXT NOT NULL DEFAULT 'auto_detected',
    source TEXT NOT NULL DEFAULT 'codeowners',
    PRIMARY KEY(team_id, repo_id)
);

CREATE TABLE IF NOT EXISTS flows (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    narrative TEXT NOT NULL DEFAULT '',
    mermaid_diagram TEXT NOT NULL DEFAULT '',
    services TEXT NOT NULL DEFAULT '[]',
    entry_point TEXT NOT NULL DEFAULT '',
    exit_point TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_flows_name ON flows(name);

CREATE TABLE IF NOT EXISTS notifications (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    severity TEXT NOT NULL DEFAULT 'info' CHECK(severity IN ('info','warning','critical')),
    title TEXT NOT NULL,
    message TEXT NOT NULL DEFAULT '',
    affected_services TEXT NOT NULL DEFAULT '[]',
    affected_teams TEXT NOT NULL DEFAULT '[]',
    delivered INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_notifications_delivered ON notifications(delivered);
CREATE INDEX IF NOT EXISTS idx_notifications_created ON notifications(created_at);

CREATE TABLE IF NOT EXISTS notification_preferences (
    team_id TEXT NOT NULL,
    channel TEXT NOT NULL DEFAULT 'dashboard',
    severity_filter TEXT NOT NULL DEFAULT 'info',
    digest_frequency TEXT NOT NULL DEFAULT 'realtime' CHECK(digest_frequency IN ('realtime','daily','weekly')),
    webhook_url TEXT,
    PRIMARY KEY(team_id, channel)
);

CREATE TABLE IF NOT EXISTS chat_sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL DEFAULT 'anonymous',
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS chat_messages (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK(role IN ('user','assistant','system')),
    content TEXT NOT NULL,
    metadata TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_chat_messages_session ON chat_messages(session_id, created_at);

CREATE TABLE IF NOT EXISTS import_sources (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL CHECK(type IN ('confluence','readme','adr','openapi','asyncapi')),
    name TEXT NOT NULL,
    config TEXT NOT NULL DEFAULT '{}',
    last_imported DATETIME,
    status TEXT NOT NULL DEFAULT 'configured' CHECK(status IN ('configured','importing','completed','failed')),
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS api_tokens (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    scope TEXT NOT NULL DEFAULT 'read' CHECK(scope IN ('read','readwrite','admin')),
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    expires_at DATETIME,
    last_used DATETIME
);
`
