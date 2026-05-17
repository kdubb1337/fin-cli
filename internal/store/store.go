// Package store is the Rung 4 local SQLite mirror of upstream data.
//
// One DB per `~/.fin/cache.db` (override via $FIN_HOME). Read-write opens go
// through Open; the read-only `fin sql` passthrough uses OpenReadOnly.
// Migrations live in migrations/*.sql, are embedded at build time, and apply
// in lexical order keyed off PRAGMA user_version.
package store

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DefaultPath returns ~/.fin/cache.db (or $FIN_HOME/cache.db).
func DefaultPath() (string, error) {
	if h := os.Getenv("FIN_HOME"); h != "" {
		return filepath.Join(h, "cache.db"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".fin", "cache.db"), nil
}

// Store wraps the migrated *sql.DB.
type Store struct {
	DB   *sql.DB
	Path string
}

// Open opens (and migrates) the cache at path. Pass "" to use DefaultPath.
func Open(path string) (*Store, error) {
	if path == "" {
		p, err := DefaultPath()
		if err != nil {
			return nil, err
		}
		path = p
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	// _pragma=foreign_keys(1) enforces ON DELETE CASCADE; busy_timeout reduces
	// "database is locked" errors when sync and a concurrent query race.
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{DB: db, Path: path}, nil
}

// OpenReadOnly opens the cache without applying migrations. The connection
// rejects writes at the SQLite layer; `fin sql` relies on this.
func OpenReadOnly(path string) (*Store, error) {
	if path == "" {
		p, err := DefaultPath()
		if err != nil {
			return nil, err
		}
		path = p
	}
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	dsn := fmt.Sprintf("file:%s?mode=ro&_pragma=query_only(1)&_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{DB: db, Path: path}, nil
}

// Close releases the underlying handle.
func (s *Store) Close() error { return s.DB.Close() }

// migrate applies migrations/*.sql in lexical order, advancing user_version.
func migrate(db *sql.DB) error {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return err
	}
	files := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	var current int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&current); err != nil {
		return err
	}
	for _, name := range files {
		// Migrations are named NNNN_<slug>.sql; we use the NNNN as the version.
		n, err := versionOf(name)
		if err != nil {
			return fmt.Errorf("migration %q: %w", name, err)
		}
		if n <= current {
			continue
		}
		body, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(body)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", n)); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		current = n
	}
	return nil
}

func versionOf(name string) (int, error) {
	prefix := name
	if i := strings.IndexByte(name, '_'); i > 0 {
		prefix = name[:i]
	}
	return strconv.Atoi(prefix)
}

// SchemaVersion returns the current PRAGMA user_version.
func (s *Store) SchemaVersion() (int, error) {
	var v int
	err := s.DB.QueryRow(`PRAGMA user_version`).Scan(&v)
	return v, err
}

// IntegrityCheck runs PRAGMA integrity_check and returns "ok" or the failure.
func (s *Store) IntegrityCheck() (string, error) {
	var got string
	err := s.DB.QueryRow(`PRAGMA integrity_check`).Scan(&got)
	return got, err
}
