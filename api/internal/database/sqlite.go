package database

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
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
			any_followup INTEGER DEFAULT 0, followup_count INTEGER DEFAULT 0, replied INTEGER DEFAULT 0,
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
	`)
	return err
}
