package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/simosako/ejquick/internal/sqlite"
)

// entriesSchema mirrors the production schema from the design document:
// a plain entries table plus an FTS5 external content table over
// headword_norm with the trigram tokenizer.
const entriesSchema = `
CREATE TABLE entries (
    id            INTEGER PRIMARY KEY,
    headword      TEXT NOT NULL,
    headword_norm TEXT NOT NULL,
    body          TEXT NOT NULL
);
CREATE VIRTUAL TABLE entries_fts
USING fts5(
    headword_norm,
    content='entries',
    content_rowid='id',
    tokenize='trigram case_sensitive 1 remove_diacritics 0'
);
`

func createFixtureDB(t *testing.T) {
	t.Helper()
	db, err := sqlite.OpenInMemory()
	if err != nil {
		t.Fatalf("open in-memory: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(entriesSchema); err != nil {
		t.Fatalf("create schema (FTS5/trigram may be unavailable): %v", err)
	}

	rows := []struct {
		id   int64
		hw   string
		norm string
		body string
	}{
		{1, "English", "english", "the English language"},
		{2, "English breakfast", "english breakfast", "a cooked breakfast"},
		{3, "take care", "take care", "be careful"},
		{4, "care", "care", "attention"},
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare("INSERT INTO entries(id, headword, headword_norm, body) VALUES (?, ?, ?, ?)")
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.Exec(r.id, r.hw, r.norm, r.body); err != nil {
			t.Fatalf("insert %d: %v", r.id, err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if _, err := db.Exec("INSERT INTO entries_fts(entries_fts) VALUES('rebuild')"); err != nil {
		t.Fatalf("fts rebuild: %v", err)
	}
	if _, err := db.Exec("INSERT INTO entries_fts(entries_fts) VALUES('integrity-check')"); err != nil {
		t.Fatalf("fts integrity-check: %v", err)
	}
}

// TestSmokeFTS5Trigram verifies that the pure-Go driver ships with FTS5
// enabled and that the trigram tokenizer works with the configured options
// on an external content table.
func TestSmokeFTS5Trigram(t *testing.T) {
	db, err := sqlite.OpenInMemory()
	if err != nil {
		t.Fatalf("open in-memory: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(entriesSchema); err != nil {
		t.Fatalf("create schema (FTS5/trigram may be unavailable): %v", err)
	}

	rows := []struct {
		id   int64
		hw   string
		norm string
		body string
	}{
		{1, "English", "english", "the English language"},
		{2, "English breakfast", "english breakfast", "a cooked breakfast"},
		{3, "take care", "take care", "be careful"},
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare("INSERT INTO entries(id, headword, headword_norm, body) VALUES (?, ?, ?, ?)")
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.Exec(r.id, r.hw, r.norm, r.body); err != nil {
			t.Fatalf("insert %d: %v", r.id, err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if _, err := db.Exec("INSERT INTO entries_fts(entries_fts) VALUES('rebuild')"); err != nil {
		t.Fatalf("fts rebuild: %v", err)
	}

	// A trigram substring query: "ngli" is contained in "english".
	// The query is a quoted phrase so it is treated as a literal.
	ftsQuery := `"ngli"`
	var count int
	err = db.QueryRow(
		`SELECT count(*) FROM entries_fts f JOIN entries e ON e.id = f.rowid
		 WHERE entries_fts MATCH ? AND instr(e.headword_norm, ?) > 0`,
		ftsQuery, "ngli").Scan(&count)
	if err != nil {
		t.Fatalf("fts match query: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 substring matches for %q, got %d", "ngli", count)
	}

	// Integrity check must pass after rebuild.
	if _, err := db.Exec("INSERT INTO entries_fts(entries_fts) VALUES('integrity-check')"); err != nil {
		t.Fatalf("fts integrity-check: %v", err)
	}
}

// TestSmokeReadOnlyOpen verifies that mode=ro opens an existing database
// read-only, rejects writes, and fails for a missing file.
func TestSmokeReadOnlyOpen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dict.sqlite3")

	db, err := sqlite.OpenReadWrite(path)
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	if _, err := db.Exec("CREATE TABLE t(v)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	ro, err := sqlite.OpenReadOnly(path)
	if err != nil {
		t.Fatalf("open read-only: %v", err)
	}
	defer ro.Close()

	var v int
	if err := ro.QueryRow("SELECT count(*) FROM t").Scan(&v); err != nil {
		t.Fatalf("select on read-only db: %v", err)
	}
	if _, err := ro.Exec("INSERT INTO t VALUES (1)"); err == nil {
		t.Fatal("write on read-only db unexpectedly succeeded")
	}

	// A missing file must not be created by OpenReadOnly.
	if _, err := sqlite.OpenReadOnly(filepath.Join(dir, "missing.sqlite3")); err == nil {
		t.Fatal("opening a missing file read-only unexpectedly succeeded")
	}
}

// TestSmokeQueryContextCancel verifies that a running query can be
// interrupted through context cancellation.
func TestSmokeQueryContextCancel(t *testing.T) {
	db, err := sqlite.OpenInMemory()
	if err != nil {
		t.Fatalf("open in-memory: %v", err)
	}
	defer db.Close()

	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatalf("get connection: %v", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(context.Background(), "CREATE TABLE t(v TEXT)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := conn.ExecContext(context.Background(), "INSERT INTO t VALUES ('still usable')"); err != nil {
		t.Fatalf("insert sentinel: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel while the query is running: use a slow recursive CTE.
	timer := time.AfterFunc(50*time.Millisecond, cancel)
	defer timer.Stop()
	start := time.Now()
	rows, err := conn.QueryContext(ctx,
		`WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM c) SELECT count(*) FROM c`)
	if err == nil {
		for rows.Next() {
		}
		err = rows.Err()
		rows.Close()
	}
	elapsed := time.Since(start)
	if err == nil {
		cancel()
		t.Fatalf("query unexpectedly succeeded without cancellation")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("query error = %v, want context.Canceled", err)
	}
	// The query must return quickly instead of counting billions of rows.
	if elapsed > 5*time.Second {
		t.Errorf("cancellation took too long: %v", elapsed)
	}
	cancel()

	// Verify the same connection can still query its existing table.
	var got string
	if err := conn.QueryRowContext(context.Background(), "SELECT v FROM t").Scan(&got); err != nil {
		t.Fatalf("query existing table after cancel: %v", err)
	}
	if got != "still usable" {
		t.Errorf("sentinel after cancel = %q", got)
	}
}

// TestSmokeParameterBinding verifies that values containing FTS5 operators
// are bound as parameters and never interpreted as query syntax.
func TestSmokeParameterBinding(t *testing.T) {
	db, err := sqlite.OpenInMemory()
	if err != nil {
		t.Fatalf("open in-memory: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(entriesSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO entries(id, headword, headword_norm, body)
		VALUES (1, 'AND OR', 'and or', 'operators in a headword')`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if _, err := db.Exec("INSERT INTO entries_fts(entries_fts) VALUES('rebuild')"); err != nil {
		t.Fatalf("fts rebuild: %v", err)
	}

	// Searching for the literal "and or" must not raise a syntax error.
	ftsQuery := `"and or"`
	var count int
	err = db.QueryRow(
		`SELECT count(*) FROM entries_fts f JOIN entries e ON e.id = f.rowid
		 WHERE entries_fts MATCH ?`, ftsQuery).Scan(&count)
	if err != nil {
		t.Fatalf("fts match with operator-like literal: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 match, got %d", count)
	}
}

var _ = sql.DB{} // keep database/sql imported for documentation clarity
