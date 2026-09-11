package builder

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/simosako/ejquick/internal/dictionary"
	"github.com/simosako/ejquick/internal/normalize"
	"github.com/simosako/ejquick/internal/parser"
	"github.com/simosako/ejquick/internal/sqlite"
)

// progressEvery is the physical-line interval for reading progress lines.
const progressEvery = 100_000

// Options controls one build run.
type Options struct {
	// Type selects the dictionary and its normalization rules.
	Type dictionary.Type
	// Input is the source CP932 TXT file.
	Input string
	// Output is the final database path.
	Output string
	// Force replaces an existing output database.
	Force bool
	// Compact runs VACUUM before final validation.
	Compact bool
	// Progress receives the fixed-line progress output (usually stderr).
	// nil disables output.
	Progress io.Writer
}

// Stats summarizes a successful build.
type Stats struct {
	SourceLines int64
	Entries     int64
	Skipped     int64
	DBSize      int64
	Elapsed     time.Duration
	Compacted   bool
}

// Run executes the whole conversion: build into a temporary database in
// the output directory, validate it read-only, then atomically publish it.
func Run(opts Options) (Stats, error) {
	start := time.Now()
	var stats Stats

	if !dictionary.IsValid(opts.Type) {
		return stats, fmt.Errorf("invalid dictionary type %q", opts.Type)
	}
	if opts.Input == "" || opts.Output == "" {
		return stats, errors.New("input and output paths are required")
	}

	in, err := os.Open(opts.Input)
	if err != nil {
		return stats, fmt.Errorf("open input: %w", err)
	}
	defer in.Close()

	// Refuse to touch an existing output unless --force was given.
	if _, err := os.Stat(opts.Output); err == nil && !opts.Force {
		return stats, fmt.Errorf("output %s already exists (use --force to replace)", opts.Output)
	} else if err != nil && !os.IsNotExist(err) {
		return stats, fmt.Errorf("stat output: %w", err)
	}

	// Create the temporary database in the output directory so the
	// final publish is a same-filesystem rename.
	if err := os.MkdirAll(filepath.Dir(opts.Output), 0o755); err != nil {
		return stats, fmt.Errorf("create output directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(opts.Output), ".ejquick-build-*.sqlite3")
	if err != nil {
		return stats, fmt.Errorf("create temporary database: %w", err)
	}
	tmpPath := tmp.Name()
	// Close the placeholder file handle; SQLite opens its own.
	if err := tmp.Close(); err != nil {
		return stats, fmt.Errorf("close temporary placeholder: %w", err)
	}
	os.Remove(tmpPath)

	// Everything below must clean up the temporary database on error.
	buildErr := func() error {
		db, err := sqlite.OpenReadWrite(tmpPath)
		if err != nil {
			return err
		}
		if err := build(db, in, opts, &stats); err != nil {
			db.Close()
			return err
		}
		if err := db.Close(); err != nil {
			return fmt.Errorf("close build database: %w", err)
		}
		return nil
	}()
	if buildErr != nil {
		os.Remove(tmpPath)
		return stats, buildErr
	}

	// Validate the finished temporary database read-only.
	if err := validate(tmpPath, opts, stats); err != nil {
		os.Remove(tmpPath)
		return stats, fmt.Errorf("validation failed: %w", err)
	}

	// Publish: sync the file, then atomically rename over the output.
	if err := publish(tmpPath, opts.Output); err != nil {
		os.Remove(tmpPath)
		return stats, fmt.Errorf("publish: %w", err)
	}

	fi, err := os.Stat(opts.Output)
	if err != nil {
		return stats, fmt.Errorf("stat published database: %w", err)
	}
	stats.DBSize = fi.Size()
	stats.Compacted = opts.Compact
	stats.Elapsed = time.Since(start)

	writeSummary(opts.Progress, stats)
	return stats, nil
}

// build performs all write operations on the temporary database.
func build(db *sql.DB, in io.Reader, opts Options, stats *Stats) error {
	if _, err := db.Exec(pragmas); err != nil {
		return fmt.Errorf("apply pragmas: %w", err)
	}
	if _, err := db.Exec(entriesSchema); err != nil {
		return fmt.Errorf("create entries table: %w", err)
	}
	if err := insertEntries(db, in, opts, stats); err != nil {
		return err
	}
	if err := phase(opts.Progress, "B-tree index", func() error {
		_, err := db.Exec(indexSchema)
		return err
	}); err != nil {
		return fmt.Errorf("create b-tree index: %w", err)
	}
	if err := phase(opts.Progress, "FTS build", func() error {
		if _, err := db.Exec(ftsSchema); err != nil {
			return err
		}
		if _, err := db.Exec("INSERT INTO entries_fts(entries_fts) VALUES('rebuild')"); err != nil {
			return err
		}
		_, err := db.Exec("INSERT INTO entries_fts(entries_fts) VALUES('integrity-check')")
		return err
	}); err != nil {
		return fmt.Errorf("build fts index: %w", err)
	}
	if opts.Compact {
		if err := phase(opts.Progress, "VACUUM", func() error {
			_, err := db.Exec("VACUUM")
			return err
		}); err != nil {
			return fmt.Errorf("vacuum: %w", err)
		}
	}
	if err := phase(opts.Progress, "ANALYZE", func() error {
		_, err := db.Exec("ANALYZE")
		return err
	}); err != nil {
		return fmt.Errorf("analyze: %w", err)
	}
	if _, err := db.Exec("PRAGMA optimize"); err != nil {
		return fmt.Errorf("pragma optimize: %w", err)
	}
	return writeMetadata(db, opts, stats)
}

// insertEntries streams the source file into the entries table inside a
// single transaction, printing progress every progressEvery physical
// lines and at the end of input.
func insertEntries(db *sql.DB, in io.Reader, opts Options, stats *Stats) error {
	prog := opts.Progress
	fmt.Fprintln(prog, "Reading: start")

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin insert transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("INSERT INTO entries(id, headword, headword_norm, body) VALUES (?, ?, ?, ?)")
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	r := parser.NewReader(in)
	var nrm string
	for {
		line, err := r.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err // decode errors are fatal
		}
		stats.SourceLines = line.LineNo
		if line.Entry != nil {
			nrm, err = normalize.Normalize(opts.Type, line.Entry.Headword)
			if err != nil {
				return fmt.Errorf("normalize line %d: %w", line.LineNo, err)
			}
			if _, err := stmt.Exec(line.LineNo, line.Entry.Headword, nrm, line.Entry.Body); err != nil {
				return fmt.Errorf("insert line %d: %w", line.LineNo, err)
			}
			stats.Entries++
		} else {
			stats.Skipped++
			// Report skips as line number + reason only; never the
			// full line contents.
			fmt.Fprintf(prog, "Skipped line %d: %s\n", line.Skip.LineNo, line.Skip.Reason)
		}
		if stats.SourceLines%progressEvery == 0 {
			fmt.Fprintf(prog, "Reading: lines=%d entries=%d skipped=%d\n",
				stats.SourceLines, stats.Entries, stats.Skipped)
		}
	}
	fmt.Fprintf(prog, "Reading: done lines=%d entries=%d skipped=%d\n",
		stats.SourceLines, stats.Entries, stats.Skipped)

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit insert transaction: %w", err)
	}
	return nil
}

// sourceVersion extracts a version like "144-10" from the input filename,
// falling back to "unknown".
func sourceVersion(input string) string {
	base := filepath.Base(input)
	if m := regexp.MustCompile(`([0-9]+-[0-9]+)`).FindStringSubmatch(base); m != nil {
		return m[1]
	}
	return "unknown"
}

// writeMetadata stores the build metadata after all data is in place.
func writeMetadata(db *sql.DB, opts Options, stats *Stats) error {
	if _, err := db.Exec(metadataSchema); err != nil {
		return fmt.Errorf("create metadata table: %w", err)
	}
	compact := "false"
	if opts.Compact {
		compact = "true"
	}
	pairs := [][2]string{
		{KeySchemaVersion, SchemaVersion},
		{KeyDictionaryType, opts.Type.String()},
		{KeySourceVersion, sourceVersion(opts.Input)},
		{KeySourceFilename, filepath.Base(opts.Input)},
		{KeyBuildTime, time.Now().UTC().Format(time.RFC3339)},
		{KeyBuilderVersion, BuilderVersion},
		{KeySourceLineCount, fmt.Sprintf("%d", stats.SourceLines)},
		{KeyEntryCount, fmt.Sprintf("%d", stats.Entries)},
		{KeySkippedEntryCount, fmt.Sprintf("%d", stats.Skipped)},
		{KeyEncoding, Encoding},
		{KeyNormalizationVer, NormalizationVersion()},
		{KeyFTSVersion, FTSVersion},
		{KeyCompacted, compact},
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare("INSERT INTO metadata(key, value) VALUES (?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, kv := range pairs {
		if _, err := stmt.Exec(kv[0], kv[1]); err != nil {
			return fmt.Errorf("write metadata %s: %w", kv[0], err)
		}
	}
	return tx.Commit()
}

// validate reopens the finished database read-only and checks schema,
// metadata, counts, and that both search paths return results.
func validate(path string, opts Options, stats Stats) error {
	phaseStart(opts.Progress, "Validation")

	db, err := sqlite.OpenReadOnly(path)
	if err != nil {
		return err
	}
	defer db.Close()

	// Required tables exist.
	for _, table := range []string{"entries", "entries_fts", "metadata"} {
		var name string
		err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE name = ?", table).Scan(&name)
		if err != nil {
			return fmt.Errorf("missing table %s: %w", table, err)
		}
	}

	// Metadata values.
	meta := map[string]string{}
	rows, err := db.Query("SELECT key, value FROM metadata")
	if err != nil {
		return fmt.Errorf("read metadata: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return err
		}
		meta[k] = v
	}
	if err := rows.Err(); err != nil {
		return err
	}
	checks := map[string]string{
		KeySchemaVersion:    SchemaVersion,
		KeyDictionaryType:   opts.Type.String(),
		KeyNormalizationVer: NormalizationVersion(),
		KeyFTSVersion:       FTSVersion,
		KeyEncoding:         Encoding,
		KeySourceLineCount:  fmt.Sprintf("%d", stats.SourceLines),
		KeyEntryCount:       fmt.Sprintf("%d", stats.Entries),
		KeySkippedEntryCount: fmt.Sprintf("%d", stats.Skipped),
	}
	wantCompact := "false"
	if opts.Compact {
		wantCompact = "true"
	}
	checks[KeyCompacted] = wantCompact
	for k, want := range checks {
		if meta[k] != want {
			return fmt.Errorf("metadata %s = %q, want %q", k, meta[k], want)
		}
	}

	// Row count matches the insert statistics.
	var count int64
	if err := db.QueryRow("SELECT count(*) FROM entries").Scan(&count); err != nil {
		return fmt.Errorf("count entries: %w", err)
	}
	if count != stats.Entries {
		return fmt.Errorf("entries count = %d, want %d", count, stats.Entries)
	}

	// Smoke tests: one prefix query and one FTS substring query.
	var smokeID int64
	err = db.QueryRow(
		"SELECT id FROM entries ORDER BY headword_norm LIMIT 1").Scan(&smokeID)
	if err != nil {
		return fmt.Errorf("prefix smoke query: %w", err)
	}
	var norm string
	if err := db.QueryRow("SELECT headword_norm FROM entries WHERE id = ?", smokeID).Scan(&norm); err != nil {
		return fmt.Errorf("read smoke headword: %w", err)
	}
	// The whole normalized headword is itself a trigram-matchable
	// substring whenever it has at least three runes.
	runes := []rune(norm)
	if len(runes) >= 3 {
		ftsQuery := `"` + strings.ReplaceAll(norm, `"`, `""`) + `"`
		var ftsCount int64
		err = db.QueryRow(
			`SELECT count(*) FROM entries_fts f JOIN entries e ON e.id = f.rowid
			 WHERE entries_fts MATCH ? AND instr(e.headword_norm, ?) > 0`,
			ftsQuery, norm).Scan(&ftsCount)
		if err != nil {
			return fmt.Errorf("fts smoke query: %w", err)
		}
		if ftsCount < 1 {
			return fmt.Errorf("fts smoke query matched nothing for %q", norm)
		}
	}

	phaseDone(opts.Progress, "Validation")
	return nil
}

// publish syncs the temporary file and atomically renames it over the
// output path, then syncs the directory when the OS supports it.
func publish(tmpPath, output string) error {
	f, err := os.Open(tmpPath)
	if err != nil {
		return fmt.Errorf("open temporary for sync: %w", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("sync temporary database: %w", err)
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, output); err != nil {
		return fmt.Errorf("rename over output: %w", err)
	}
	if runtime.GOOS == "windows" {
		// Windows cannot fsync directories; the rename itself is
		// durable enough for the initial release.
		return nil
	}
	d, err := os.Open(filepath.Dir(output))
	if err != nil {
		return fmt.Errorf("open output directory for sync: %w", err)
	}
	if err := d.Sync(); err != nil {
		d.Close()
		return fmt.Errorf("sync output directory: %w", err)
	}
	return d.Close()
}

// phase prints "Name: start", runs fn, then prints "Name: done" or
// "Name: failed".
func phase(w io.Writer, name string, fn func() error) error {
	phaseStart(w, name)
	if err := fn(); err != nil {
		fmt.Fprintf(w, "%s: failed: %v\n", name, err)
		return err
	}
	phaseDone(w, name)
	return nil
}

func phaseStart(w io.Writer, name string) { fmt.Fprintf(w, "%s: start\n", name) }
func phaseDone(w io.Writer, name string)  { fmt.Fprintf(w, "%s: done\n", name) }

// writeSummary prints the fixed-line completion summary.
func writeSummary(w io.Writer, stats Stats) {
	if w == nil {
		return
	}
	fmt.Fprintf(w, "Source lines: %d\n", stats.SourceLines)
	fmt.Fprintf(w, "Entries: %d\n", stats.Entries)
	fmt.Fprintf(w, "Skipped: %d\n", stats.Skipped)
	fmt.Fprintf(w, "Database: %s\n", humanBytes(stats.DBSize))
	fmt.Fprintf(w, "Compacted: %s\n", yesNo(stats.Compacted))
	fmt.Fprintf(w, "Elapsed: %s\n", stats.Elapsed.Truncate(time.Millisecond))
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// humanBytes renders a byte count with binary prefixes.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// SetVersion overrides the builder version string (used by the build
// command to stamp a release version).
func SetVersion(v string) { BuilderVersion = v }
