package controller

import (
	"fmt"

	"github.com/simosako/ejquick/internal/dictionary"
	"github.com/simosako/ejquick/internal/search"
)

// DictionaryName returns the canonical English GUI label.
func DictionaryName(dict dictionary.Type) string {
	if dict == dictionary.Waei {
		return "Japanese–English"
	}
	return "English–Japanese"
}

// UnavailableMessage describes a database category without exposing a path or
// wrapped driver error.
type UnavailableMessage struct {
	Text    string
	Build   bool
	OpenLog bool
}

// MessageForUnavailable maps the closed repository category set to canonical
// user-facing English text and recovery actions.
func MessageForUnavailable(dict dictionary.Type, category search.OpenCategory) UnavailableMessage {
	name := DictionaryName(dict)
	switch category {
	case search.OpenMissing:
		return UnavailableMessage{
			Text: fmt.Sprintf("The %s database was not found.", name), Build: true,
		}
	case search.OpenUnreadable:
		return UnavailableMessage{
			Text:    fmt.Sprintf("The %s database could not be opened. Check its configured path and file permissions.", name),
			OpenLog: true,
		}
	case search.OpenCorrupt:
		return UnavailableMessage{
			Text:  fmt.Sprintf("The %s database is damaged or is not a valid EJQuick database.", name),
			Build: true, OpenLog: true,
		}
	case search.OpenIncompatible:
		return UnavailableMessage{
			Text:  fmt.Sprintf("The %s database uses an incompatible format.", name),
			Build: true, OpenLog: true,
		}
	case search.OpenWrongDictionary:
		return UnavailableMessage{
			Text:  fmt.Sprintf("The configured %s database contains the other dictionary type.", name),
			Build: true, OpenLog: true,
		}
	default:
		return UnavailableMessage{
			Text: fmt.Sprintf("The %s database is unavailable.", name), OpenLog: true,
		}
	}
}
