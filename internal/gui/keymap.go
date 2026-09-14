// Keymap is the canonical source of the fixed input-centric keyboard
// shortcuts (GUI design D13). The GUI shows these sequences in the empty
// query guidance (D15), the Keyboard Shortcuts help dialog, and the menu
// (D19) from this single table, so the displayed keys can never drift
// from the implemented ones. Text is canonical English (design D41).
package gui

// Key actions addressable by keyboard. Navigation handled inside the
// search field's key press handling is listed here for documentation and
// guidance; the rest are QAction shortcuts.
type KeyAction int

const (
	// KeyPrevResult selects the previous search result.
	KeyPrevResult KeyAction = iota
	// KeyNextResult selects the next search result.
	KeyNextResult
	// KeyPageUp scrolls the detail page one page up.
	KeyPageUp
	// KeyPageDown scrolls the detail page one page down.
	KeyPageDown
	// KeyFocusSearch focuses the search field and selects the query.
	KeyFocusSearch
	// KeyClearSearch clears the query and all search state.
	KeyClearSearch
	// KeySwitchDictionary switches to the other available dictionary.
	KeySwitchDictionary
	// KeyCopyEntry copies the selected entry to the clipboard.
	KeyCopyEntry
	// KeyQuit quits the application.
	KeyQuit
)

// keyActionCount bounds valid KeyAction values.
const keyActionCount = 9

// KeyDef describes one action of the fixed keymap.
type KeyDef struct {
	// Action identifies the entry.
	Action KeyAction
	// Label is the canonical action text shown in help.
	Label string
	// MenuText is the menu display text with a Qt mnemonic (design D41).
	MenuText string
	// Keys is the platform-neutral key display, e.g. "Up / Ctrl+P".
	Keys string
}

// Keymap lists the fixed keymap in help display order. Menu texts, the
// shortcut help, and the empty-query guidance all derive from this one
// table (design D13/D19).
var Keymap = []KeyDef{
	{KeyPrevResult, "Select previous result", "Select &Previous Result", "Up / Ctrl+P"},
	{KeyNextResult, "Select next result", "Select &Next Result", "Down / Ctrl+N"},
	{KeyPageUp, "Scroll detail up one page", "Scroll Detail &Up One Page", "PageUp"},
	{KeyPageDown, "Scroll detail down one page", "Scroll Detail &Down One Page", "PageDown"},
	{KeyFocusSearch, "Focus Search", "&Focus Search", "Ctrl+L"},
	{KeyClearSearch, "Clear Search", "C&lear Search", "Escape"},
	{KeySwitchDictionary, "Switch dictionary", "&Switch Dictionary", "Ctrl+Tab"},
	{KeyCopyEntry, "Copy Entire Entry", "Copy &Entire Entry", "Ctrl+Shift+C"},
	{KeyQuit, "Quit", "&Quit", "Ctrl+Q"},
}

// keyDef returns the KeyDef for an action.
func keyDef(a KeyAction) KeyDef {
	for _, d := range Keymap {
		if d.Action == a {
			return d
		}
	}
	return KeyDef{}
}

// EmptyQueryGuidance is the compact operation hint shown in the message
// page while the query is empty (design D15): one first line plus one
// line with the guidance-relevant keys. The text mirrors the design
// example; the switch-dictionary key comes from the keymap table so it
// cannot drift from the implemented shortcut.
func EmptyQueryGuidance() []string {
	return []string{
		"Enter a search term",
		"Up/Down: Select result  " +
			keyDef(KeySwitchDictionary).Keys + ": Switch dictionary",
	}
}
