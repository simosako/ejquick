// Package sqlite provides helpers to open the dictionary SQLite databases
// with the modernc.org/sqlite driver (pure Go, no CGo).
package sqlite

import (
	"database/sql"
	"fmt"
	"net/url"

	_ "modernc.org/sqlite" // register the "sqlite" database/sql driver
)

// DriverName is the database/sql driver name registered by modernc.org/sqlite.
const DriverName = "sqlite"

// OpenReadOnly opens the database file in read-only mode as a plain file path.
// The database must already exist; this function never creates a file.
func OpenReadOnly(path string) (*sql.DB, error) {
	dsn := "file:" + url.PathEscape(path) + "?mode=ro"
	db, err := sql.Open(DriverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s read-only: %w", path, err)
	}
	// Connection settings tuned for a local read-only workload.
	// The pool stays small because a single interactive search issues
	// one query at a time.
	db.SetMaxOpenConns(2)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping %s: %w", path, err)
	}
	return db, nil
}

// OpenReadWrite opens the database file for reading and writing.
// Used by the builder for the temporary build database.
func OpenReadWrite(path string) (*sql.DB, error) {
	db, err := sql.Open(DriverName, path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return db, nil
}

// OpenInMemory opens a private in-memory database. Used by tests and smoke
// tests only.
func OpenInMemory() (*sql.DB, error) {
	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		return nil, fmt.Errorf("open in-memory database: %w", err)
	}
	return db, nil
}
