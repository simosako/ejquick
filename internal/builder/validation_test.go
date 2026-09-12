package builder

import (
	"testing"

	"github.com/simosako/ejquick/internal/sqlite"
)

func TestValidateFTSSmokeUsesMatchableEntry(t *testing.T) {
	db, err := sqlite.OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(entriesSchema + ftsSchema); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO entries(id, headword, headword_norm, body) VALUES
		(1, 'a', 'a', 'short'), (2, 'alpha', 'alpha', 'long')`); err != nil {
		t.Fatal(err)
	}

	if err := validateFTSSmoke(db); err == nil {
		t.Fatal("validation accepted an empty FTS index because the first entry was short")
	}
	if _, err := db.Exec("INSERT INTO entries_fts(entries_fts) VALUES('rebuild')"); err != nil {
		t.Fatal(err)
	}
	if err := validateFTSSmoke(db); err != nil {
		t.Fatalf("validation rejected rebuilt FTS index: %v", err)
	}
}

func TestValidateFTSSmokeWithOnlyShortEntries(t *testing.T) {
	db, err := sqlite.OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(entriesSchema + ftsSchema); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO entries(id, headword, headword_norm, body) VALUES
		(1, 'a', 'a', 'first'), (2, 'bb', 'bb', 'second')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO entries_fts(entries_fts) VALUES('rebuild')"); err != nil {
		t.Fatal(err)
	}
	if err := validateFTSSmoke(db); err != nil {
		t.Fatalf("validation rejected a valid short-entry FTS table: %v", err)
	}
}
