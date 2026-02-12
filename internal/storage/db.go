package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps a SQLite connection with application-level helpers.
type DB struct {
	conn *sql.DB
	mu   sync.RWMutex
}

// New opens (or creates) a SQLite database at the given path.
// If path is empty, it defaults to ~/.graphscope/graphscope.db.
func New(path string) (*DB, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home dir: %w", err)
		}
		dir := filepath.Join(home, ".graphscope")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create config dir: %w", err)
		}
		path = filepath.Join(dir, "graphscope.db")
	}

	conn, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	conn.SetMaxOpenConns(1) // SQLite handles one writer at a time

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

// Conn returns the raw *sql.DB for advanced queries.
func (db *DB) Conn() *sql.DB {
	return db.conn
}

// Close shuts down the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// migrate runs all schema migrations in order.
func (db *DB) migrate() error {
	migrations := []string{
		migrationV1,
	}

	// Create migration tracking table
	if _, err := db.conn.Exec(`CREATE TABLE IF NOT EXISTS migrations (
		version INTEGER PRIMARY KEY,
		applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	for i, m := range migrations {
		version := i + 1
		var exists int
		err := db.conn.QueryRow("SELECT COUNT(*) FROM migrations WHERE version = ?", version).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check migration %d: %w", version, err)
		}
		if exists > 0 {
			continue
		}

		tx, err := db.conn.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", version, err)
		}

		if _, err := tx.Exec(m); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration %d: %w", version, err)
		}

		if _, err := tx.Exec("INSERT INTO migrations (version) VALUES (?)", version); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %d: %w", version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", version, err)
		}
	}

	return nil
}

const migrationV1 = `
CREATE TABLE IF NOT EXISTS schemas (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	source TEXT NOT NULL,
	raw_json TEXT NOT NULL,
	parsed_json TEXT NOT NULL,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS traffic (
	id TEXT PRIMARY KEY,
	timestamp DATETIME NOT NULL,
	method TEXT NOT NULL,
	url TEXT NOT NULL,
	host TEXT NOT NULL,
	headers_json TEXT,
	operation_name TEXT,
	query TEXT,
	variables_json TEXT,
	response_code INTEGER,
	response_body BLOB,
	fingerprint TEXT,
	cluster_id TEXT,
	schema_id TEXT REFERENCES schemas(id),
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_traffic_fingerprint ON traffic(fingerprint);
CREATE INDEX IF NOT EXISTS idx_traffic_host ON traffic(host);
CREATE INDEX IF NOT EXISTS idx_traffic_cluster ON traffic(cluster_id);
CREATE INDEX IF NOT EXISTS idx_traffic_operation ON traffic(operation_name);

CREATE TABLE IF NOT EXISTS analysis_results (
	id TEXT PRIMARY KEY,
	schema_id TEXT REFERENCES schemas(id),
	analysis_type TEXT NOT NULL,
	result_json TEXT NOT NULL,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_analysis_schema ON analysis_results(schema_id);
CREATE INDEX IF NOT EXISTS idx_analysis_type ON analysis_results(analysis_type);

CREATE TABLE IF NOT EXISTS schema_diffs (
	id TEXT PRIMARY KEY,
	schema_a_id TEXT REFERENCES schemas(id),
	schema_b_id TEXT REFERENCES schemas(id),
	diff_json TEXT NOT NULL,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS wordlists (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL UNIQUE,
	words_json TEXT NOT NULL,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
`
