package db

import (
	"testing"
)

func TestOpenMemory(t *testing.T) {
	d, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error: %v", err)
	}
	defer d.Close()

	// Verify tables exist by inserting into each one.
	tables := []string{
		"audit_entries", "confidence_metadata", "facts",
		"knowledge_questions", "teams", "flows",
		"notifications", "chat_sessions", "import_sources", "api_tokens",
	}

	for _, table := range tables {
		var count int
		err := d.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
		if err != nil {
			t.Errorf("table %s: %v", table, err)
		}
	}
}

func TestMigrateIdempotent(t *testing.T) {
	d, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error: %v", err)
	}
	defer d.Close()

	// Running migrate again should not fail.
	if err := d.migrate(); err != nil {
		t.Fatalf("second migrate() error: %v", err)
	}
}
