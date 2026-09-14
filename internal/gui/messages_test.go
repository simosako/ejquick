package gui

import (
	"strings"
	"testing"

	"github.com/simosako/ejquick/internal/dictionary"
	"github.com/simosako/ejquick/internal/search"
)

func TestDictionaryLabels(t *testing.T) {
	tests := map[dictionary.Type]string{
		dictionary.Eiwa: "English–Japanese",
		dictionary.Waei: "Japanese–English",
	}
	for dt, want := range tests {
		if got := DictionaryLabel(dt); got != want {
			t.Errorf("DictionaryLabel(%v) = %q, want %q", dt, got, want)
		}
		// The en dash is deliberate (design D41); an ASCII hyphen would
		// be a silent spec violation.
		if !strings.ContainsRune(DictionaryLabel(dt), '–') {
			t.Errorf("DictionaryLabel(%v) missing en dash", dt)
		}
	}
}

func TestDictionaryUnavailableMessages(t *testing.T) {
	tests := []struct {
		category search.OpenCategory
		contains string
	}{
		{search.OpenMissing, "was not found"},
		{search.OpenUnreadable, "could not be opened"},
		{search.OpenCorrupt, "damaged or is not a valid EJQuick database"},
		{search.OpenIncompatible, "incompatible format"},
		{search.OpenWrongDictionary, "contains the other dictionary type"},
		{search.OpenUnknown, "unavailable"},
		{"future_category", "unavailable"}, // unknown values fall back
	}
	for _, tt := range tests {
		got := dictionaryUnavailable(dictionary.Waei, tt.category)
		if !strings.Contains(got, tt.contains) {
			t.Errorf("message(%v) = %q, missing %q", tt.category, got, tt.contains)
		}
		if !strings.Contains(got, "Japanese–English") {
			t.Errorf("message(%v) = %q does not name the dictionary", tt.category, got)
		}
		// Raw errors and paths never reach the message (design D44).
		for _, banned := range []string{".sqlite3", "SQLITE", "no such table"} {
			if strings.Contains(got, banned) {
				t.Errorf("message(%v) = %q leaks %q", tt.category, got, banned)
			}
		}
	}
}

func TestCountText(t *testing.T) {
	tests := []struct {
		shown, max int
		want       string
	}{
		{0, 50, ""},
		{1, 50, "1 shown"},
		{49, 50, "49 shown"},
		{50, 50, "Showing first 50"},
		{500, 500, "Showing first 500"},
		{-1, 50, ""},
	}
	for _, tt := range tests {
		if got := CountText(tt.shown, tt.max); got != tt.want {
			t.Errorf("CountText(%d, %d) = %q, want %q", tt.shown, tt.max, got, tt.want)
		}
	}
}

func TestSearchErrorAndNoResultsText(t *testing.T) {
	if got := SearchErrorText(); strings.Join(got, "|") != "Search failed|Edit the query to try again" {
		t.Errorf("SearchErrorText = %q", got)
	}
	if got := NoResultsText(); strings.Join(got, "|") != "No matching headwords" {
		t.Errorf("NoResultsText = %q", got)
	}
}
