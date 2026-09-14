package search

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"modernc.org/sqlite"
	libcsqlite "modernc.org/sqlite/lib"
)

// OpenCategory classifies why a dictionary database could not be opened
// (GUI design D45). The set is closed: frontends map each category to a
// canonical message and recovery action instead of parsing error text.
type OpenCategory string

const (
	// OpenMissing: the configured database file does not exist.
	OpenMissing OpenCategory = "missing"
	// OpenUnreadable: the file exists but cannot be accessed (permissions
	// and other access failures).
	OpenUnreadable OpenCategory = "unreadable"
	// OpenCorrupt: SQLite judges the file corrupt or not a database.
	OpenCorrupt OpenCategory = "corrupt"
	// OpenIncompatible: the file opens as SQLite but does not satisfy the
	// required schema, metadata versions, or FTS validation.
	OpenIncompatible OpenCategory = "incompatible"
	// OpenWrongDictionary: the metadata dictionary type belongs to the
	// other dictionary.
	OpenWrongDictionary OpenCategory = "wrong_dictionary"
	// OpenUnknown: anything else.
	OpenUnknown OpenCategory = "unknown"
)

// OpenError reports that a dictionary database could not be opened. Its
// message is exactly the human-readable error the CLI has always printed,
// so wrapping does not change CLI output or exit codes (GUI design D39);
// Category carries the machine-readable classification for frontends
// (GUI design D44).
type OpenError struct {
	category OpenCategory
	err      error
}

// Error returns the wrapped human-readable message.
func (e *OpenError) Error() string { return e.err.Error() }

// Unwrap returns the underlying cause.
func (e *OpenError) Unwrap() error { return e.err }

// Category returns the open failure classification.
func (e *OpenError) Category() OpenCategory { return e.category }

// OpenCategoryOf classifies err. ok is false when err is not an
// *OpenError; callers then treat the failure as OpenUnknown.
func OpenCategoryOf(err error) (OpenCategory, bool) {
	var oe *OpenError
	if errors.As(err, &oe) {
		return oe.category, true
	}
	return OpenUnknown, false
}

// classifyOpenFailure maps a sqlite.OpenReadOnly failure to a category.
// The database path is stat'ed only on the failure path, so the
// success-path behavior and error strings of OpenRepository are
// unchanged.
func classifyOpenFailure(path string, err error) OpenCategory {
	info, statErr := os.Stat(path)
	if statErr != nil {
		if errors.Is(statErr, fs.ErrNotExist) {
			return OpenMissing
		}
		return OpenUnreadable
	}
	if !info.Mode().IsRegular() {
		// Directories, devices, and other special files cannot be a
		// dictionary database.
		return OpenUnreadable
	}
	var derr *sqlite.Error
	if errors.As(err, &derr) {
		switch derr.Code() {
		case libcsqlite.SQLITE_CORRUPT, libcsqlite.SQLITE_NOTADB:
			return OpenCorrupt
		case libcsqlite.SQLITE_CANTOPEN, libcsqlite.SQLITE_PERM,
			libcsqlite.SQLITE_READONLY, libcsqlite.SQLITE_IOERR,
			libcsqlite.SQLITE_BUSY, libcsqlite.SQLITE_NOMEM:
			return OpenUnreadable
		}
	}
	return OpenUnknown
}

// verifyError is a schema validation failure carrying its category.
type verifyError struct {
	category OpenCategory
	err      error
}

func (e *verifyError) Error() string { return e.err.Error() }
func (e *verifyError) Unwrap() error { return e.err }

// incompatiblef builds an OpenIncompatible verifyError.
func incompatiblef(format string, args ...any) *verifyError {
	return &verifyError{category: OpenIncompatible, err: fmt.Errorf(format, args...)}
}

// classifyVerifyQueryError builds a verifyError whose category depends on
// the driver error: SQLite corruption seen while validating the schema is
// OpenCorrupt, everything else is a schema incompatibility.
func classifyVerifyQueryError(err error, format string, args ...any) *verifyError {
	category := OpenIncompatible
	var derr *sqlite.Error
	if errors.As(err, &derr) && derr.Code() == libcsqlite.SQLITE_CORRUPT {
		category = OpenCorrupt
	}
	return &verifyError{category: category, err: fmt.Errorf(format, args...)}
}
