package main

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestRunCreatesRootedArchive(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "ejquick_v1.0.0_windows_amd64")
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"ejquick.exe":    "binary",
		"docs/README.md": "documentation",
	}
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	outPath := filepath.Join(t.TempDir(), "release.zip")
	if err := run(outPath, dir+string(filepath.Separator)); err != nil {
		t.Fatalf("run: %v", err)
	}

	zr, err := zip.OpenReader(outPath)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer zr.Close()

	got := make(map[string]string)
	for _, file := range zr.File {
		r, err := file.Open()
		if err != nil {
			t.Fatalf("open %s: %v", file.Name, err)
		}
		data, readErr := io.ReadAll(r)
		closeErr := r.Close()
		if readErr != nil {
			t.Fatalf("read %s: %v", file.Name, readErr)
		}
		if closeErr != nil {
			t.Fatalf("close %s: %v", file.Name, closeErr)
		}
		got[file.Name] = string(data)
	}

	root := filepath.Base(dir)
	if len(got) != len(files) {
		t.Fatalf("archive entries = %v, want %d files", got, len(files))
	}
	for name, want := range files {
		archiveName := filepath.ToSlash(filepath.Join(root, name))
		if got[archiveName] != want {
			t.Errorf("entry %q = %q, want %q", archiveName, got[archiveName], want)
		}
	}
}

func TestRunRejectsFileInput(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input")
	if err := os.WriteFile(input, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "release.zip")

	if err := run(outPath, input); err == nil {
		t.Fatal("run unexpectedly accepted a file input")
	}
	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Fatalf("output exists after failure: %v", err)
	}
}

func TestRunRemovesPartialArchive(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "release")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a-valid"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("missing", filepath.Join(dir, "z-broken")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	outPath := filepath.Join(t.TempDir(), "release.zip")

	if err := run(outPath, dir); err == nil {
		t.Fatal("run unexpectedly succeeded with a dangling symlink")
	}
	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Fatalf("partial output exists after failure: %v", err)
	}
}
