// Canonical English UI text for the search GUI (design D41). Messages
// are whole units: the code never concatenates fragments to build
// sentences, and counts are formatted per state (design D17).
package gui

import (
	"fmt"

	"github.com/simosako/ejquick/internal/dictionary"
	"github.com/simosako/ejquick/internal/search"
)

// DictionaryLabel returns the display name of a dictionary type. The GUI
// shows "English–Japanese" / "Japanese–English" (with an en dash) while
// config, protocol, and log identifiers keep using eiwa/waei (design
// D41).
func DictionaryLabel(dt dictionary.Type) string {
	switch dt {
	case dictionary.Eiwa:
		return "English–Japanese"
	case dictionary.Waei:
		return "Japanese–English"
	}
	return string(dt)
}

// Search error and empty/no-result texts are fixed (design D47): a
// single generic message with no per-category variants, and retry is
// simply editing the query.
const (
	searchFailedTitle = "Search failed"
	searchFailedHint  = "Edit the query to try again"
	noResultsText     = "No matching headwords"
)

// SearchErrorText returns the two-line search failure message.
func SearchErrorText() []string {
	return []string{searchFailedTitle, searchFailedHint}
}

// NoResultsText returns the message shown for a zero-result query.
func NoResultsText() []string {
	return []string{noResultsText}
}

// CountText formats the result count label (design D17): "N shown"
// normally, "Showing first N" when the result limit was reached. Empty
// query, zero results, and search errors show no count.
func CountText(shown, maxResults int) string {
	switch {
	case shown <= 0:
		return ""
	case shown >= maxResults:
		return fmt.Sprintf("Showing first %d", shown)
	default:
		return fmt.Sprintf("%d shown", shown)
	}
}

// dictionaryUnavailable returns the canonical message for a dictionary
// database that could not be opened (design D45). The Build Dictionary
// Database... recovery action is intentionally not produced here yet: it
// arrives with the builder milestone (design D5/D20/D34); until then the
// message text alone still tells the user what is wrong.
func dictionaryUnavailable(dt dictionary.Type, category search.OpenCategory) string {
	name := DictionaryLabel(dt)
	switch category {
	case search.OpenMissing:
		return fmt.Sprintf("The %s database was not found.", name)
	case search.OpenUnreadable:
		return fmt.Sprintf("The %s database could not be opened. Check its configured path and file permissions.", name)
	case search.OpenCorrupt:
		return fmt.Sprintf("The %s database is damaged or is not a valid EJQuick database.", name)
	case search.OpenIncompatible:
		return fmt.Sprintf("The %s database uses an incompatible format.", name)
	case search.OpenWrongDictionary:
		return fmt.Sprintf("The configured %s database contains the other dictionary type.", name)
	default:
		return fmt.Sprintf("The %s database is unavailable.", name)
	}
}
