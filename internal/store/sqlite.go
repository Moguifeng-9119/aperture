package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(1)

	s := &Store{db: db}

	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return s, nil
}

func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	migrations := []string{migration001}

	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			return fmt.Errorf("migration: %w", err)
		}
	}
	return nil
}

const migration001 = `
CREATE TABLE IF NOT EXISTS projects (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS api_keys (
    id              TEXT PRIMARY KEY,
    key_hash        TEXT NOT NULL UNIQUE,
    prefix          TEXT NOT NULL,
    name            TEXT NOT NULL,
    project_id      TEXT NOT NULL REFERENCES projects(id),
    rate_limit_rpm  INTEGER DEFAULT 100,
    budget_monthly_usd REAL DEFAULT 0,
    allowed_models  TEXT DEFAULT '[]',
    is_active       BOOLEAN DEFAULT 1,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_used_at    DATETIME
);

CREATE TABLE IF NOT EXISTS routing_decisions (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    request_id      TEXT NOT NULL,
    project_id      TEXT,
    conversation_id TEXT,
    strategy        TEXT NOT NULL,
    complexity      TEXT,
    confidence      REAL,
    model           TEXT NOT NULL,
    provider        TEXT NOT NULL,
    reason          TEXT NOT NULL,
    messages_json   TEXT,
    tokens_in       INTEGER DEFAULT 0,
    tokens_out      INTEGER DEFAULT 0,
    cost_usd        REAL DEFAULT 0,
    saving_usd      REAL DEFAULT 0,
    latency_ms      INTEGER,
    http_status     INTEGER,
    error           TEXT
);

CREATE INDEX IF NOT EXISTS idx_decisions_timestamp ON routing_decisions(timestamp);
CREATE INDEX IF NOT EXISTS idx_decisions_project ON routing_decisions(project_id);
CREATE INDEX IF NOT EXISTS idx_decisions_model ON routing_decisions(model);
`
