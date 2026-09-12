package search_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/simosako/ejquick/internal/builder"
	"github.com/simosako/ejquick/internal/dictionary"
	"github.com/simosako/ejquick/internal/search"
)

// buildFixtureDB builds a small dictionary database via the real builder.
// fixtureLines are CP932-encoded artificial entries (ASCII only in tests
// keeps the helper trivial).
func buildFixtureDB(t *testing.T, dt dictionary.Type, lines []string) string {
	t.Helper()
	dir := t.TempDir()
	input := filepath.Join(dir, "in.TXT")
	body := ""
	for _, l := range lines {
		body += l + "\r\n"
	}
	if err := os.WriteFile(input, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, dt.String()+".sqlite3")
	if _, err := builder.Run(builder.Options{
		Type:     dt,
		Input:    input,
		Output:   output,
		Progress: os.Stderr,
	}); err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	return output
}

const fixtureDictionary = `
care : attention
take care : be careful
careful : cautious
take care of : look after
scare : frighten
careless : incautious
daycare : childcare
medical care : treatment
care package : aid parcel
caretaker : custodian
caress : touch gently
career : profession
organic : natural
card : rectangular piece
scar : mark
scared : afraid
carefully : with care
aftercare : follow-up support
undercare : not a real word but unique
`

func newService(t *testing.T, path string, maxResults int) *search.Service {
	t.Helper()
	repo, err := search.OpenRepository(path, dictionary.Eiji)
	if err != nil {
		t.Fatalf("open repository: %v", err)
	}
	t.Cleanup(func() { repo.Close() })
	svc, err := search.NewService(repo, maxResults)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	return svc
}

func headwords(entries []search.Entry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Headword
	}
	return out
}

func TestSearchExactComesFirstThenShorter(t *testing.T) {
	db := buildFixtureDB(t, dictionary.Eiji, fixtureLines(t))
	svc := newService(t, db, 50)

	entries, err := svc.Search(context.Background(), "care")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	got := make([]string, len(entries))
	for i, entry := range entries {
		got[i] = fmt.Sprintf("%d:%s", entry.ID, entry.Headword)
	}
	want := []string{
		"1:care",
		"12:career",
		"11:caress",
		"3:careful",
		"6:careless",
		"17:carefully",
		"10:caretaker",
		"9:care package",
		"5:scare",
		"16:scared",
		"7:daycare",
		"18:aftercare",
		"2:take care",
		"19:undercare",
		"4:take care of",
		"8:medical care",
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("ordered results = %v, want %v", got, want)
	}
}

func fixtureLines(t *testing.T) []string {
	t.Helper()
	var lines []string
	for _, l := range splitLines(fixtureDictionary) {
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, trimCR(s[start:i]))
			start = i + 1
		}
	}
	out = append(out, trimCR(s[start:]))
	return out
}

func trimCR(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\r' || s[len(s)-1] == ' ') {
		s = s[:len(s)-1]
	}
	return s
}

func TestSearchTwoRunesPrefixOnly(t *testing.T) {
	db := buildFixtureDB(t, dictionary.Eiji, fixtureLines(t))
	svc := newService(t, db, 50)

	entries, err := svc.Search(context.Background(), "ca")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected prefix results")
	}
	for _, e := range entries {
		if !hasPrefixNorm(t, db, e.ID, "ca") {
			t.Errorf("two-rune query returned non-prefix entry %q", e.Headword)
		}
	}
	// "scare" contains "ca" but does not start with it and must be
	// absent from a two-rune result.
	for _, e := range entries {
		if e.Headword == "scare" {
			t.Error("substring result leaked into 2-rune query")
		}
	}
}

func TestSearchThreeRunesAddsSubstring(t *testing.T) {
	db := buildFixtureDB(t, dictionary.Eiji, fixtureLines(t))
	svc := newService(t, db, 50)

	entries, err := svc.Search(context.Background(), "are")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	got := headwords(entries)
	if len(got) < 3 {
		t.Fatalf("too few results: %v", got)
	}
	// No headword starts with "are", so every result is a substring match
	// ranked by match position then length: "care" (pos 1) precedes
	// "scare" (pos 1, longer) precedes "career" (pos 1, longer still) ...
	wantFirst := "care"
	if got[0] != wantFirst {
		t.Errorf("first = %q, want %q (earliest, shortest match)", got[0], wantFirst)
	}
	if !containsStr(got, "scare") {
		t.Errorf("substring candidates missing: %v", got)
	}
}

func TestSearchRespectsMaxResults(t *testing.T) {
	db := buildFixtureDB(t, dictionary.Eiji, fixtureLines(t))
	svc := newService(t, db, 3)
	entries, err := svc.Search(context.Background(), "care")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("results = %d, want 3", len(entries))
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	db := buildFixtureDB(t, dictionary.Eiji, fixtureLines(t))
	svc := newService(t, db, 50)
	for _, q := range []string{"", "   ", "\t"} {
		entries, err := svc.Search(context.Background(), q)
		if err != nil {
			t.Fatalf("search(%q): %v", q, err)
		}
		if len(entries) != 0 {
			t.Errorf("search(%q) returned %d rows", q, len(entries))
		}
	}
}

func TestSearchCaseFoldingAndMarker(t *testing.T) {
	db := buildFixtureDB(t, dictionary.Eiji, fixtureLines(t))
	svc := newService(t, db, 50)

	lower, err := svc.Search(context.Background(), "care")
	if err != nil {
		t.Fatal(err)
	}
	upper, err := svc.Search(context.Background(), "CARE")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(headwords(lower)) != fmt.Sprint(headwords(upper)) {
		t.Errorf("case folding mismatch: %v vs %v", headwords(lower), headwords(upper))
	}

	// A query carrying the structural marker still matches because
	// normalization strips it.
	marked, err := svc.Search(context.Background(), "■care")
	if err != nil {
		t.Fatal(err)
	}
	if len(marked) == 0 {
		t.Error("marker-prefixed query returned nothing")
	}
}

func TestSearchLiteralFTSOperators(t *testing.T) {
	db := buildFixtureDB(t, dictionary.Eiji, []string{
		`AND OR : operators`,
		`plain : entry`,
		`"quoted" : entry with quotes`,
		`(paren) : entry`,
	})
	svc := newService(t, db, 50)
	for _, q := range []string{"and or", `"quoted"`, "(paren)"} {
		entries, err := svc.Search(context.Background(), q)
		if err != nil {
			t.Errorf("search(%q): %v", q, err)
			continue
		}
		if len(entries) == 0 {
			t.Errorf("search(%q): no results", q)
		}
	}
	// A non-matching operator-ish query must not error.
	if _, err := svc.Search(context.Background(), "NOT"); err != nil {
		t.Errorf("search(NOT) errored: %v", err)
	}
}

func TestSearchNoResults(t *testing.T) {
	db := buildFixtureDB(t, dictionary.Eiji, fixtureLines(t))
	svc := newService(t, db, 50)
	entries, err := svc.Search(context.Background(), "zzzzz")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected no results, got %v", headwords(entries))
	}
}

func TestNewServiceValidatesLimit(t *testing.T) {
	db := buildFixtureDB(t, dictionary.Eiji, fixtureLines(t))
	repo, err := search.OpenRepository(db, dictionary.Eiji)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	for _, n := range []int{0, -1, 501, 100000} {
		if _, err := search.NewService(repo, n); err == nil {
			t.Errorf("NewService(%d) unexpectedly succeeded", n)
		}
	}
	for _, n := range []int{1, 50, 500} {
		if _, err := search.NewService(repo, n); err != nil {
			t.Errorf("NewService(%d): %v", n, err)
		}
	}
}

func TestOpenRepositoryRejectsWrongDictionary(t *testing.T) {
	db := buildFixtureDB(t, dictionary.Eiji, fixtureLines(t))
	if _, err := search.OpenRepository(db, dictionary.Waei); err == nil {
		t.Error("opening an eiji database as waei unexpectedly succeeded")
	}
}

func TestOpenRepositoryRejectsMissingPrefixIndex(t *testing.T) {
	path := buildFixtureDB(t, dictionary.Eiji, fixtureLines(t))
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DROP INDEX idx_entries_headword_norm"); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	repo, err := search.OpenRepository(path, dictionary.Eiji)
	if repo != nil {
		repo.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "idx_entries_headword_norm") {
		t.Fatalf("OpenRepository error = %v, want missing prefix index", err)
	}
}

func TestOpenRepositoryRejectsPrefixIndexOnWrongColumn(t *testing.T) {
	path := buildFixtureDB(t, dictionary.Eiji, fixtureLines(t))
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DROP INDEX idx_entries_headword_norm"); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE INDEX idx_entries_headword_norm ON entries(body)"); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	repo, err := search.OpenRepository(path, dictionary.Eiji)
	if repo != nil {
		repo.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "want headword_norm") {
		t.Fatalf("OpenRepository error = %v, want wrong index column error", err)
	}
}

func TestOpenRepositoryRejectsInvalidFTSTable(t *testing.T) {
	path := buildFixtureDB(t, dictionary.Eiji, fixtureLines(t))
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DROP TABLE entries_fts"); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE entries_fts (headword_norm TEXT)"); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	repo, err := search.OpenRepository(path, dictionary.Eiji)
	if repo != nil {
		repo.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "fts smoke query") {
		t.Fatalf("OpenRepository error = %v, want FTS smoke query failure", err)
	}
}

func TestSearchContextCanceled(t *testing.T) {
	db := buildFixtureDB(t, dictionary.Eiji, fixtureLines(t))
	svc := newService(t, db, 50)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := svc.Search(ctx, "care"); err == nil {
		t.Error("canceled context unexpectedly succeeded")
	}
}

func hasPrefixNorm(t *testing.T, path string, id int64, prefix string) bool {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var norm string
	if err := db.QueryRow("SELECT headword_norm FROM entries WHERE id = ?", id).Scan(&norm); err != nil {
		t.Fatal(err)
	}
	return len(norm) >= len(prefix) && norm[:len(prefix)] == prefix
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
