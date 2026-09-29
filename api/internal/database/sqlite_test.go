package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenAndMigrate(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sqlite_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "sub", "test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Verify tables exist
	var tableCount int
	err = db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name IN ('leads', 'email_events', 'webhook_receipts')`).Scan(&tableCount)
	if err != nil {
		t.Fatalf("query tables failed: %v", err)
	}
	if tableCount != 3 {
		t.Errorf("expected 3 tables, got %d", tableCount)
	}

	// Verify idempotency of migration: opening again or migrating again should not fail
	if err := migrate(db); err != nil {
		t.Errorf("second migrate failed: %v", err)
	}

	// Verify addColumnIfMissing on an existing column
	if err := addColumnIfMissing(db, "leads", "category TEXT"); err != nil {
		t.Errorf("addColumnIfMissing on existing column failed: %v", err)
	}

	// Verify addColumnIfMissing on a new column
	if err := addColumnIfMissing(db, "leads", "test_new_col TEXT DEFAULT ''"); err != nil {
		t.Errorf("addColumnIfMissing on new column failed: %v", err)
	}

	// Verify column was added
	var colFound bool
	rows, err := db.Query(`PRAGMA table_info(leads)`)
	if err != nil {
		t.Fatalf("table_info failed: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, primaryKey int
		var col, dataType string
		var dflt any
		if err := rows.Scan(&cid, &col, &dataType, &notNull, &dflt, &primaryKey); err != nil {
			t.Fatalf("scan pragma failed: %v", err)
		}
		if col == "test_new_col" {
			colFound = true
			break
		}
	}
	if !colFound {
		t.Errorf("expected test_new_col to be added to leads")
	}
}
