package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	// busy_timeout lets the background campaign sender and HTTP requests share the file safely.
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS leads (
			id INTEGER PRIMARY KEY, name TEXT NOT NULL DEFAULT '', email TEXT NOT NULL DEFAULT '',
			phone TEXT NOT NULL DEFAULT '', category TEXT NOT NULL DEFAULT '', subcategory TEXT NOT NULL DEFAULT '',
			mail_sent INTEGER DEFAULT 0, is_invalid INTEGER DEFAULT 0, is_opened INTEGER DEFAULT 0,
			any_followup INTEGER DEFAULT 0, followup_count INTEGER DEFAULT 0, followup_limit INTEGER DEFAULT 3,
			last_followup_at TEXT, next_followup_at TEXT, followup_paused INTEGER DEFAULT 0, replied INTEGER DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE UNIQUE INDEX IF NOT EXISTS unique_lead_email ON leads(email) WHERE email != '';
		CREATE UNIQUE INDEX IF NOT EXISTS unique_lead_phone ON leads(phone) WHERE phone != '';
		CREATE TABLE IF NOT EXISTS email_events (
			id INTEGER PRIMARY KEY, lead_id INTEGER NOT NULL, kind TEXT NOT NULL,
			subject TEXT NOT NULL DEFAULT '', body TEXT NOT NULL DEFAULT '', content TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(lead_id) REFERENCES leads(id)
		);
		CREATE INDEX IF NOT EXISTS email_events_lead ON email_events(lead_id);
		CREATE TABLE IF NOT EXISTS webhook_receipts (
			id TEXT PRIMARY KEY, event_type TEXT NOT NULL, received_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS campaigns (
			id INTEGER PRIMARY KEY, mode TEXT NOT NULL, subject TEXT NOT NULL DEFAULT '', body TEXT NOT NULL DEFAULT '',
			segment TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'queued',
			total INTEGER NOT NULL DEFAULT 0, sent INTEGER NOT NULL DEFAULT 0, failed INTEGER NOT NULL DEFAULT 0,
			last_error TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, finished_at TEXT NOT NULL DEFAULT ''
		);
	`)
	if err != nil {
		return err
	}
	leadColumns := []string{
		"followup_limit INTEGER DEFAULT 3", "last_followup_at TEXT", "next_followup_at TEXT", "followup_paused INTEGER DEFAULT 0",
		"organization TEXT NOT NULL DEFAULT ''", "designation TEXT NOT NULL DEFAULT ''", "address TEXT NOT NULL DEFAULT ''",
		"segment TEXT NOT NULL DEFAULT ''", "unsubscribed INTEGER NOT NULL DEFAULT 0", "unsub_token TEXT NOT NULL DEFAULT ''",
	}
	for _, column := range leadColumns {
		if err := addColumnIfMissing(db, "leads", column); err != nil {
			return err
		}
	}
	for _, column := range []string{"from_addr TEXT NOT NULL DEFAULT ''", "to_addr TEXT NOT NULL DEFAULT ''", "resend_id TEXT NOT NULL DEFAULT ''", "campaign_id INTEGER NOT NULL DEFAULT 0"} {
		if err := addColumnIfMissing(db, "email_events", column); err != nil {
			return err
		}
	}
	// Category was replaced by segment; carry existing values over once.
	if _, err := db.Exec(`UPDATE leads SET segment=category WHERE segment='' AND category!=''`); err != nil {
		return err
	}
	// Campaigns that were running when the API stopped cannot resume by themselves.
	_, err = db.Exec(`UPDATE campaigns SET status='interrupted', finished_at=CURRENT_TIMESTAMP WHERE status IN ('queued','running')`)
	return err
}

func addColumnIfMissing(db *sql.DB, table, definition string) error {
	name := strings.Fields(definition)[0]
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	found := false
	for rows.Next() {
		var cid, notNull, primaryKey int
		var column, dataType string
		var defaultValue any
		if err := rows.Scan(&cid, &column, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			rows.Close()
			return err
		}
		if column == name {
			found = true
		}
	}
	rows.Close()
	if found {
		return nil
	}
	_, err = db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + definition)
	return err
}
