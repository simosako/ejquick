package gui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/simosako/ejquick/internal/builder"
	"github.com/simosako/ejquick/internal/config"
	"github.com/simosako/ejquick/internal/dictionary"
	"github.com/simosako/ejquick/internal/logging"
	"github.com/simosako/ejquick/internal/search"
)

// buildStartupDB builds a tiny dictionary database through the real
// builder so open classification tests exercise the production path.
func buildStartupDB(t *testing.T, dir string, dt dictionary.Type) string {
	t.Helper()
	input := filepath.Join(dir, dt.String()+".TXT")
	if err := os.WriteFile(input, []byte("\x81\xa1word : meaning\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, dt.String()+".sqlite3")
	if _, err := builder.Run(builder.Options{
		Type:     dt,
		Input:    input,
		Output:   output,
		Progress: nil,
	}); err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	return output
}

func startupConfig(t *testing.T, eiwaPath, waeiPath string, def dictionary.Type) *config.Config {
	t.Helper()
	cfg := config.Defaults()
	cfg.Search.DefaultDictionary = def.String()
	if eiwaPath != "" {
		cfg.Eiwa.Database = eiwaPath
	}
	if waeiPath != "" {
		cfg.Waei.Database = waeiPath
	}
	return cfg
}

func statusOf(t *testing.T, statuses map[dictionary.Type]DictStatus, dt dictionary.Type) DictStatus {
	t.Helper()
	st, ok := statuses[dt]
	if !ok {
		t.Fatalf("no status recorded for %v", dt)
	}
	return st
}

func TestOpenDictionariesBothAvailable(t *testing.T) {
	dir := t.TempDir()
	eiwa := buildStartupDB(t, dir, dictionary.Eiwa)
	waei := buildStartupDB(t, dir, dictionary.Waei)

	res := openDictionaries(startupConfig(t, eiwa, waei, dictionary.Waei), logging.Nop())
	if len(res.Services) != 2 {
		t.Fatalf("services = %v, want both", res.Services)
	}
	if !statusOf(t, res.Statuses, dictionary.Eiwa).Available ||
		!statusOf(t, res.Statuses, dictionary.Waei).Available {
		t.Errorf("statuses = %+v, want both available", res.Statuses)
	}
	// The configured default is selected when available.
	if res.Initial != dictionary.Waei {
		t.Errorf("initial = %v, want waei (the configured default)", res.Initial)
	}
}

func TestOpenDictionariesDefaultFallsBack(t *testing.T) {
	dir := t.TempDir()
	waei := buildStartupDB(t, dir, dictionary.Waei)
	missing := filepath.Join(dir, "eiwa.sqlite3")

	res := openDictionaries(startupConfig(t, missing, waei, dictionary.Eiwa), logging.Nop())
	if statusOf(t, res.Statuses, dictionary.Eiwa).Available {
		t.Error("missing eiwa database reported available")
	}
	if got := statusOf(t, res.Statuses, dictionary.Eiwa).Category; got != search.OpenMissing {
		t.Errorf("eiwa category = %q, want missing", got)
	}
	if !statusOf(t, res.Statuses, dictionary.Waei).Available {
		t.Error("waei unavailable")
	}
	// Fallback rule: first available dictionary (design D39).
	if res.Initial != dictionary.Waei {
		t.Errorf("initial = %v, want waei fallback", res.Initial)
	}
}

func TestOpenDictionariesClassifiesFailures(t *testing.T) {
	dir := t.TempDir()
	eiwa := buildStartupDB(t, dir, dictionary.Eiwa)

	// waei DB is actually an eiwa database: wrong_dictionary.
	res := openDictionaries(startupConfig(t, eiwa, eiwa, dictionary.Eiwa), logging.Nop())
	if got := statusOf(t, res.Statuses, dictionary.Waei).Category; got != search.OpenWrongDictionary {
		t.Errorf("waei category = %q, want wrong_dictionary", got)
	}
	if !statusOf(t, res.Statuses, dictionary.Eiwa).Available {
		t.Error("eiwa should stay available")
	}

	// A garbage file classifies as corrupt.
	garbage := filepath.Join(dir, "garbage.sqlite3")
	os.WriteFile(garbage, []byte("not a database at all, really not one"), 0o644)
	res = openDictionaries(startupConfig(t, garbage, "", dictionary.Eiwa), logging.Nop())
	if got := statusOf(t, res.Statuses, dictionary.Eiwa).Category; got != search.OpenCorrupt {
		t.Errorf("eiwa category = %q, want corrupt", got)
	}
}

func TestOpenDictionariesNoneAvailable(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "none.sqlite3")
	res := openDictionaries(startupConfig(t, missing, missing, dictionary.Eiwa), logging.Nop())
	if len(res.Services) != 0 {
		t.Fatalf("services = %v, want none", res.Services)
	}
	// The default is kept so the message page can explain it first
	// (design D39/D45).
	if res.Initial != dictionary.Eiwa {
		t.Errorf("initial = %v, want the configured default", res.Initial)
	}
}

// TestControllerWithRealService runs the controller against a real
// search.Service over a builder-produced database (design D35: widget
// tests may use real SQLite with small artificial sources).
func TestControllerWithRealService(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "in.TXT")
	body := ""
	for _, l := range []string{"care : attention", "career : profession", "scar : mark"} {
		body += "\x81\xa1" + l + "\r\n"
	}
	if err := os.WriteFile(input, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	db := filepath.Join(dir, "eiwa.sqlite3")
	if _, err := builder.Run(builder.Options{
		Type: dictionary.Eiwa, Input: input, Output: db, Progress: nil,
	}); err != nil {
		t.Fatalf("build fixture: %v", err)
	}

	res := openDictionaries(startupConfig(t, db, "", dictionary.Eiwa), logging.Nop())
	if !statusOf(t, res.Statuses, dictionary.Eiwa).Available {
		t.Fatal("fixture database did not open")
	}
	c := newTestController(t, NewController(ControllerConfig{
		Services: res.Services, Statuses: res.Statuses,
		Initial: res.Initial, MaxResults: 50, Logger: logging.Nop(),
	}))

	c.SetQuery("care")
	v := c.View()
	if v.Page != pageDetail || len(v.Headwords) != 2 {
		t.Fatalf("view = page %d headwords %v, want detail page with 2 rows", v.Page, v.Headwords)
	}
	if v.Entry.Headword != "care" || v.Entry.Body != "attention" {
		t.Errorf("selected entry = %+v", v.Entry)
	}
}
