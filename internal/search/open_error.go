package search

import (
	"errors"

	modernsqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// OpenCategory classifies repository startup failures for frontends.
type OpenCategory string

const (
	OpenMissing         OpenCategory = "missing"
	OpenUnreadable      OpenCategory = "unreadable"
	OpenCorrupt         OpenCategory = "corrupt"
	OpenIncompatible    OpenCategory = "incompatible"
	OpenWrongDictionary OpenCategory = "wrong_dictionary"
	OpenUnknown         OpenCategory = "unknown"
)

// OpenError preserves a repository open failure and exposes a stable category.
type OpenError struct {
	Category OpenCategory
	Err      error
}

func (e *OpenError) Error() string { return e.Err.Error() }
func (e *OpenError) Unwrap() error { return e.Err }

// OpenCategoryOf returns the stable category of err, or OpenUnknown.
func OpenCategoryOf(err error) OpenCategory {
	var openErr *OpenError
	if errors.As(err, &openErr) {
		return openErr.Category
	}
	return OpenUnknown
}

func openFailure(category OpenCategory, err error) error {
	return &OpenError{Category: category, Err: err}
}

func sqliteOpenCategory(err error) OpenCategory {
	var sqliteErr *modernsqlite.Error
	if !errors.As(err, &sqliteErr) {
		return OpenUnknown
	}
	code := sqliteErr.Code() & 0xff
	switch code {
	case sqlite3.SQLITE_CORRUPT, sqlite3.SQLITE_NOTADB:
		return OpenCorrupt
	case sqlite3.SQLITE_CANTOPEN, sqlite3.SQLITE_PERM, sqlite3.SQLITE_AUTH,
		sqlite3.SQLITE_READONLY, sqlite3.SQLITE_IOERR:
		return OpenUnreadable
	default:
		return OpenUnknown
	}
}

type schemaFailure struct {
	category OpenCategory
	err      error
}

func (e *schemaFailure) Error() string { return e.err.Error() }
func (e *schemaFailure) Unwrap() error { return e.err }
