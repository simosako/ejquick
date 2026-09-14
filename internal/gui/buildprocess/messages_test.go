package buildprocess

import "testing"

func TestMessage(t *testing.T) {
	tests := []struct {
		category Category
		want     string
	}{
		{CategoryBinaryMissing, "The dictionary builder could not be started."},
		{CategoryVersionMismatch, "The dictionary builder version does not match this application."},
		{CategoryProtocolError, "The dictionary builder sent an invalid response."},
		{CategoryOutputBusy, "Another dictionary database build is already running."},
		{CategorySourceInvalid, "The selected source file cannot be used."},
		{CategoryBuildFailed, "The dictionary database could not be built."},
		{CategoryProcessDied, "The dictionary builder stopped unexpectedly."},
		{CategoryUnknown, "The dictionary database could not be built."},
		{Category("future_category"), "The dictionary database could not be built."},
	}
	for _, tt := range tests {
		t.Run(string(tt.category), func(t *testing.T) {
			if got := Message(tt.category); got != tt.want {
				t.Errorf("Message(%q) = %q, want %q", tt.category, got, tt.want)
			}
		})
	}
}
