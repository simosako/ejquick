package gui_test

import (
	"strings"
	"testing"

	"github.com/simosako/ejquick/internal/gui"
)

func TestParseDefault(t *testing.T) {
	opts, action, err := gui.Parse(nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if action != gui.ActionRun {
		t.Errorf("action = %v, want ActionRun", action)
	}
	if opts == nil {
		t.Fatal("opts is nil")
	}
	if opts.ConfigPath != "" || opts.ConfigPathSet || opts.Debug {
		t.Errorf("opts = %+v, want zero value", *opts)
	}
}

func TestParseConfigAndDebug(t *testing.T) {
	opts, action, err := gui.Parse([]string{"--debug", "-c", "/tmp/config.toml"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if action != gui.ActionRun {
		t.Errorf("action = %v, want ActionRun", action)
	}
	if !opts.Debug {
		t.Error("Debug = false, want true")
	}
	if !opts.ConfigPathSet || opts.ConfigPath != "/tmp/config.toml" {
		t.Errorf("config = %q set=%v, want /tmp/config.toml set=true", opts.ConfigPath, opts.ConfigPathSet)
	}
}

func TestParseConfigLastWins(t *testing.T) {
	opts, _, err := gui.Parse([]string{"--config", "a.toml", "--config", "b.toml"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if opts.ConfigPath != "b.toml" {
		t.Errorf("config = %q, want b.toml", opts.ConfigPath)
	}
}

func TestParseHelpAndVersionWinImmediately(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want gui.Action
	}{
		{"help long", []string{"--help"}, gui.ActionHelp},
		{"help short", []string{"-h"}, gui.ActionHelp},
		{"version long", []string{"--version"}, gui.ActionVersion},
		{"version short", []string{"-v"}, gui.ActionVersion},
		{"first wins help", []string{"--debug", "--version", "--help"}, gui.ActionVersion},
		{"first wins version", []string{"-h", "-v"}, gui.ActionHelp},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, action, err := gui.Parse(tt.args)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if action != tt.want {
				t.Errorf("action = %v, want %v", action, tt.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"unknown long option", []string{"--limit", "10"}},
		{"unknown short option", []string{"-x"}},
		{"positional query", []string{"hello"}},
		{"positional after --", []string{"--", "hello"}},
		{"missing config value", []string{"--config"}},
		{"config short missing value", []string{"-c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := gui.Parse(tt.args)
			if err == nil {
				t.Fatalf("parse(%q) succeeded, want error", tt.args)
			}
		})
	}
}

func TestParseBareSeparator(t *testing.T) {
	opts, action, err := gui.Parse([]string{"--"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if action != gui.ActionRun {
		t.Errorf("action = %v, want ActionRun", action)
	}
	if opts == nil {
		t.Fatal("opts is nil")
	}
}

func TestUsageDocumentsAllOptions(t *testing.T) {
	for _, want := range []string{"--config", "--debug", "--help", "--version", "ejquick-gui"} {
		if !strings.Contains(gui.Usage, want) {
			t.Errorf("usage missing %q", want)
		}
	}
}
