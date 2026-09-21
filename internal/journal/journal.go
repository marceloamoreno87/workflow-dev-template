// internal/journal/journal.go
package journal

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

const schema = `
PRAGMA journal_mode=WAL;
PRAGMA busy_timeout=5000;
PRAGMA foreign_keys=ON;
CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS commands (
	command_id TEXT PRIMARY KEY,
	aggregate_id TEXT NOT NULL,
	expected_version INTEGER NOT NULL,
	actor_id TEXT NOT NULL,
	type TEXT NOT NULL,
	reason TEXT NOT NULL,
	received_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS events (
	aggregate_id TEXT NOT NULL,
	version INTEGER NOT NULL,
	command_id TEXT NOT NULL,
	actor_id TEXT NOT NULL,
	type TEXT NOT NULL,
	from_state TEXT NOT NULL,
	to_state TEXT NOT NULL,
	reason TEXT NOT NULL,
	at TEXT NOT NULL,
	PRIMARY KEY (aggregate_id, version)
);
CREATE INDEX IF NOT EXISTS idx_events_command ON events(command_id);
CREATE TABLE IF NOT EXISTS leases (
	resource TEXT PRIMARY KEY,
	holder TEXT NOT NULL,
	expires_at TEXT NOT NULL
);
INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES (1, strftime('%Y-%m-%dT%H:%M:%fZ','now'));
`

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}
