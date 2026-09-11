package logging_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/simosako/ejquick/internal/logging"
)

func TestOpenFileAndAppend(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "ejquick.log")

	log, err := logging.OpenFile(path, false)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	log.Error("boom: %s", "detail")
	log.Debug("hidden") // debug disabled
	if err := log.Close(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, " ERROR boom: detail") {
		t.Errorf("missing error line: %q", got)
	}
	if strings.Contains(got, "hidden") {
		t.Errorf("debug line written while disabled: %q", got)
	}
	if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`).MatchString(got) {
		t.Errorf("missing timestamp: %q", got)
	}

	// Appends, does not truncate.
	log2, err := logging.OpenFile(path, true)
	if err != nil {
		t.Fatal(err)
	}
	log2.Error("second")
	log2.Debug("visible now")
	log2.Close()
	data, _ = os.ReadFile(path)
	if !strings.Contains(string(data), "second") {
		t.Error("append failed")
	}
	if !strings.Contains(string(data), " DEBUG visible now") {
		t.Errorf("debug line missing while enabled: %q", string(data))
	}
}

func TestDefaultPathShape(t *testing.T) {
	p, err := logging.DefaultPath()
	if err != nil {
		t.Skipf("cannot determine default log path: %v", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(p), "ejquick/ejquick.log") {
		t.Errorf("unexpected default log path: %q", p)
	}
}

func TestNopIsSafe(t *testing.T) {
	var l *logging.Logger // nil must also be safe
	l.Error("x")
	l.Debug("y")
	if l.DebugEnabled() {
		t.Error("nil logger reports debug enabled")
	}
	logging.Nop().Error("z")
}
