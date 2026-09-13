package dictionary_test

import (
	"testing"

	"github.com/simosako/ejquick/internal/dictionary"
)

func TestParseType(t *testing.T) {
	for _, s := range []string{"eiwa", "waei"} {
		got, err := dictionary.ParseType(s)
		if err != nil {
			t.Errorf("ParseType(%q) unexpected error: %v", s, err)
			continue
		}
		if got != dictionary.Type(s) {
			t.Errorf("ParseType(%q) = %q", s, got)
		}
	}
	for _, s := range []string{"", "eiji", "Eiji", "EIJI", "Eiwa", "EIWA", "jp", "en"} {
		if _, err := dictionary.ParseType(s); err == nil {
			t.Errorf("ParseType(%q) expected error", s)
		}
	}
}

func TestLabels(t *testing.T) {
	if dictionary.Eiwa.Label() != "EIWA" {
		t.Errorf("eiwa label = %q", dictionary.Eiwa.Label())
	}
	if dictionary.Waei.Label() != "WAEI" {
		t.Errorf("waei label = %q", dictionary.Waei.Label())
	}
}

func TestOther(t *testing.T) {
	if dictionary.Eiwa.Other() != dictionary.Waei {
		t.Errorf("eiwa other = %q", dictionary.Eiwa.Other())
	}
	if dictionary.Waei.Other() != dictionary.Eiwa {
		t.Errorf("waei other = %q", dictionary.Waei.Other())
	}
}
