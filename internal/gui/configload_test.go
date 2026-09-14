package gui

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/simosako/ejquick/internal/config"
)

func TestLoadConfigExplicitMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.toml")
	_, err := LoadConfig(path, true)
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("error %v is not *ConfigError", err)
	}
	if cfgErr.Category() != ConfigMissing {
		t.Errorf("category = %v, want missing", cfgErr.Category())
	}
	if cfgErr.Path() != path {
		t.Errorf("path = %q, want %q", cfgErr.Path(), path)
	}
}

func TestLoadConfigExplicitInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("[search]\nmax_results = 999\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(path, true)
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("error %v is not *ConfigError", err)
	}
	if cfgErr.Category() != ConfigInvalid {
		t.Errorf("category = %v, want invalid", cfgErr.Category())
	}
}

func TestLoadConfigExplicitUnreadable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits")
	}
	if os.Getuid() == 0 {
		t.Skip("running as root ignores permission bits")
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[search]\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(path, true)
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("error %v is not *ConfigError", err)
	}
	if cfgErr.Category() != ConfigUnreadable {
		t.Errorf("category = %v, want unreadable", cfgErr.Category())
	}
}

func TestLoadConfigDefaultMissingMeansDefaults(t *testing.T) {
	dir := t.TempDir()
	defaultConfigPath = func() (string, error) {
		return filepath.Join(dir, "ejquick", "config.toml"), nil
	}
	t.Cleanup(func() { defaultConfigPath = config.DefaultPath })

	cfg, err := LoadConfig("", false)
	if err != nil {
		t.Fatalf("missing default config errored: %v", err)
	}
	if cfg == nil || cfg.Search.MaxResults != config.Defaults().Search.MaxResults {
		t.Errorf("cfg = %+v, want defaults", cfg)
	}
}

func TestLoadConfigDefaultInvalid(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "ejquick"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ejquick", "config.toml"),
		[]byte("bogus_key = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	defaultConfigPath = func() (string, error) {
		return filepath.Join(dir, "ejquick", "config.toml"), nil
	}
	t.Cleanup(func() { defaultConfigPath = config.DefaultPath })

	_, err := LoadConfig("", false)
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("error %v is not *ConfigError", err)
	}
	if cfgErr.Category() != ConfigInvalid {
		t.Errorf("category = %v, want invalid", cfgErr.Category())
	}
}

func TestLoadConfigDefaultPathUnavailable(t *testing.T) {
	defaultConfigPath = func() (string, error) {
		return "", errors.New("no home directory")
	}
	t.Cleanup(func() { defaultConfigPath = config.DefaultPath })

	_, err := LoadConfig("", false)
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("error %v is not *ConfigError", err)
	}
	if cfgErr.Category() != ConfigPathUnavailable {
		t.Errorf("category = %v, want path unavailable", cfgErr.Category())
	}
	if cfgErr.Path() != "" {
		t.Errorf("path = %q, want empty", cfgErr.Path())
	}
}

func TestConfigMessages(t *testing.T) {
	tests := map[ConfigCategory]string{
		ConfigPathUnavailable: "The configuration file location could not be determined.",
		ConfigMissing:         "The configuration file was not found.",
		ConfigUnreadable:      "The configuration file could not be read.",
		ConfigInvalid:         "The configuration file is invalid.",
		ConfigUnknown:         "The configuration could not be loaded.",
		ConfigCategory(99):    "The configuration could not be loaded.",
	}
	for category, want := range tests {
		if got := ConfigMessage(category); got != want {
			t.Errorf("ConfigMessage(%v) = %q, want %q", category, got, want)
		}
	}
}

func TestConfigOpenChoice(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(existing, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "missing", "config.toml")

	if got := ConfigOpenChoice(ConfigMissing, missing); got != ConfigOpenParentFolder {
		t.Errorf("missing -> %v, want parent folder", got)
	}
	if got := ConfigOpenChoice(ConfigUnreadable, existing); got != ConfigOpenFile {
		t.Errorf("unreadable existing -> %v, want file", got)
	}
	if got := ConfigOpenChoice(ConfigUnreadable, missing); got != ConfigOpenParentFolder {
		t.Errorf("unreadable missing -> %v, want parent folder", got)
	}
	if got := ConfigOpenChoice(ConfigInvalid, existing); got != ConfigOpenFile {
		t.Errorf("invalid existing -> %v, want file", got)
	}
	if got := ConfigOpenChoice(ConfigPathUnavailable, ""); got != ConfigOpenNone {
		t.Errorf("path unavailable -> %v, want none", got)
	}
	if got := ConfigOpenChoice(ConfigUnknown, existing); got != ConfigOpenFile {
		t.Errorf("unknown existing -> %v, want file", got)
	}
	if got := ConfigOpenChoice(ConfigUnknown, ""); got != ConfigOpenNone {
		t.Errorf("unknown without path -> %v, want none", got)
	}
}

func TestNearestExistingDir(t *testing.T) {
	dir := t.TempDir()
	deep := filepath.Join(dir, "a", "b", "c", "config.toml")
	if got := NearestExistingDir(deep); got != dir {
		t.Errorf("nearest = %q, want %q", got, dir)
	}
	// A path whose nearest existing ancestor is the filesystem root
	// still resolves (or returns "" on unusual systems); it must not
	// loop forever.
	_ = NearestExistingDir("/definitely/missing/path/config.toml")
}
