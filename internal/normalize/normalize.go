// Package normalize implements the per-dictionary headword normalization
// used both by the builder (to generate headword_norm) and by the search
// application (to normalize user queries). Both sides must use exactly the
// same functions so that B-tree and FTS5 matching agree.
package normalize

import (
	"fmt"
	"strings"

	"golang.org/x/text/cases"

	"github.com/simosako/ejquick/internal/dictionary"
)

// Version identifies the normalization rule set. It is stored in the DB
// metadata as normalization_version; changing the rules requires a new
// version and a rebuild.
const Version = "1"

// folder implements Unicode full case folding without any
// language-specific mappings.
var folder = cases.Fold()

// Normalize returns the normalized search key for a headword or query
// according to the dictionary type:
//
//	eiji: trim spaces, NFC, then Unicode case folding
//	waei: trim spaces, NFKC, then Unicode case folding
//
// NFC/NFKC are applied first so that folding sees canonically equivalent
// code point sequences in a single form.
func Normalize(dt dictionary.Type, s string) (string, error) {
	trimmed := strings.TrimSpace(s)
	var normalized string
	switch dt {
	case dictionary.Eiji:
		normalized = fold(nfc(trimmed))
	case dictionary.Waei:
		normalized = fold(nfkc(trimmed))
	default:
		return "", fmt.Errorf("normalize: unknown dictionary type %q", dt)
	}
	return normalized, nil
}

// MustNormalize is Normalize for values already validated by the caller.
// It panics on an unknown dictionary type, which is a programming error.
func MustNormalize(dt dictionary.Type, s string) string {
	out, err := Normalize(dt, s)
	if err != nil {
		panic(err)
	}
	return out
}

// EmptyAfter reports whether the string becomes empty after normalization,
// which makes it unusable as a search query.
func EmptyAfter(dt dictionary.Type, s string) bool {
	return MustNormalize(dt, s) == ""
}
