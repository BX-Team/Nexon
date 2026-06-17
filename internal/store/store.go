// Package store owns the SQLite database, migrations, and all queries.
package store

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, no CGO
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Store wraps the SQLite connection.
type Store struct {
	db *sql.DB
}

// Open opens or creates the SQLite database at path, enables WAL, and applies migrations.
func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create data dir: %w", err)
		}
	}
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// Single writer: WAL tolerates many readers but we keep writes serialized.
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// DB exposes the underlying handle for advanced/ad-hoc use.
func (s *Store) DB() *sql.DB { return s.db }

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY)`); err != nil {
		return err
	}
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		var exists string
		err := s.db.QueryRow(`SELECT name FROM schema_migrations WHERE name = ?`, name).Scan(&exists)
		if err == nil {
			continue // already applied
		}
		if err != sql.ErrNoRows {
			return err
		}
		body, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		if _, err := s.db.Exec(string(body)); err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
		if _, err := s.db.Exec(`INSERT INTO schema_migrations(name) VALUES (?)`, name); err != nil {
			return err
		}
	}
	return nil
}
