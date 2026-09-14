// Package search implements the dictionary search service shared by the
// TUI and the CLI. It owns the Auto search strategy: prefix candidates
// from the B-tree index first, then FTS5 substring candidates to fill the
// remaining slots, with deterministic application-side ranking.
package search

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/simosako/ejquick/internal/dbformat"
	"github.com/simosako/ejquick/internal/dictionary"
	"github.com/simosako/ejquick/internal/normalize"
	"github.com/simosako/ejquick/internal/sqlite"
)

// DefaultMaxResults is the default result limit.
const DefaultMaxResults = 50

// HardMaxResults is the absolute upper bound for any result limit.
const HardMaxResults = 500

// Entry is one search result row.
type Entry struct {
	ID       int64
	Headword string
	Body     string
}

// Repository provides read access to one dictionary database.
type Repository struct {
	db *sql.DB
	dt dictionary.Type
}

// OpenRepository opens the database at path read-only and verifies that it
// was produced for the given dictionary type with a compatible schema.
//
// Failures are returned as *OpenError so frontends can classify the cause
// (GUI design D44/D45) while the human-readable message — and therefore
// the CLI output and exit codes — stay exactly as before.
func OpenRepository(path string, dt dictionary.Type) (*Repository, error) {
	db, err := sqlite.OpenReadOnly(path)
	if err != nil {
		return nil, &OpenError{category: classifyOpenFailure(path, err), err: err}
	}
	if err := verifySchema(db, dt); err != nil {
		db.Close()
		category := OpenUnknown
		var ve *verifyError
		if errors.As(err, &ve) {
			category = ve.category
		}
		return nil, &OpenError{category: category, err: fmt.Errorf("%s (%s): %w", path, dt, err)}
	}
	return &Repository{db: db, dt: dt}, nil
}

// Close releases the database connection.
func (r *Repository) Close() error { return r.db.Close() }

// Dictionary returns the dictionary type of this repository.
func (r *Repository) Dictionary() dictionary.Type { return r.dt }

// verifySchema checks the startup invariants from the design: required
// tables and indexes, metadata versions, and a read-only FTS query. Every
// failure is a *verifyError carrying its OpenCategory; the messages are
// unchanged from the original implementation.
func verifySchema(db *sql.DB, dt dictionary.Type) error {
	for _, table := range []string{"entries", "entries_fts", "metadata"} {
		var name string
		if err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type IN ('table','view') AND name = ?",
			table).Scan(&name); err != nil {
			return classifyVerifyQueryError(err, "missing table %s: %w", table, err)
		}
	}
	var indexName string
	if err := db.QueryRow(
		`SELECT name FROM sqlite_master
		 WHERE type = 'index' AND tbl_name = 'entries' AND name = ?`,
		"idx_entries_headword_norm").Scan(&indexName); err != nil {
		return classifyVerifyQueryError(err, "missing index idx_entries_headword_norm: %w", err)
	}
	var indexColumn string
	if err := db.QueryRow(
		"SELECT name FROM pragma_index_info(?) WHERE seqno = 0",
		indexName).Scan(&indexColumn); err != nil {
		return classifyVerifyQueryError(err, "inspect index idx_entries_headword_norm: %w", err)
	}
	if indexColumn != "headword_norm" {
		return incompatiblef("index idx_entries_headword_norm starts with %q, want headword_norm", indexColumn)
	}
	rows, err := db.Query("SELECT key, value FROM metadata")
	if err != nil {
		return classifyVerifyQueryError(err, "%w", err)
	}
	defer rows.Close()
	got := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return classifyVerifyQueryError(err, "%w", err)
		}
		got[k] = v
	}
	if err := rows.Err(); err != nil {
		return classifyVerifyQueryError(err, "%w", err)
	}
	// The dictionary type is checked first so a swapped database reports
	// wrong_dictionary even when other metadata also mismatches (GUI
	// design D45). A missing dictionary_type row stays incompatible: the
	// file was not produced by a compatible EJQuick build.
	if gotType, ok := got["dictionary_type"]; ok && gotType != dt.String() {
		return &verifyError{
			category: OpenWrongDictionary,
			err:      fmt.Errorf("metadata dictionary_type = %q, want %q (rebuild the database)", gotType, dt.String()),
		}
	}
	want := map[string]string{
		"schema_version":        dbformat.SchemaVersion,
		"normalization_version": normalize.Version,
		"fts_version":           dbformat.FTSVersion,
	}
	for k, w := range want {
		if got[k] != w {
			return incompatiblef("metadata %s = %q, want %q (rebuild the database)", k, got[k], w)
		}
	}
	var ftsCount int64
	if err := db.QueryRow(
		"SELECT count(*) FROM entries_fts WHERE entries_fts MATCH ?",
		`"ejquick-startup-smoke"`).Scan(&ftsCount); err != nil {
		return classifyVerifyQueryError(err, "fts smoke query: %w", err)
	}
	return nil
}

// poolLimitForPrefix returns the prefix pool size for a result limit.
func poolLimitForPrefix(maxResults int) int {
	p := maxResults * 4
	if p < 100 {
		p = 100
	}
	if p > 500 {
		p = 500
	}
	return p
}

// candidateLimitForSubstring returns the FTS candidate pool size for the
// number of remaining result slots.
func candidateLimitForSubstring(remaining int) int {
	c := remaining * 10
	if c < 100 {
		c = 100
	}
	if c > 500 {
		c = 500
	}
	return c
}

// rowIter scans id/headword/headword_norm/body rows.
type candidateRow struct {
	id       int64
	headword string
	norm     string
	body     string
}

// queryPrefixPool fetches exact and prefix matches in dictionary order.
func (r *Repository) queryPrefixPool(ctx context.Context, normQuery string, poolLimit int) ([]candidateRow, error) {
	// U+10FFFF appended so the range covers every string starting with
	// the normalized query prefix in UTF-8 byte order.
	upper := normQuery + "\U0010FFFF"
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, headword, headword_norm, body FROM entries
		 WHERE headword_norm >= ? AND headword_norm < ?
		 ORDER BY headword_norm LIMIT ?`, normQuery, upper, poolLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []candidateRow
	for rows.Next() {
		var c candidateRow
		if err := rows.Scan(&c.id, &c.headword, &c.norm, &c.body); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// querySubstringPool fetches FTS candidates whose match does not start at
// the first character (prefix matches are already covered by the prefix
// pool).
func (r *Repository) querySubstringPool(ctx context.Context, normQuery string, limit int) ([]candidateRow, error) {
	ftsQuery := `"` + strings.ReplaceAll(normQuery, `"`, `""`) + `"`
	rows, err := r.db.QueryContext(ctx,
		`SELECT e.id, e.headword, e.headword_norm, e.body
		 FROM entries_fts f JOIN entries e ON e.id = f.rowid
		 WHERE entries_fts MATCH ? AND instr(e.headword_norm, ?) > 1
		 LIMIT ?`, ftsQuery, normQuery, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []candidateRow
	for rows.Next() {
		var c candidateRow
		if err := rows.Scan(&c.id, &c.headword, &c.norm, &c.body); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
