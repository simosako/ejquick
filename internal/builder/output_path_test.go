package builder

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestOutputLockIsNonBlocking(t *testing.T) {
	output := filepath.Join(t.TempDir(), "eiwa.sqlite3")
	_, first, err := prepareOutput(output)
	if err != nil {
		t.Fatalf("first lock: %v", err)
	}
	defer first.close()

	_, second, err := prepareOutput(output)
	if second != nil {
		second.close()
		t.Fatal("second lock unexpectedly succeeded")
	}
	if CodeOf(err) != ErrorOutputBusy || !errors.Is(err, errOutputBusy) {
		t.Fatalf("second lock error = %v, code %q", err, CodeOf(err))
	}
}
