package controller

import (
	"strings"
	"testing"

	"github.com/simosako/ejquick/internal/dictionary"
	"github.com/simosako/ejquick/internal/search"
)

func TestUnavailableMessages(t *testing.T) {
	tests := []struct {
		category search.OpenCategory
		text     string
		build    bool
		openLog  bool
	}{
		{search.OpenMissing, "was not found", true, false},
		{search.OpenUnreadable, "could not be opened", false, true},
		{search.OpenCorrupt, "is damaged", true, true},
		{search.OpenIncompatible, "incompatible format", true, true},
		{search.OpenWrongDictionary, "other dictionary type", true, true},
		{search.OpenUnknown, "is unavailable", false, true},
	}
	for _, test := range tests {
		t.Run(string(test.category), func(t *testing.T) {
			message := MessageForUnavailable(dictionary.Eiwa, test.category)
			if !strings.Contains(message.Text, "English–Japanese") || !strings.Contains(message.Text, test.text) {
				t.Errorf("text = %q", message.Text)
			}
			if message.Build != test.build || message.OpenLog != test.openLog {
				t.Errorf("actions = build:%t log:%t", message.Build, message.OpenLog)
			}
			if strings.Contains(message.Text, "/private/") || strings.Contains(message.Text, "SQLite") {
				t.Errorf("message exposes technical detail: %q", message.Text)
			}
		})
	}
}
