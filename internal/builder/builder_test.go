package builder_test

import (
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/simosako/ejquick/internal/builder"
	"github.com/simosako/ejquick/internal/dictionary"
)

// encodeCP932Lines builds a small synthetic CP932 TXT fixture. The samples
// are artificial data written for this test, not dictionary content.
func encodeCP932Lines(t *testing.T, lines []string) []byte {
	t.Helper()
	var b strings.Builder
	for _, l := range lines {
		enc, err := encodeCP932(l)
		if err != nil {
			t.Fatalf("encode %q: %v", l, err)
		}
		b.Write(enc)
		b.WriteString("\r\n")
	}
	return []byte(b.String())
}

func writeFixture(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func runBuild(t *testing.T, opts builder.Options) (builder.Stats, error) {
	t.Helper()
	if opts.Progress == nil {
		opts.Progress = os.Stderr
	}
	return builder.Run(opts)
}

func TestBuildBasicDatabase(t *testing.T) {
	dir := t.TempDir()
	input := writeFixture(t, dir, "EIJIRO1-0.TXT", encodeCP932Lines(t, []string{
		"english : the English language",
		"English breakfast : a cooked breakfast",
		"cat : a small animal",
		"take care : be careful",
	}))
	output := filepath.Join(dir, "eiji.sqlite3")

	stats, err := runBuild(t, builder.Options{
		Type:     dictionary.Eiji,
		Input:    input,
		Output:   output,
		Progress: testWriter{t},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if stats.Entries != 4 || stats.SourceLines != 4 || stats.Skipped != 0 {
		t.Errorf("stats mismatch: %+v", stats)
	}
	if stats.DBSize <= 0 {
		t.Errorf("db size not recorded: %d", stats.DBSize)
	}

	// Open the published database read-only and verify contents.
	db := openRO(t, output)
	defer db.Close()

	var id int64
	var head, norm, body string
	err = db.QueryRow("SELECT id, headword, headword_norm, body FROM entries WHERE id = 1").
		Scan(&id, &head, &norm, &body)
	if err != nil {
		t.Fatalf("select id 1: %v", err)
	}
	if head != "english" || norm != "english" || body != "the English language" {
		t.Errorf("entry 1 mismatch: %q %q %q", head, norm, body)
	}

	// Normalization is applied: "English breakfast" folds to lower case.
	err = db.QueryRow("SELECT headword_norm FROM entries WHERE id = 2").Scan(&norm)
	if err != nil || norm != "english breakfast" {
		t.Errorf("entry 2 norm = %q err=%v", norm, err)
	}

	// FTS5 trigram index answers substring queries.
	var cnt int64
	err = db.QueryRow(
		`SELECT count(*) FROM entries_fts f JOIN entries e ON e.id = f.rowid
		 WHERE entries_fts MATCH ? AND instr(e.headword_norm, ?) > 0`,
		`"glish br"`, "glish br").Scan(&cnt)
	if err != nil {
		t.Fatalf("fts query: %v", err)
	}
	if cnt != 1 {
		t.Errorf("fts substring match = %d, want 1", cnt)
	}

	// Metadata basics.
	checkMeta(t, db, map[string]string{
		"schema_version":        "1",
		"dictionary_type":       "eiji",
		"source_version":        "1-0",
		"source_line_count":     "4",
		"entry_count":           "4",
		"skipped_entry_count":   "0",
		"encoding":              "CP932",
		"normalization_version": "1",
		"fts_version":           "1",
		"compacted":             "false",
	})
}

func TestBuildNilProgressDisablesOutput(t *testing.T) {
	dir := t.TempDir()
	input := writeFixture(t, dir, "EIJIRO1-0.TXT", encodeCP932Lines(t, []string{
		"alpha : first letter",
	}))

	stats, err := builder.Run(builder.Options{
		Type:   dictionary.Eiji,
		Input:  input,
		Output: filepath.Join(dir, "eiji.sqlite3"),
	})
	if err != nil {
		t.Fatalf("Run with nil Progress: %v", err)
	}
	if stats.Entries != 1 {
		t.Errorf("entries = %d, want 1", stats.Entries)
	}
}

func TestBuildSkipsKeepIDGaps(t *testing.T) {
	dir := t.TempDir()
	input := writeFixture(t, dir, "WAEIJI1-0.TXT", encodeCP932Lines(t, []string{
		"ねこ : cat",       // line 1 ok
		"malformed line", // line 2 skipped: missing separator
		"a : b : c",      // line 3 skipped: multiple separators
		"いぬ : dog",       // line 4 ok
	}))
	output := filepath.Join(dir, "waei.sqlite3")

	stats, err := runBuild(t, builder.Options{
		Type:     dictionary.Waei,
		Input:    input,
		Output:   output,
		Progress: testWriter{t},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if stats.SourceLines != 4 || stats.Entries != 2 || stats.Skipped != 2 {
		t.Fatalf("stats mismatch: %+v", stats)
	}

	db := openRO(t, output)
	defer db.Close()

	// ids are physical line numbers: 1 and 4 exist, 2 and 3 are gaps.
	var ids []int64
	rows, err := db.Query("SELECT id FROM entries ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != 1 || ids[1] != 4 {
		t.Errorf("ids = %v, want [1 4]", ids)
	}

	// Waei normalization: full-width characters are NFKC-folded.
	var norm string
	if err := db.QueryRow("SELECT headword_norm FROM entries WHERE id = 4").Scan(&norm); err != nil {
		t.Fatal(err)
	}
	if norm != "いぬ" {
		t.Errorf("waei norm = %q", norm)
	}

	checkMeta(t, db, map[string]string{
		"source_line_count":   "4",
		"entry_count":         "2",
		"skipped_entry_count": "2",
		"dictionary_type":     "waei",
		"compacted":           "false",
	})
}

func TestBuildOutputExistsWithoutForce(t *testing.T) {
	dir := t.TempDir()
	input := writeFixture(t, dir, "in.TXT", encodeCP932Lines(t, []string{"a : b"}))
	output := filepath.Join(dir, "eiji.sqlite3")
	if err := os.WriteFile(output, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := runBuild(t, builder.Options{
		Type: dictionary.Eiji, Input: input, Output: output,
	})
	if err == nil {
		t.Fatal("expected error when output exists without --force")
	}
	// The existing file must be untouched.
	data, _ := os.ReadFile(output)
	if string(data) != "existing" {
		t.Error("existing output was modified")
	}
	// No temporary database is left behind.
	assertNoTempFiles(t, dir)
}

func TestBuildRejectsInputAsOutput(t *testing.T) {
	data := encodeCP932Lines(t, []string{"source : must remain intact"})

	tests := []struct {
		name       string
		force      bool
		outputPath func(t *testing.T, input string) string
	}{
		{
			name: "same path without force",
			outputPath: func(_ *testing.T, input string) string {
				return input
			},
		},
		{
			name:  "same path with force",
			force: true,
			outputPath: func(_ *testing.T, input string) string {
				return input
			},
		},
		{
			name:  "relative path",
			force: true,
			outputPath: func(t *testing.T, input string) string {
				wd, err := os.Getwd()
				if err != nil {
					t.Fatal(err)
				}
				rel, err := filepath.Rel(wd, input)
				if err != nil {
					t.Fatal(err)
				}
				return rel
			},
		},
		{
			name:  "symlink",
			force: true,
			outputPath: func(t *testing.T, input string) string {
				output := filepath.Join(filepath.Dir(input), "output.sqlite3")
				if err := os.Symlink(input, output); err != nil {
					t.Skipf("symlinks are not supported: %v", err)
				}
				return output
			},
		},
		{
			name:  "hard link",
			force: true,
			outputPath: func(t *testing.T, input string) string {
				output := filepath.Join(filepath.Dir(input), "output.sqlite3")
				if err := os.Link(input, output); err != nil {
					t.Skipf("hard links are not supported: %v", err)
				}
				return output
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			input := writeFixture(t, dir, "Input.TXT", data)
			output := tt.outputPath(t, input)

			_, err := runBuild(t, builder.Options{
				Type: dictionary.Eiji, Input: input, Output: output, Force: tt.force,
			})
			if err == nil || !strings.Contains(err.Error(), "same file") {
				t.Fatalf("Run error = %v, want same-file error", err)
			}
			got, err := os.ReadFile(input)
			if err != nil {
				t.Fatalf("read input after rejected build: %v", err)
			}
			if string(got) != string(data) {
				t.Fatal("input was modified")
			}
			inputInfo, err := os.Stat(input)
			if err != nil {
				t.Fatal(err)
			}
			outputInfo, err := os.Stat(output)
			if err != nil {
				t.Fatalf("stat output after rejected build: %v", err)
			}
			if !os.SameFile(inputInfo, outputInfo) {
				t.Fatal("output no longer refers to input")
			}
			assertNoTempFiles(t, dir)
		})
	}
}

func TestBuildForceReplacesOutput(t *testing.T) {
	dir := t.TempDir()
	input := writeFixture(t, dir, "EIJIRO1-0.TXT", encodeCP932Lines(t, []string{
		"one : 1",
		"removed : old only",
		"two : 2",
	}))
	output := filepath.Join(dir, "eiji.sqlite3")

	if _, err := runBuild(t, builder.Options{
		Type: dictionary.Eiji, Input: input, Output: output, Progress: testWriter{t},
	}); err != nil {
		t.Fatal(err)
	}

	// Rebuild from modified input with --force. The old-only entry must not
	// survive, and IDs must follow the new physical line positions.
	input2 := writeFixture(t, dir, "EIJIRO1-0.TXT", encodeCP932Lines(t, []string{
		"inserted : new first line",
		"one : first",
		"two : second",
	}))
	stats, err := runBuild(t, builder.Options{
		Type: dictionary.Eiji, Input: input2, Output: output, Force: true, Progress: testWriter{t},
	})
	if err != nil {
		t.Fatalf("force rebuild: %v", err)
	}
	if stats.Entries != 3 {
		t.Fatalf("entries = %d, want 3", stats.Entries)
	}

	db := openRO(t, output)
	defer db.Close()
	var removed int64
	if err := db.QueryRow("SELECT count(*) FROM entries WHERE headword_norm = 'removed'").Scan(&removed); err != nil {
		t.Fatal(err)
	}
	if removed != 0 {
		t.Errorf("old-only entries after rebuild = %d, want 0", removed)
	}
	rows, err := db.Query("SELECT id, headword FROM entries ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var id int64
		var headword string
		if err := rows.Scan(&id, &headword); err != nil {
			t.Fatal(err)
		}
		got = append(got, fmt.Sprintf("%d:%s", id, headword))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	want := []string{"1:inserted", "2:one", "3:two"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("entries = %v, want %v", got, want)
	}
	assertNoTempFiles(t, dir)
}

func TestBuildFailurePreservesExistingOutput(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{name: "build failure", data: append([]byte("ok : fine\r\n"), 0x80, '\n')},
		{name: "validation failure", data: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			input := writeFixture(t, dir, "EIJIRO1-0.TXT", tt.data)
			output := writeFixture(t, dir, "eiji.sqlite3", []byte("existing database"))
			var progress bytes.Buffer

			if _, err := builder.Run(builder.Options{
				Type: dictionary.Eiji, Input: input, Output: output, Force: true, Progress: &progress,
			}); err == nil {
				t.Fatal("build unexpectedly succeeded")
			}
			got, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != "existing database" {
				t.Errorf("existing output was changed to %q", got)
			}
			if !strings.Contains(progress.String(), ": failed:") {
				t.Errorf("progress has no failed phase: %q", progress.String())
			}
			assertNoTempFiles(t, dir)
		})
	}
}

func TestBuildProgressHasFixedLines(t *testing.T) {
	dir := t.TempDir()
	input := writeFixture(t, dir, "EIJIRO1-0.TXT", encodeCP932Lines(t, []string{
		"alpha : first letter",
	}))
	var progress bytes.Buffer
	if _, err := builder.Run(builder.Options{
		Type: dictionary.Eiji, Input: input, Output: filepath.Join(dir, "eiji.sqlite3"), Progress: &progress,
	}); err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(progress.String(), "\r\x1b") {
		t.Fatalf("progress contains terminal control characters: %q", progress.String())
	}
	lines := strings.Split(strings.TrimSuffix(progress.String(), "\n"), "\n")
	wantFixed := []string{
		"Reading: start",
		"Reading: done lines=1 entries=1 skipped=0",
		"B-tree index: start",
		"B-tree index: done",
		"FTS build: start",
		"FTS build: done",
		"ANALYZE: start",
		"ANALYZE: done",
		"Validation: start",
		"Validation: done",
		"Source lines: 1",
		"Entries: 1",
		"Skipped: 0",
	}
	if len(lines) != len(wantFixed)+3 {
		t.Fatalf("progress lines = %d, want %d: %q", len(lines), len(wantFixed)+3, progress.String())
	}
	for i, want := range wantFixed {
		if lines[i] != want {
			t.Errorf("progress line %d = %q, want %q", i+1, lines[i], want)
		}
	}
	if !strings.HasPrefix(lines[13], "Database: ") {
		t.Errorf("database summary = %q", lines[13])
	}
	if lines[14] != "Compacted: no" {
		t.Errorf("compact summary = %q", lines[14])
	}
	elapsed := strings.TrimPrefix(lines[15], "Elapsed: ")
	if _, err := time.ParseDuration(elapsed); err != nil {
		t.Errorf("elapsed summary = %q: %v", lines[15], err)
	}
	if strings.Contains(progress.String(), "VACUUM:") {
		t.Errorf("normal build reported VACUUM: %q", progress.String())
	}
}

func TestBuildCompactSetsMetadata(t *testing.T) {
	dir := t.TempDir()
	input := writeFixture(t, dir, "EIJIRO1-0.TXT", encodeCP932Lines(t, []string{
		"alpha : a",
		"beta : b",
		"gamma : c",
	}))
	output := filepath.Join(dir, "eiji.sqlite3")

	var progress bytes.Buffer
	if _, err := runBuild(t, builder.Options{
		Type: dictionary.Eiji, Input: input, Output: output, Compact: true, Progress: &progress,
	}); err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"VACUUM: start\n", "VACUUM: done\n", "ANALYZE: start\n", "Validation: done\n"} {
		if strings.Count(progress.String(), line) != 1 {
			t.Errorf("progress occurrence of %q is not one: %q", line, progress.String())
		}
	}
	db := openRO(t, output)
	defer db.Close()
	checkMeta(t, db, map[string]string{"compacted": "true"})
}

func TestBuildDecodeErrorStops(t *testing.T) {
	dir := t.TempDir()
	// Valid line followed by an invalid CP932 lead byte 0x80.
	data := append([]byte("ok : fine\r\n"), 0x80, '\n')
	input := writeFixture(t, dir, "EIJIRO1-0.TXT", data)
	output := filepath.Join(dir, "eiji.sqlite3")

	_, err := runBuild(t, builder.Options{
		Type: dictionary.Eiji, Input: input, Output: output, Progress: testWriter{t},
	})
	if err == nil {
		t.Fatal("expected decode error")
	}
	if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
		t.Error("output must not exist after failure")
	}
	assertNoTempFiles(t, dir)
}

func TestBuildEmptyInputFailsValidation(t *testing.T) {
	dir := t.TempDir()
	input := writeFixture(t, dir, "EIJIRO1-0.TXT", []byte(""))
	output := filepath.Join(dir, "eiji.sqlite3")

	// A build with zero entries cannot pass the smoke queries and must
	// fail without publishing anything.
	_, err := runBuild(t, builder.Options{
		Type: dictionary.Eiji, Input: input, Output: output, Progress: testWriter{t},
	})
	if err == nil {
		t.Fatal("expected validation error for empty dictionary")
	}
	if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
		t.Error("output must not be published")
	}
	assertNoTempFiles(t, dir)
}

// --- helpers ---

func openRO(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}
	return db
}

func checkMeta(t *testing.T, db *sql.DB, want map[string]string) {
	t.Helper()
	rows, err := db.Query("SELECT key, value FROM metadata")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			t.Fatal(err)
		}
		got[k] = v
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("metadata %s = %q, want %q", k, got[k], w)
		}
	}
}

func assertNoTempFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".ejquick-build-") {
			t.Errorf("temporary database left behind: %s", e.Name())
		}
	}
}

// testWriter routes builder progress output to t.Log.
type testWriter struct{ t *testing.T }

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Log(string(p))
	return len(p), nil
}

// encodeCP932 encodes s to CP932 using a reverse table built from the
// parser package's decode table via a small brute-force map for the
// characters used in tests. Only ASCII and a fixed set of Japanese
// characters are needed here, so we encode directly.
func encodeCP932(s string) ([]byte, error) {
	var out []byte
	for _, r := range s {
		switch {
		case r < 0x80:
			out = append(out, byte(r))
		default:
			b, ok := cp932TestEncode[r]
			if !ok {
				return nil, fmt.Errorf("no CP932 encoding for %q in test fixture", r)
			}
			out = append(out, b...)
		}
	}
	return out, nil
}

// cp932TestEncode maps the few non-ASCII runes used by the fixtures.
var cp932TestEncode = map[rune][]byte{
	'ね': {0x82, 0xCB},
	'こ': {0x82, 0xB1},
	'い': {0x82, 0xA2},
	'ぬ': {0x82, 0xCA},
	'犬': {0x8C, 0xA2},
}
