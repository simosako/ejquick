package builder

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/simosako/ejquick/internal/dictionary"
	"github.com/simosako/ejquick/internal/sqlite"
)

func TestInsertEntriesReportsProgressInterval(t *testing.T) {
	db, err := sqlite.OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(entriesSchema); err != nil {
		t.Fatal(err)
	}

	input := strings.NewReader(strings.Repeat("\x81\xa1entry : artificial body\r\n", progressEvery+1))
	var progress bytes.Buffer
	var stats Stats
	if err := insertEntries(db, input, Options{Type: dictionary.Eiji, Progress: &progress}, &stats); err != nil {
		t.Fatal(err)
	}
	want := "Reading: start\n" +
		"Reading: lines=100000 entries=100000 skipped=0\n" +
		"Reading: done lines=100001 entries=100001 skipped=0\n"
	if progress.String() != want {
		t.Errorf("progress = %q, want %q", progress.String(), want)
	}
}

func TestRunPublishFailurePreservesExistingOutput(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "EIJIRO1-0.TXT")
	output := filepath.Join(dir, "eiji.sqlite3")
	if err := os.WriteFile(input, []byte("\x81\xa1alpha : artificial body\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, []byte("existing database"), 0o644); err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("injected publish failure")
	publishCalled := false
	_, err := run(Options{
		Type: dictionary.Eiji, Input: input, Output: output, Force: true, Progress: io.Discard,
	}, func(tmpPath, gotOutput string) error {
		publishCalled = true
		if gotOutput != output {
			t.Errorf("publish output = %q, want %q", gotOutput, output)
		}
		if _, err := os.Stat(tmpPath); err != nil {
			t.Errorf("temporary database unavailable at publish: %v", err)
		}
		return wantErr
	})
	if !publishCalled {
		t.Fatal("publish was not called")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run error = %v, want %v", err, wantErr)
	}
	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "existing database" {
		t.Errorf("existing output was changed to %q", got)
	}
	assertNoBuildTemps(t, dir)
}

func TestPhaseReportsFailure(t *testing.T) {
	var progress bytes.Buffer
	wantErr := errors.New("injected failure")
	if err := phase(&progress, "Test phase", func() error { return wantErr }); !errors.Is(err, wantErr) {
		t.Fatalf("phase error = %v, want %v", err, wantErr)
	}
	want := "Test phase: start\nTest phase: failed: injected failure\n"
	if progress.String() != want {
		t.Errorf("progress = %q, want %q", progress.String(), want)
	}
}

func assertNoBuildTemps(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".ejquick-build-") {
			t.Errorf("temporary database left behind: %s", entry.Name())
		}
	}
}
