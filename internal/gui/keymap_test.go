package gui

import (
	"strings"
	"testing"
)

func TestKeymapCoversAllActions(t *testing.T) {
	seen := make(map[KeyAction]bool, keyActionCount)
	for _, d := range Keymap {
		if d.Action < 0 || int(d.Action) >= keyActionCount {
			t.Errorf("key action %d out of range", d.Action)
			continue
		}
		if seen[d.Action] {
			t.Errorf("duplicate keymap entry for action %d", d.Action)
		}
		seen[d.Action] = true
		if d.Label == "" || d.MenuText == "" || d.Keys == "" {
			t.Errorf("keymap entry %+v has an empty field", d)
		}
		if !strings.Contains(d.MenuText, "&") {
			t.Errorf("menu text %q has no mnemonic", d.MenuText)
		}
	}
	for a := KeyAction(0); int(a) < keyActionCount; a++ {
		if !seen[a] {
			t.Errorf("keymap missing action %d", a)
		}
	}
}

func TestEmptyQueryGuidanceMentionsKeys(t *testing.T) {
	lines := EmptyQueryGuidance()
	if len(lines) != 2 {
		t.Fatalf("guidance lines = %q", lines)
	}
	if lines[0] != "Enter a search term" {
		t.Errorf("first line = %q", lines[0])
	}
	// The guidance must match the implemented keys (design D13/D15): it
	// is derived from the same table.
	switchKeys := keyDef(KeySwitchDictionary).Keys
	if !strings.Contains(lines[1], switchKeys) {
		t.Errorf("guidance %q missing switch keys %q", lines[1], switchKeys)
	}
	if !strings.Contains(lines[1], "Up/Down") {
		t.Errorf("guidance %q missing Up/Down hint", lines[1])
	}
}

// Menu mnemonics must stay unique within the whole keymap so Qt
// activates the right entry.
func TestMenuTextMnemonicsUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, d := range Keymap {
		m, ok := firstMnemonic(d.MenuText)
		if !ok {
			continue
		}
		key := string(m)
		if seen[key] {
			t.Errorf("duplicate mnemonic %q in %q", m, d.MenuText)
		}
		seen[key] = true
	}
}

func firstMnemonic(s string) (rune, bool) {
	for i, r := range s {
		if r == '&' && i+1 < len(s) {
			for _, next := range s[i+1:] {
				return next, true
			}
		}
	}
	return 0, false
}
