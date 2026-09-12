package logging

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"
)

type failingWriter struct {
	writes int
}

func (w *failingWriter) Write([]byte) (int, error) {
	w.writes++
	return 0, errors.New("disk full")
}

func TestWriteErrorWarnsOnceAndDisablesLogging(t *testing.T) {
	failed := &failingWriter{}
	var stderr bytes.Buffer
	log := &Logger{w: failed, stderr: &stderr, debug: true}

	log.Error("first")
	log.Error("second")
	log.Debug("third")

	if failed.writes != 1 {
		t.Errorf("failed writer called %d times, want 1", failed.writes)
	}
	if got := strings.Count(stderr.String(), "\n"); got != 1 {
		t.Errorf("warning lines = %d, want 1: %q", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "logging unavailable") ||
		!strings.Contains(stderr.String(), "disk full") {
		t.Errorf("warning = %q", stderr.String())
	}
}

func TestCloseIsSynchronizedWithWrites(t *testing.T) {
	path := t.TempDir() + "/ejquick.log"
	log, err := OpenFile(path, true)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				log.Debug("concurrent write %d", j)
			}
		}()
	}
	if err := log.Close(); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if err := log.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}
