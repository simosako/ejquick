package gui

import (
	"path/filepath"
	"testing"
)

func TestNearestExistingParent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing", "nested", "config.toml")
	got, ok := nearestExistingParent(path)
	if !ok || got != dir {
		t.Fatalf("nearestExistingParent = %q, %t; want %q, true", got, ok, dir)
	}
	if got, ok := nearestExistingParent(""); ok || got != "" {
		t.Fatalf("nearestExistingParent(empty) = %q, %t", got, ok)
	}
}
