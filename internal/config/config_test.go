package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/simosako/ejquick/internal/config"
	"github.com/simosako/ejquick/internal/dictionary"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadFullConfig(t *testing.T) {
	home, _ := os.UserHomeDir()
	p := writeConfig(t, `
[eiji]
database = "~/.local/share/ejquick/eiji.sqlite3"

[waei]
database = "/absolute/waei.sqlite3"

[search]
default_dictionary = "waei"
max_results = 100
`)
	cfg, err := config.Load(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Database(dictionary.Eiji) != filepath.Join(home, ".local", "share", "ejquick", "eiji.sqlite3") {
		t.Errorf("eiji db = %q", cfg.Database(dictionary.Eiji))
	}
	if cfg.Database(dictionary.Waei) != "/absolute/waei.sqlite3" {
		t.Errorf("waei db = %q", cfg.Database(dictionary.Waei))
	}
	if cfg.DefaultDict() != dictionary.Waei {
		t.Errorf("default dict = %q", cfg.DefaultDict())
	}
	if cfg.Search.MaxResults != 100 {
		t.Errorf("max results = %d", cfg.Search.MaxResults)
	}
}

func TestLoadDefaultsWhenOmitted(t *testing.T) {
	p := writeConfig(t, "# empty config\n")
	cfg, err := config.Load(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.DefaultDict() != dictionary.Eiji {
		t.Errorf("default dict = %q", cfg.DefaultDict())
	}
	if cfg.Search.MaxResults != 50 {
		t.Errorf("max results = %d", cfg.Search.MaxResults)
	}
	eiji := cfg.Database(dictionary.Eiji)
	if !strings.Contains(filepath.ToSlash(eiji), "ejquick/eiji.sqlite3") {
		t.Errorf("default eiji db path = %q", eiji)
	}
}

func TestLoadRelativePathResolvedAgainstConfigDir(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(p, []byte("[eiji]\ndatabase = \"../data/eiji.sqlite3\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(filepath.Dir(dir), "data", "eiji.sqlite3")
	if cfg.Database(dictionary.Eiji) != want {
		t.Errorf("relative path = %q, want %q", cfg.Database(dictionary.Eiji), want)
	}
}

func TestLoadRejectsBadValues(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"bad dictionary", "[search]\ndefault_dictionary = \"en\"\n"},
		{"max results too large", "[search]\nmax_results = 501\n"},
		{"max results negative", "[search]\nmax_results = -1\n"},
		{"parse error", "[search\nbroken"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := writeConfig(t, tt.body)
			if _, err := config.Load(p); err == nil {
				t.Errorf("Load unexpectedly succeeded for %s", tt.name)
			}
		})
	}
}

func TestLoadMissingFileIsError(t *testing.T) {
	if _, err := config.Load(filepath.Join(t.TempDir(), "missing.toml")); err == nil {
		t.Error("missing config file unexpectedly loaded")
	}
}

func TestDefaultPath(t *testing.T) {
	p, err := config.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(filepath.ToSlash(p), "ejquick/config.toml") {
		t.Errorf("default path = %q", p)
	}
}
