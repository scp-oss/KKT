// Package db wraps the SQLite storage layer for the KKT monitor.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS kkt (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	serial_number TEXT NOT NULL UNIQUE,
	organization  TEXT NOT NULL DEFAULT '',
	reg_number    TEXT NOT NULL DEFAULT '',
	fn_number     TEXT NOT NULL DEFAULT '',
	address       TEXT NOT NULL DEFAULT '',
	model         TEXT NOT NULL DEFAULT '',
	ofd_end_date  TEXT NOT NULL DEFAULT '',
	fn_end_date   TEXT NOT NULL DEFAULT '',
	created_at    TEXT NOT NULL,
	updated_at    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS recipients (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	chat_id      TEXT NOT NULL,
	name         TEXT NOT NULL DEFAULT '',
	organization TEXT NOT NULL DEFAULT '',
	enabled      INTEGER NOT NULL DEFAULT 1,
	created_at   TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
	token      TEXT PRIMARY KEY,
	role       TEXT NOT NULL DEFAULT 'admin',
	expires_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS notification_log (
	id             INTEGER PRIMARY KEY AUTOINCREMENT,
	kkt_id         INTEGER NOT NULL,
	field          TEXT NOT NULL,
	threshold_days INTEGER NOT NULL,
	end_date       TEXT NOT NULL,
	sent_at        TEXT NOT NULL,
	UNIQUE(kkt_id, field, threshold_days, end_date)
);
`

// Open opens (creating if needed) the SQLite database at path and applies the schema.
func Open(path string) (*DB, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}
	sqlDB, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite only supports a single writer; keep this simple and serialized.
	sqlDB.SetMaxOpenConns(1)

	if _, err := sqlDB.Exec(schema); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	if err := migrate(sqlDB); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}
	return &DB{sqlDB}, nil
}

// migrate adds columns introduced after the initial schema to databases
// created by older versions of this program.
func migrate(sqlDB *sql.DB) error {
	if err := addColumnIfMissing(sqlDB, "kkt", "organization", `ALTER TABLE kkt ADD COLUMN organization TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	if err := addColumnIfMissing(sqlDB, "sessions", "role", `ALTER TABLE sessions ADD COLUMN role TEXT NOT NULL DEFAULT 'admin'`); err != nil {
		return err
	}
	if err := addColumnIfMissing(sqlDB, "recipients", "organization", `ALTER TABLE recipients ADD COLUMN organization TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	return nil
}

func addColumnIfMissing(sqlDB *sql.DB, table, column, alterSQL string) error {
	rows, err := sqlDB.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	found := false
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			rows.Close()
			return err
		}
		if name == column {
			found = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()

	if !found {
		if _, err := sqlDB.Exec(alterSQL); err != nil {
			return err
		}
	}
	return nil
}
