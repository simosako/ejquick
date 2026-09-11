package dictionary_test

import (
	"testing"

	"github.com/simosako/ejquick/internal/dictionary"
)

func TestParseType(t *testing.T) {
	for _, s := range []string{"eiji", "waei"} {
		got, err := dictionary.ParseType(s)
		if err != nil {
			t.Errorf("ParseType(%q) unexpected error: %v", s, err)
			continue
		}
		if got != dictionary.Type(s) {
			t.Errorf("ParseType(%q) = %q", s, got)
		}
	}
	for _, s := range []string{"", "Eiji", "EIJI", "jp", "en"} {
		if _, err := dictionary.ParseType(s); err == nil {
			t.Errorf("ParseType(%q) expected error", s)
		}
	}
}

func TestLabels(t *testing.T) {
	if dictionary.Eiji.Label() != "EIJI" {
		t.Errorf("eiji label = %q", dictionary.Eiji.Label())
	}
	if dictionary.Waei.Label() != "WAEI" {
		t.Errorf("waei label = %q", dictionary.Waei.Label())
	}
}

func TestOther(t *testing.T) {
	if dictionary.Eiji.Other() != dictionary.Waei {
		t.Errorf("eiji other = %q", dictionary.Eiji.Other())
	}
	if dictionary.Waei.Other() != dictionary.Eiji {
		t.Errorf("waei other = %q", dictionary.Waei.Other())
	}
}
