package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/simosako/ejquick/internal/builder"
	"github.com/simosako/ejquick/internal/dictionary"
)

func TestParseArgsTracksQueryPresence(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantQuery   string
		wantPresent bool
	}{
		{name: "TUI", args: nil},
		{name: "empty CLI query", args: []string{""}, wantPresent: true},
		{name: "CLI query", args: []string{"alpha"}, wantQuery: "alpha", wantPresent: true},
		{name: "dash query", args: []string{"--", "-prefix"}, wantQuery: "-prefix", wantPresent: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, query, present, done, err := parseArgs(tt.args, io.Discard)
			if err != nil {
				t.Fatalf("parseArgs: %v", err)
			}
			if opts == nil || done {
				t.Fatalf("opts = %v, done = %v", opts, done)
			}
			if query != tt.wantQuery || present != tt.wantPresent {
				t.Errorf("query = %q, present = %v; want %q, %v", query, present, tt.wantQuery, tt.wantPresent)
			}
		})
	}
}

func TestParseArgsTracksExplicitEmptyConfigPath(t *testing.T) {
	opts, _, _, _, err := parseArgs([]string{"--config", "", "alpha"}, io.Discard)
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if !opts.configPathSet || opts.configPath != "" {
		t.Errorf("configPathSet = %v, configPath = %q", opts.configPathSet, opts.configPath)
	}
}

func TestRunRejectsFormatWithoutQuery(t *testing.T) {
	for _, format := range []string{"plain", "jsonl"} {
		t.Run(format, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run([]string{"--format", format}, strings.NewReader("not a query"), &stdout, &stderr)

			if code != 2 {
				t.Errorf("exit code = %d, want 2", code)
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want empty", stdout.String())
			}
			if !strings.Contains(stderr.String(), "--format requires a query argument") {
				t.Errorf("stderr = %q", stderr.String())
			}
		})
	}
}

func TestRunRejectsInvalidArguments(t *testing.T) {
	tests := [][]string{
		{"one", "two"},
		{"--unknown"},
		{"--dictionary"},
		{"--dictionary", "unknown", "query"},
		{"--limit", "0", "query"},
		{"--limit", "501", "query"},
		{"--format", "xml", "query"},
	}
	for _, args := range tests {
		var stdout, stderr bytes.Buffer
		if code := run(args, panicReader{}, &stdout, &stderr); code != 2 {
			t.Errorf("run(%v) exit code = %d, want 2", args, code)
		}
		if stdout.Len() != 0 {
			t.Errorf("run(%v) stdout = %q, want empty", args, stdout.String())
		}
		if !strings.Contains(stderr.String(), "Usage: ejquick") {
			t.Errorf("run(%v) stderr = %q", args, stderr.String())
		}
	}
}

func TestRunDoesNotDefaultExplicitEmptyConfigPath(t *testing.T) {
	setTestAppDirs(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{"--config", "", "alpha"}, panicReader{}, &stdout, &stderr)

	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "read config") {
		t.Errorf("stderr = %q, want explicit config read error", stderr.String())
	}
}

func TestRunRejectsEmptyNormalizedQueryBeforeOpeningDatabase(t *testing.T) {
	setTestAppDirs(t)
	configPath := writeTestConfig(t, filepath.Join(t.TempDir(), "missing.sqlite3"))

	for _, query := range []string{"", " \t\n", " \u25a0 "} {
		t.Run(strconv.Quote(query), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run([]string{"--config", configPath, query}, strings.NewReader("ignored"), &stdout, &stderr)
			if code != 2 {
				t.Errorf("exit code = %d, want 2", code)
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want empty", stdout.String())
			}
			if !strings.Contains(stderr.String(), "query must not be empty after normalization") {
				t.Errorf("stderr = %q", stderr.String())
			}
			if strings.Contains(stderr.String(), "open database") {
				t.Errorf("database was opened before query validation: %q", stderr.String())
			}
		})
	}
}

func TestRunCLISearchFormatsAndExitCodes(t *testing.T) {
	setTestAppDirs(t)
	dbPath := buildTestDatabase(t)
	configPath := writeTestConfig(t, dbPath)

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantOutput string
	}{
		{
			name:       "plain",
			args:       []string{"--config", configPath, "alpha"},
			wantCode:   0,
			wantOutput: "Alpha\nfirst body\n\n",
		},
		{
			name:       "jsonl",
			args:       []string{"--config", configPath, "--format", "jsonl", "alpha"},
			wantCode:   0,
			wantOutput: "{\"id\":1,\"dictionary\":\"eiji\",\"headword\":\"Alpha\",\"body\":\"first body\"}\n",
		},
		{
			name:     "no results",
			args:     []string{"--config", configPath, "missing"},
			wantCode: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, panicReader{}, &stdout, &stderr)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d; stderr = %q", code, tt.wantCode, stderr.String())
			}
			if stdout.String() != tt.wantOutput {
				t.Errorf("stdout = %q, want %q", stdout.String(), tt.wantOutput)
			}
			if stderr.Len() != 0 {
				t.Errorf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestCLIQueryDoesNotReadProcessStdin(t *testing.T) {
	setTestAppDirs(t)
	dbPath := buildTestDatabase(t)
	configPath := writeTestConfig(t, dbPath)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCommandHelper$", "--", "--config", configPath, "alpha")
	cmd.Env = append(os.Environ(), "EJQUICK_COMMAND_HELPER=1")
	stdin, keepOpen, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	defer keepOpen.Close()
	cmd.Stdin = stdin
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			t.Fatal("CLI blocked reading stdin")
		}
		t.Fatalf("CLI subprocess: %v; stderr = %q", err, stderr.String())
	}
	if stdout.String() != "Alpha\nfirst body\n\n" {
		t.Errorf("stdout = %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}

func TestCommandHelper(t *testing.T) {
	if os.Getenv("EJQUICK_COMMAND_HELPER") != "1" {
		return
	}
	separator := -1
	for i, arg := range os.Args {
		if arg == "--" {
			separator = i
			break
		}
	}
	if separator < 0 {
		os.Exit(99)
	}
	os.Exit(run(os.Args[separator+1:], os.Stdin, os.Stdout, os.Stderr))
}

func TestRunHelpAndVersionDoNotNeedConfig(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("AppData", "")

	for _, args := range [][]string{{"--help"}, {"--version"}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, panicReader{}, &stdout, &stderr); code != 0 {
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

func TestLoadConfigReturnsDefaultPathError(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("AppData", "")

	if _, err := loadConfig("", false); err == nil {
		t.Fatal("loadConfig unexpectedly ignored the default path error")
	}
}

func TestLoadConfigReturnsDefaultPathStatError(t *testing.T) {
	notDirectory := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(notDirectory, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	switch runtime.GOOS {
	case "windows":
		t.Setenv("AppData", notDirectory)
	case "darwin":
		t.Setenv("HOME", notDirectory)
	default:
		t.Setenv("XDG_CONFIG_HOME", notDirectory)
	}

	if _, err := loadConfig("", false); err == nil || !strings.Contains(err.Error(), "stat default config") {
		t.Fatalf("loadConfig error = %v, want stat error", err)
	}
}

func setTestAppDirs(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_STATE_HOME", dir)
	t.Setenv("AppData", dir)
}

func buildTestDatabase(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	input := filepath.Join(dir, "EIJIRO1-0.TXT")
	if err := os.WriteFile(input, []byte("Alpha : first body\r\n-prefix : dash body\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "eiji.sqlite3")
	if _, err := builder.Run(builder.Options{
		Type: dictionary.Eiji, Input: input, Output: output, Progress: io.Discard,
	}); err != nil {
		t.Fatalf("build test database: %v", err)
	}
	return output
}

func writeTestConfig(t *testing.T, dbPath string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	body := fmt.Sprintf("[eiji]\ndatabase = %s\n\n[search]\ndefault_dictionary = \"eiji\"\nmax_results = 50\n", strconv.Quote(dbPath))
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

type panicReader struct{}

func (panicReader) Read([]byte) (int, error) {
	panic("stdin must not be read")
}
