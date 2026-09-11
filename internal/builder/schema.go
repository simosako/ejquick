// Package builder converts a CP932 dictionary TXT file into a read-only
// SQLite database with a B-tree index for prefix search and an FTS5
// trigram index for substring search.
package builder

import "github.com/simosako/ejquick/internal/normalize"

// SchemaVersion identifies the database layout produced by the builder.
// The search application refuses databases with an unknown value.
const SchemaVersion = "1"

// FTSVersion identifies the FTS5 configuration (tokenizer options and
// indexed columns). Changing the configuration requires a new version and
// a rebuild.
const FTSVersion = "1"

// BuilderVersion is the version string stored in the database metadata.
// It is overridden at link time by the build command via SetVersion.
var BuilderVersion = "dev"

// entriesSchema creates only the entries table: indexes are added after
// the bulk insert for better build performance.
const entriesSchema = `
CREATE TABLE entries (
    id            INTEGER PRIMARY KEY,
    headword      TEXT NOT NULL,
    headword_norm TEXT NOT NULL,
    body          TEXT NOT NULL
);
`

// indexSchema creates the B-tree index used for prefix search.
const indexSchema = `
CREATE INDEX idx_entries_headword_norm
ON entries(headword_norm);
`

// ftsSchema creates the FTS5 external content table over headword_norm
// with the trigram tokenizer. case_sensitive=1 and remove_diacritics=0
// keep FTS5 from re-folding text that normalization already handled.
const ftsSchema = `
CREATE VIRTUAL TABLE entries_fts
USING fts5(
    headword_norm,
    content='entries',
    content_rowid='id',
    tokenize='trigram case_sensitive 1 remove_diacritics 0'
);
`

// metadataSchema creates the key/value metadata table.
const metadataSchema = `
CREATE TABLE metadata (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
`

// pragmas are executed on the temporary build database before any schema
// is created. page_size must come first. The negative cache_size is a
// KiB-denominated cap (128 MiB).
const pragmas = `
PRAGMA page_size = 4096;
PRAGMA journal_mode = OFF;
PRAGMA synchronous = OFF;
PRAGMA locking_mode = EXCLUSIVE;
PRAGMA temp_store = FILE;
PRAGMA cache_size = -131072;
`

// Metadata keys stored in the metadata table.
const (
	KeySchemaVersion     = "schema_version"
	KeyDictionaryType    = "dictionary_type"
	KeySourceVersion     = "source_version"
	KeySourceFilename    = "source_filename"
	KeyBuildTime         = "build_time"
	KeyBuilderVersion    = "builder_version"
	KeySourceLineCount   = "source_line_count"
	KeyEntryCount        = "entry_count"
	KeySkippedEntryCount = "skipped_entry_count"
	KeyEncoding          = "encoding"
	KeyNormalizationVer  = "normalization_version"
	KeyFTSVersion        = "fts_version"
	KeyCompacted         = "compacted"
)

// Encoding is the fixed source encoding name stored in metadata.
const Encoding = "CP932"

// NormalizationVersion returns the normalization rule version used to
// generate headword_norm.
func NormalizationVersion() string { return normalize.Version }
