package builder_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/simosako/ejquick/internal/builder"
	"github.com/simosako/ejquick/internal/dictionary"
)

func TestRunContextReportsTypedEvents(t *testing.T) {
	dir := t.TempDir()
	input := writeFixture(t, dir, "EIJIRO1-0.TXT", encodeCP932Lines(t, []string{
		"alpha : first letter",
	}))
	output := filepath.Join(dir, "eiwa.sqlite3")

	var events []builder.Event
	stats, err := builder.RunContext(context.Background(), builder.Options{
		Type: dictionary.Eiwa, Input: input, Output: output,
	}, func(event builder.Event) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("RunContext: %v", err)
	}
	if len(events) == 0 || events[0].Kind != builder.EventTemporaryCreated {
		t.Fatalf("first event = %+v, want temporary_created", events)
	}
	last := events[len(events)-1]
	if last.Kind != builder.EventCompleted || last.Stats.Entries != stats.Entries {
		t.Fatalf("last event = %+v, want completed stats", last)
	}
	canonicalDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("resolve test directory: %v", err)
	}
	for _, event := range events {
		if event.Kind == builder.EventTemporaryCreated {
			if filepath.Dir(event.TemporaryPath) != canonicalDir {
				t.Errorf("temporary directory = %q, want %q", filepath.Dir(event.TemporaryPath), canonicalDir)
			}
			if _, err := os.Stat(event.TemporaryPath); !os.IsNotExist(err) {
				t.Errorf("temporary path remains after success: %v", err)
			}
		}
	}
}

func TestRunContextCancellationCleansTemporaryDatabase(t *testing.T) {
	dir := t.TempDir()
	input := writeFixture(t, dir, "EIJIRO1-0.TXT", encodeCP932Lines(t, []string{
		"alpha : first letter",
	}))
	output := filepath.Join(dir, "eiwa.sqlite3")
	ctx, cancel := context.WithCancel(context.Background())
	var temporary string

	_, err := builder.RunContext(ctx, builder.Options{
		Type: dictionary.Eiwa, Input: input, Output: output,
	}, func(event builder.Event) error {
		if event.Kind == builder.EventTemporaryCreated {
			temporary = event.TemporaryPath
		}
		if event.Kind == builder.EventPhase && event.Phase == builder.PhaseReading && event.State == builder.PhaseStarted {
			cancel()
		}
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunContext error = %v, want context.Canceled", err)
	}
	if temporary == "" {
		t.Fatal("temporary path was not reported")
	}
	if _, err := os.Stat(temporary); !os.IsNotExist(err) {
		t.Errorf("temporary path remains after cancellation: %v", err)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Errorf("output exists after cancellation: %v", err)
	}
}

func TestBuildKeepsReusableOutputLockFile(t *testing.T) {
	dir := t.TempDir()
	input := writeFixture(t, dir, "EIJIRO1-0.TXT", encodeCP932Lines(t, []string{
		"alpha : first letter",
	}))
	output := filepath.Join(dir, "eiwa.sqlite3")

	for i := 0; i < 2; i++ {
		_, err := builder.Run(builder.Options{
			Type: dictionary.Eiwa, Input: input, Output: output, Force: i > 0,
		})
		if err != nil {
			t.Fatalf("build %d: %v", i+1, err)
		}
	}
	lockPath := filepath.Join(dir, ".eiwa.sqlite3.ejquick-build.lock")
	info, err := os.Lstat(lockPath)
	if err != nil {
		t.Fatalf("stat lock file: %v", err)
	}
	if !info.Mode().IsRegular() || info.Size() != 1 {
		t.Errorf("lock info = mode %v size %d", info.Mode(), info.Size())
	}
}
