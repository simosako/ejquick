package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunHelpAndVersion(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"--version"}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 0 {
			t.Errorf("run(%v) exit code = %d, want 0", args, code)
		}
		if stdout.Len() == 0 {
			t.Errorf("run(%v) produced no stdout", args)
		}
		if stderr.Len() != 0 {
			t.Errorf("run(%v) stderr = %q", args, stderr.String())
		}
	}
}

func TestRunRejectsInvalidArguments(t *testing.T) {
	tests := [][]string{
		nil,
		{"--type", "bad", "--input", "in", "--output", "out"},
		{"--type", "eiji", "--output", "out"},
		{"--type", "eiji", "--input", "in"},
		{"--type", "eiji", "--input", "in", "--output", "out", "extra"},
	}
	for _, args := range tests {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 1 {
			t.Errorf("run(%v) exit code = %d, want 1", args, code)
		}
		if stdout.Len() != 0 {
			t.Errorf("run(%v) stdout = %q, want empty", args, stdout.String())
		}
		if !strings.Contains(stderr.String(), "Usage: ejquick-build") {
			t.Errorf("run(%v) stderr = %q", args, stderr.String())
		}
	}
}

func TestRunBuildWritesProgressOnlyToStderr(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "EIJIRO1-0.TXT")
	output := filepath.Join(dir, "eiji.sqlite3")
	if err := os.WriteFile(input, []byte("alpha : first body\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--type", "eiji",
		"--input", input,
		"--output", output,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	for _, line := range []string{"Reading: start", "Validation: done", "Entries: 1"} {
		if !strings.Contains(stderr.String(), line) {
			t.Errorf("stderr does not contain %q: %q", line, stderr.String())
		}
	}
	if _, err := os.Stat(output); err != nil {
		t.Errorf("stat output: %v", err)
	}
}

func TestRunBuildFailureReturnsTwo(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.TXT")
	output := filepath.Join(dir, "eiji.sqlite3")
	if err := os.WriteFile(input, []byte("alpha : first body\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--type", "eiji", "--input", input, "--output", output}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "already exists") {
		t.Errorf("stderr = %q", stderr.String())
	}
}
