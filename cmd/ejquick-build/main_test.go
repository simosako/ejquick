package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/simosako/ejquick/internal/dictionary"
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

func TestRunMachineProtocolBuild(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "EIJIRO1-0.TXT")
	output := filepath.Join(dir, "eiwa.sqlite3")
	if err := os.WriteFile(input, []byte("\x81\xa1alpha : first body\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdin, writer := io.Pipe()
	release := make(chan struct{})
	go func() {
		_, _ = fmt.Fprintln(writer, `{"protocol":1,"command":"start"}`)
		<-release
		_ = writer.Close()
	}()
	var stdout, stderr bytes.Buffer
	code := runWithIO([]string{
		"--machine-protocol", "1", "--type", "eiwa", "--output", output, input,
	}, stdin, &stdout, &stderr)
	close(release)
	if code != 0 {
		t.Fatalf("exit code = %d; stderr = %q", code, stderr.String())
	}
	events := decodeProtocolEvents(t, stdout.Bytes())
	if len(events) < 3 || events[0].Event != "ready" || events[len(events)-1].Event != "completed" {
		t.Fatalf("events = %+v", events)
	}
	if events[0].Protocol != 1 || events[0].Dictionary != "eiwa" {
		t.Errorf("ready event = %+v", events[0])
	}
	completed := events[len(events)-1]
	if completed.Entries != 1 || completed.SourceLines != 1 || completed.DBSize <= 0 {
		t.Errorf("completed event = %+v", completed)
	}
	terminal := 0
	for _, event := range events {
		if event.Event == "completed" || event.Event == "failed" || event.Event == "cancelled" {
			terminal++
		}
	}
	if terminal != 1 {
		t.Errorf("terminal event count = %d, want 1", terminal)
	}
}

func TestRunMachineProtocolCancel(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "EIJIRO1-0.TXT")
	output := filepath.Join(dir, "eiwa.sqlite3")
	if err := os.WriteFile(input, []byte("\x81\xa1alpha : first body\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdin := strings.NewReader("{\"protocol\":1,\"command\":\"start\"}\n" +
		"{\"protocol\":1,\"command\":\"cancel\"}\n")
	var stdout, stderr bytes.Buffer
	code := runWithIO([]string{
		"--machine-protocol", "1", "--type", "eiwa", "--output", output, input,
	}, stdin, &stdout, &stderr)
	if code != 130 {
		t.Fatalf("exit code = %d, want 130; stderr = %q", code, stderr.String())
	}
	events := decodeProtocolEvents(t, stdout.Bytes())
	if got := events[len(events)-1].Event; got != "cancelled" {
		t.Fatalf("terminal event = %q, want cancelled", got)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Errorf("output exists after cancel: %v", err)
	}
}

func TestRunMachineProtocolRejectsInvalidFirstCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runWithIO([]string{
		"--machine-protocol", "1", "--type", "eiwa", "input.TXT",
	}, strings.NewReader("{\"protocol\":1,\"command\":\"cancel\"}\n"), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	events := decodeProtocolEvents(t, stdout.Bytes())
	if len(events) != 2 || events[0].Event != "ready" || events[1].Code != "protocol_error" {
		t.Fatalf("events = %+v", events)
	}
}

func decodeProtocolEvents(t *testing.T, data []byte) []protocolEvent {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(data))
	var events []protocolEvent
	for {
		var event protocolEvent
		if err := decoder.Decode(&event); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("decode event: %v; output = %q", err, data)
		}
		events = append(events, event)
	}
	return events
}

func TestRunRejectsInvalidArguments(t *testing.T) {
	tests := [][]string{
		nil,
		{"--type", "bad", "in"},
		{"--type", "eiji", "in"},
		{"--type", "eiwa"},
		{"--type", "eiwa", "--input", "in"},
		{"--type", "eiwa", "in", "extra"},
		{"--type", "eiwa", "--output"},
		{"--type", "eiwa", "--output", "", "in"},
		{"--type", "eiwa", ""},
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
	output := filepath.Join(dir, "eiwa.sqlite3")
	if err := os.WriteFile(input, []byte("\x81\xa1alpha : first body\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--type", "eiwa",
		"--output", output,
		input,
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
	output := filepath.Join(dir, "eiwa.sqlite3")
	if err := os.WriteFile(input, []byte("\x81\xa1alpha : first body\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--type", "eiwa", "--output", output, input}, &stdout, &stderr)
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

func TestParseArgsUsesDefaultOutputForDictionary(t *testing.T) {
	defaultDir := setDefaultDataHome(t, t.TempDir())

	for _, dt := range dictionary.All {
		t.Run(dt.String(), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			opts, done, err := parseArgs([]string{"input.TXT", "--type", dt.String()}, &stdout, &stderr)
			if err != nil {
				t.Fatalf("parseArgs: %v", err)
			}
			if done {
				t.Fatal("parseArgs returned done")
			}
			if opts.Input != "input.TXT" {
				t.Errorf("input = %q, want input.TXT", opts.Input)
			}
			want := filepath.Join(defaultDir, dt.String()+".sqlite3")
			if opts.Output != want {
				t.Errorf("output = %q, want %q", opts.Output, want)
			}
		})
	}
}

func TestParseArgsAcceptsDashPrefixedInputAfterSeparator(t *testing.T) {
	var stdout, stderr bytes.Buffer
	opts, done, err := parseArgs([]string{"--type", "eiwa", "--", "-input.TXT"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if done {
		t.Fatal("parseArgs returned done")
	}
	if opts.Input != "-input.TXT" {
		t.Errorf("input = %q, want -input.TXT", opts.Input)
	}
}

func TestParseArgsAcceptsOptionsAfterInput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	opts, done, err := parseArgs([]string{
		"input.TXT", "--type", "waei", "--output", "output.sqlite3", "--force", "--compact",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if done {
		t.Fatal("parseArgs returned done")
	}
	if opts.Type != dictionary.Waei || opts.Input != "input.TXT" || opts.Output != "output.sqlite3" {
		t.Errorf("options = %+v", opts)
	}
	if !opts.Force || !opts.Compact {
		t.Errorf("force = %t, compact = %t; want both true", opts.Force, opts.Compact)
	}
}

func TestRunCreatesDefaultOutputDirectory(t *testing.T) {
	root := t.TempDir()
	dataHome := filepath.Join(root, "missing", "data")
	defaultDir := setDefaultDataHome(t, dataHome)

	input := filepath.Join(root, "EIJIRO1-0.TXT")
	if err := os.WriteFile(input, []byte("\x81\xa1alpha : first body\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(defaultDir, "eiwa.sqlite3")
	if _, err := os.Stat(filepath.Dir(output)); !os.IsNotExist(err) {
		t.Fatalf("default output directory exists before build: %v", err)
	}

	var stdout, stderr bytes.Buffer
	if code := run([]string{"--type", "eiwa", input}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatalf("stat default output: %v", err)
	}
}

func setDefaultDataHome(t *testing.T, dir string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Setenv("LocalAppData", dir)
	} else {
		t.Setenv("XDG_DATA_HOME", dir)
	}
	return filepath.Join(dir, "ejquick")
}
