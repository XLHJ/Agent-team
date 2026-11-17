// Package store implements Phase-4 persistence: approvals and audit_log
// in SQLite (modernc.org/sqlite, pure Go, CGO-free for GOOS=windows).
package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps the SQLite handle.
type DB struct {
	mu sync.Mutex
	db *sql.DB
}

// AuditRec is one audit_log row.
type AuditRec struct {
	ID        int64
	EventType string
	Payload   string
	CreatedAt time.Time
}

// Open creates or opens the SQLite file and applies the schema.
func Open(path string) (*DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	dsn := "file:" + path
	dsn += "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	sdb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	return &DB{db: sdb}, nil
}

// Close releases the SQLite handle.
func (d *DB) Close() error { return d.db.Close() }

// schema creates the two tables and their indexes (doc section 6.1).
func (d *DB) Schema() error {
	const q = `
CREATE TABLE IF NOT EXISTS approvals (
 approval_id     TEXT PRIMARY KEY,
 task_id         TEXT NOT NULL,
 instance_id     TEXT NOT NULL,
 operation_type  TEXT NOT NULL,
 risk_level      TEXT NOT NULL,
 context_summary TEXT NOT NULL,
 timeout_ms      INTEGER NOT NULL,
 mode            TEXT NOT NULL,
 status          TEXT NOT NULL,
 decision        TEXT,
 modified_params TEXT,
 reason          TEXT,
 params_preview  TEXT,
 created_at      INTEGER NOT NULL,
 resolved_at     INTEGER
);
CREATE INDEX IF NOT EXISTS idx_approvals_status   ON approvals(status);
CREATE INDEX IF NOT EXISTS idx_approvals_task     ON approvals(task_id);
CREATE INDEX IF NOT EXISTS idx_approvals_instance ON approvals(instance_id);
CREATE INDEX IF NOT EXISTS idx_approvals_created  ON approvals(created_at);
CREATE TABLE IF NOT EXISTS audit_log (
 id         INTEGER PRIMARY KEY AUTOINCREMENT,
 event_type TEXT NOT NULL,
 payload    TEXT NOT NULL,
 created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_log(created_at);
CREATE TABLE IF NOT EXISTS usage_logs (
 id            INTEGER PRIMARY KEY AUTOINCREMENT,
 task_id       TEXT NOT NULL,
 instance_id   TEXT NOT NULL DEFAULT '',
 model         TEXT NOT NULL,
 source        TEXT NOT NULL,
 input_tokens  INTEGER NOT NULL,
 output_tokens INTEGER NOT NULL,
 ttft_ms       INTEGER NOT NULL,
 tokens_per_s  REAL NOT NULL,
 duration_ms   INTEGER NOT NULL,
 created_at    INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_usage_logs_created ON usage_logs(created_at);
CREATE TABLE IF NOT EXISTS usage_stats_hourly (
 bucket        INTEGER NOT NULL,
 model         TEXT NOT NULL,
 source        TEXT NOT NULL,
 calls         INTEGER NOT NULL DEFAULT 0,
 input_tokens  INTEGER NOT NULL DEFAULT 0,
 output_tokens INTEGER NOT NULL DEFAULT 0,
 ttft_sum      INTEGER NOT NULL DEFAULT 0,
 duration_sum  INTEGER NOT NULL DEFAULT 0,
 PRIMARY KEY (bucket, model, source)
);
CREATE TABLE IF NOT EXISTS usage_stats_daily (
 bucket        INTEGER NOT NULL,
 model         TEXT NOT NULL,
 source        TEXT NOT NULL,
 calls         INTEGER NOT NULL DEFAULT 0,
 input_tokens  INTEGER NOT NULL DEFAULT 0,
 output_tokens INTEGER NOT NULL DEFAULT 0,
 ttft_sum      INTEGER NOT NULL DEFAULT 0,
 duration_sum  INTEGER NOT NULL DEFAULT 0,
 PRIMARY KEY (bucket, model, source)
);
CREATE TABLE IF NOT EXISTS usage_agg_state (
 key   TEXT PRIMARY KEY,
 value INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS app_settings (
 key   TEXT PRIMARY KEY,
 value TEXT NOT NULL
);
`
	_, err := d.db.Exec(q)
	return err
}

// Audit appends one audit_log row (payload must be valid JSON or empty).
func (d *DB) Audit(eventType, payload string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if payload == "" {
		payload = "{}"
	}
	_, err := d.db.Exec(
		"INSERT INTO audit_log(event_type,payload,created_at) VALUES(?,?,?)",
		eventType, payload, time.Now().UnixMilli(),
	)
	return err
}

// AuditList returns audit rows, newest first.
func (d *DB) AuditList(limit int) ([]AuditRec, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := d.db.Query(
		"SELECT id,event_type,payload,created_at FROM audit_log" +
			" ORDER BY id DESC LIMIT ?")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditRec
	for rows.Next() {
		var a AuditRec
		var ms int64
		if err := rows.Scan(&a.ID, &a.EventType, &a.Payload, &ms); err != nil {
			return nil, err
		}
		a.CreatedAt = time.UnixMilli(ms)
		out = append(out, a)
	}
	return out, rows.Err()
}

// Exec runs a statement on the shared handle.
func (d *DB) Exec(q string, args ...any) (sql.Result, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.Exec(q, args...)
}

// Query runs a query on the shared handle.
func (d *DB) Query(q string, args ...any) (*sql.Rows, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.Query(q, args...)
}

// QueryRow runs a single-row query on the shared handle.
func (d *DB) QueryRow(q string, args ...any) *sql.Row {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.QueryRow(q, args...)
}
