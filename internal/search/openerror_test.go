package search_test

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/simosako/ejquick/internal/builder"
	"github.com/simosako/ejquick/internal/dictionary"
	"github.com/simosako/ejquick/internal/search"
)

// buildTinyDB builds a one-entry dictionary database for open error
// classification tests.
func buildTinyDB(t *testing.T, dt dictionary.Type) string {
	t.Helper()
	dir := t.TempDir()
	input := filepath.Join(dir, "in.TXT")
	if err := os.WriteFile(input, []byte("\x81\xa1word : meaning\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, dt.String()+".sqlite3")
	if _, err := builder.Run(builder.Options{
		Type:     dt,
		Input:    input,
		Output:   output,
		Progress: nil,
	}); err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	return output
}

func categoryOf(t *testing.T, err error) search.OpenCategory {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	category, ok := search.OpenCategoryOf(err)
	if !ok {
		t.Fatalf("error %v is not a search.OpenError", err)
	}
	return category
}

func TestOpenRepositoryClassifiesMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.sqlite3")
	_, err := search.OpenRepository(path, dictionary.Eiwa)
	if got := categoryOf(t, err); got != search.OpenMissing {
		t.Errorf("category = %q, want missing", got)
	}
}

func TestOpenRepositoryClassifiesNotADatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "garbage.sqlite3")
	garbage := make([]byte, 4096)
	for i := range garbage {
		garbage[i] = byte('x')
	}
	if err := os.WriteFile(path, garbage, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := search.OpenRepository(path, dictionary.Eiwa)
	if got := categoryOf(t, err); got != search.OpenCorrupt {
		t.Errorf("category = %q, want corrupt (got error %v)", got, err)
	}
}

func TestOpenRepositoryClassifiesUnreadableDirectory(t *testing.T) {
	dir := t.TempDir()
	_, err := search.OpenRepository(dir, dictionary.Eiwa)
	if got := categoryOf(t, err); got != search.OpenUnreadable {
		t.Errorf("category = %q, want unreadable", got)
	}
}

func TestOpenRepositoryClassifiesUnreadablePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits")
	}
	if os.Getuid() == 0 {
		t.Skip("running as root ignores permission bits")
	}
	path := buildTinyDB(t, dictionary.Eiwa)
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(path, 0o644) })
	_, err := search.OpenRepository(path, dictionary.Eiwa)
	if got := categoryOf(t, err); got != search.OpenUnreadable {
		t.Errorf("category = %q, want unreadable (got error %v)", got, err)
	}
}

func TestOpenRepositoryClassifiesWrongDictionary(t *testing.T) {
	path := buildTinyDB(t, dictionary.Eiwa)
	_, err := search.OpenRepository(path, dictionary.Waei)
	if got := categoryOf(t, err); got != search.OpenWrongDictionary {
		t.Errorf("category = %q, want wrong_dictionary (got error %v)", got, err)
	}
}

func TestOpenRepositoryClassifiesIncompatible(t *testing.T) {
	tests := []struct {
		name string
		sql  string
	}{
		{"old schema version", "UPDATE metadata SET value = '0' WHERE key = 'schema_version'"},
		{"missing table", "DROP TABLE entries_fts"},
		{"missing index", "DROP INDEX idx_entries_headword_norm"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := buildTinyDB(t, dictionary.Eiwa)
			db, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(tt.sql); err != nil {
				db.Close()
				t.Fatal(err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			_, err = search.OpenRepository(path, dictionary.Eiwa)
			if got := categoryOf(t, err); got != search.OpenIncompatible {
				t.Errorf("category = %q, want incompatible (got error %v)", got, err)
			}
		})
	}
}

func TestOpenErrorKeepsCauseAndMessage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.sqlite3")
	_, err := search.OpenRepository(path, dictionary.Eiwa)
	if err == nil {
		t.Fatal("expected an error")
	}
	var oe *search.OpenError
	if !errors.As(err, &oe) {
		t.Fatalf("error %v does not unwrap to *search.OpenError", err)
	}
	if oe.Category() != search.OpenMissing {
		t.Errorf("category = %q, want missing", oe.Category())
	}
	if oe.Error() == "" {
		t.Error("message is empty")
	}
	// The message stays the human-readable CLI text: it names the path.
	if !strings.Contains(oe.Error(), path) {
		t.Errorf("message %q does not mention the path %q", oe.Error(), path)
	}
}

func TestOpenCategoryOfNonOpenError(t *testing.T) {
	if _, ok := search.OpenCategoryOf(errors.New("boom")); ok {
		t.Error("plain error classified as OpenError")
	}
}
