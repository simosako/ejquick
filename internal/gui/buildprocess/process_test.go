package buildprocess

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/simosako/ejquick/internal/buildprotocol"
	"github.com/simosako/ejquick/internal/dictionary"
)

const helperProcessEnv = "EJQUICK_TEST_BUILDPROCESS_HELPER"

func TestProcessSuccess(t *testing.T) {
	options := testOptions(t)
	events := make(chan Event, 8)
	process, err := start(options, func(event Event) { events <- event }, helperCommand("success", ""))
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	result := waitResult(t, process)
	if result.Outcome != OutcomeSucceeded || result.Category != "" || result.ExitCode != 0 || result.Err != nil {
		t.Fatalf("result = %+v", result)
	}
	wantStats := Stats{
		SourceLines: 11,
		Entries:     9,
		Skipped:     2,
		DBSize:      1234,
		Elapsed:     1500 * time.Millisecond,
	}
	if !reflect.DeepEqual(result.Stats, wantStats) {
		t.Errorf("stats = %+v, want %+v", result.Stats, wantStats)
	}

	var kinds []EventKind
	for len(events) > 0 {
		kinds = append(kinds, (<-events).Kind)
	}
	wantKinds := []EventKind{EventReady, EventPhase, EventProgress, EventFinished}
	if !reflect.DeepEqual(kinds, wantKinds) {
		t.Errorf("event kinds = %v, want %v", kinds, wantKinds)
	}
	if got, ok := process.Result(); !ok || got.Outcome != OutcomeSucceeded {
		t.Errorf("Result() = (%+v, %t)", got, ok)
	}
	process.Cancel()
}

func TestProcessGracefulCancel(t *testing.T) {
	options := testOptions(t)
	events := make(chan Event, 8)
	process, err := start(options, func(event Event) { events <- event }, helperCommand("cancel", ""))
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	waitForEvent(t, events, EventReady)

	process.Cancel()
	process.Cancel()
	result := waitResult(t, process)
	if result.Outcome != OutcomeCanceled || result.Category != "" || result.ExitCode != 130 {
		t.Fatalf("result = %+v", result)
	}
	if result.HardKilled {
		t.Fatal("gracefully canceled process was marked hard-killed")
	}
}

func TestProcessRejectsIncompatibleReadyEvent(t *testing.T) {
	tests := []struct {
		name     string
		scenario string
		want     Category
	}{
		{name: "product version", scenario: "product-mismatch", want: CategoryVersionMismatch},
		{name: "protocol version", scenario: "protocol-mismatch", want: CategoryVersionMismatch},
		{name: "dictionary", scenario: "dictionary-mismatch", want: CategoryProtocolError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			process, err := start(testOptions(t), nil, helperCommand(tt.scenario, ""))
			if err != nil {
				t.Fatalf("start: %v", err)
			}
			result := waitResult(t, process)
			if result.Outcome != OutcomeFailed || result.Category != tt.want {
				t.Fatalf("result = %+v, want category %q", result, tt.want)
			}
			if result.HardKilled {
				t.Fatal("cooperative incompatible child was hard-killed")
			}
		})
	}
}

func TestProcessRejectsMalformedProtocol(t *testing.T) {
	process, err := start(testOptions(t), nil, helperCommand("malformed", ""))
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	result := waitResult(t, process)
	if result.Outcome != OutcomeFailed || result.Category != CategoryProtocolError || result.Err == nil {
		t.Fatalf("result = %+v", result)
	}
}

func TestProcessClassifiesChildFailure(t *testing.T) {
	tests := []struct {
		code string
		want Category
	}{
		{code: "source_invalid", want: CategorySourceInvalid},
		{code: "output_busy", want: CategoryOutputBusy},
		{code: "build_failed", want: CategoryBuildFailed},
		{code: "protocol_error", want: CategoryProtocolError},
		{code: "future_code", want: CategoryUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			process, err := start(testOptions(t), nil, helperCommand("failed", tt.code))
			if err != nil {
				t.Fatalf("start: %v", err)
			}
			result := waitResult(t, process)
			if result.Outcome != OutcomeFailed || result.Category != tt.want || result.ExitCode != 2 {
				t.Fatalf("result = %+v, want category %q", result, tt.want)
			}
		})
	}
}

func TestProcessRequiresCompletionAndZeroExit(t *testing.T) {
	process, err := start(testOptions(t), nil, helperCommand("completed-nonzero", ""))
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	result := waitResult(t, process)
	if result.Outcome != OutcomeFailed || result.Category != CategoryProcessDied || result.ExitCode != 2 {
		t.Fatalf("result = %+v", result)
	}
}

func TestProcessDeathKeepsBoundedStderrTail(t *testing.T) {
	process, err := start(testOptions(t), nil, helperCommand("died", ""))
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	result := waitResult(t, process)
	if result.Outcome != OutcomeFailed || result.Category != CategoryProcessDied || result.ExitCode != 23 {
		t.Fatalf("result = %+v", result)
	}
	if len(result.Stderr) > stderrTailBytes {
		t.Errorf("stderr tail length = %d, want <= %d", len(result.Stderr), stderrTailBytes)
	}
	if !strings.HasSuffix(result.Stderr, "stderr-end") {
		t.Errorf("stderr tail does not contain final bytes: %q", result.Stderr)
	}
}

func TestProcessHardKillCleansReportedTemporaryDatabase(t *testing.T) {
	options := testOptions(t)
	options.KillAfter = 100 * time.Millisecond
	temporary := filepath.Join(filepath.Dir(options.Output), ".ejquick-build-owned.sqlite3")
	if err := os.WriteFile(temporary, []byte("temporary"), 0o600); err != nil {
		t.Fatal(err)
	}
	events := make(chan Event, 8)
	process, err := start(options, func(event Event) { events <- event }, helperCommand("hang", temporary))
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	// The helper emits this phase only after the temporary path event.
	waitForEvent(t, events, EventPhase)
	process.Cancel()

	result := waitResult(t, process)
	if result.Outcome != OutcomeCanceled || !result.HardKilled || result.CleanupErr != nil {
		t.Fatalf("result = %+v", result)
	}
	if _, err := os.Lstat(temporary); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("temporary database remains after hard kill: %v", err)
	}
}

func TestCleanupTemporaryRejectsUnsafePaths(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "eiwa.sqlite3")
	if err := os.WriteFile(output, []byte("output"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		path func(*testing.T) string
	}{
		{name: "missing event", path: func(*testing.T) string { return "" }},
		{name: "relative", path: func(*testing.T) string { return ".ejquick-build-relative.sqlite3" }},
		{name: "final output", path: func(*testing.T) string { return output }},
		{name: "wrong name", path: func(t *testing.T) string {
			path := filepath.Join(dir, "temporary.sqlite3")
			writeTestFile(t, path)
			return path
		}},
		{name: "wrong directory", path: func(t *testing.T) string {
			other := t.TempDir()
			path := filepath.Join(other, ".ejquick-build-other.sqlite3")
			writeTestFile(t, path)
			return path
		}},
		{name: "directory", path: func(t *testing.T) string {
			path := filepath.Join(dir, ".ejquick-build-directory.sqlite3")
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
			return path
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.path(t)
			if err := cleanupTemporary(path, output); err == nil {
				t.Fatal("cleanupTemporary returned nil")
			}
			if path != "" && filepath.IsAbs(path) {
				if _, err := os.Lstat(path); err != nil {
					t.Errorf("rejected path was modified: %v", err)
				}
			}
		})
	}

	t.Run("symlink", func(t *testing.T) {
		target := filepath.Join(dir, "symlink-target")
		writeTestFile(t, target)
		path := filepath.Join(dir, ".ejquick-build-symlink.sqlite3")
		if err := os.Symlink(target, path); err != nil {
			t.Skipf("symlink is unavailable: %v", err)
		}
		if err := cleanupTemporary(path, output); err == nil {
			t.Fatal("cleanupTemporary accepted a symlink")
		}
		if _, err := os.Lstat(path); err != nil {
			t.Errorf("symlink was removed: %v", err)
		}
		if _, err := os.Stat(target); err != nil {
			t.Errorf("symlink target was modified: %v", err)
		}
	})
}

func TestCleanupTemporaryRemovesOnlyValidRegularFile(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "eiwa.sqlite3")
	temporary := filepath.Join(dir, ".ejquick-build-valid.sqlite3")
	writeTestFile(t, temporary)
	if err := cleanupTemporary(temporary, output); err != nil {
		t.Fatalf("cleanupTemporary: %v", err)
	}
	if _, err := os.Lstat(temporary); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("temporary file remains: %v", err)
	}
	if err := cleanupTemporary(temporary, output); err != nil {
		t.Errorf("cleanupTemporary on an already removed path: %v", err)
	}
}

func TestStartClassifiesValidationAndLaunchFailures(t *testing.T) {
	options := testOptions(t)
	options.Binary = ""
	if _, err := Start(options, nil); CategoryOf(err) != CategoryBinaryMissing {
		t.Errorf("empty binary error = %v, category = %q", err, CategoryOf(err))
	}

	options = testOptions(t)
	options.Binary = filepath.Join(t.TempDir(), "missing-ejquick-build")
	if _, err := Start(options, nil); CategoryOf(err) != CategoryBinaryMissing {
		t.Errorf("missing binary error = %v, category = %q", err, CategoryOf(err))
	}

	options = testOptions(t)
	options.Source = filepath.Join(t.TempDir(), "missing.TXT")
	if _, err := Start(options, nil); CategoryOf(err) != CategorySourceInvalid {
		t.Errorf("missing source error = %v, category = %q", err, CategoryOf(err))
	}

	options = testOptions(t)
	options.Output = options.Source
	if _, err := Start(options, nil); CategoryOf(err) != CategorySourceInvalid {
		t.Errorf("same path error = %v, category = %q", err, CategoryOf(err))
	}

	options = testOptions(t)
	parentFile := filepath.Join(t.TempDir(), "not-a-directory")
	writeTestFile(t, parentFile)
	options.Output = filepath.Join(parentFile, "eiwa.sqlite3")
	if _, err := Start(options, nil); CategoryOf(err) != CategoryBuildFailed {
		t.Errorf("output parent error = %v, category = %q", err, CategoryOf(err))
	}
}

func TestValidateCreatesMissingOutputParent(t *testing.T) {
	options := testOptions(t)
	options.Output = filepath.Join(filepath.Dir(options.Output), "missing", "nested", "eiwa.sqlite3")
	if err := Validate(options); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	info, err := os.Stat(filepath.Dir(options.Output))
	if err != nil {
		t.Fatalf("stat output parent: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("output parent is not a directory")
	}
}

func TestBuilderCommandArguments(t *testing.T) {
	options := testOptions(t)
	options.Source = "-source.TXT"
	options.Force = true
	cmd := builderCommand(options)
	want := []string{
		options.Binary,
		"--machine-protocol", "1",
		"--type", "eiwa",
		"--output", options.Output,
		"--force",
		"--", "-source.TXT",
	}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Errorf("args = %q, want %q", cmd.Args, want)
	}
}

func TestTailWriterKeepsOnlyFinalBytes(t *testing.T) {
	writer := &tailWriter{limit: 5}
	if n, err := writer.Write([]byte("abc")); err != nil || n != 3 {
		t.Fatalf("first Write = (%d, %v)", n, err)
	}
	if n, err := writer.Write([]byte("defgh")); err != nil || n != 5 {
		t.Fatalf("second Write = (%d, %v)", n, err)
	}
	if got := writer.String(); got != "defgh" {
		t.Errorf("tail = %q, want defgh", got)
	}
}

func testOptions(t *testing.T) Options {
	t.Helper()
	dir := t.TempDir()
	source := filepath.Join(dir, "source.TXT")
	if err := os.WriteFile(source, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	return Options{
		Binary:         os.Args[0],
		Dictionary:     dictionary.Eiwa,
		Source:         source,
		Output:         filepath.Join(dir, "eiwa.sqlite3"),
		ProductVersion: "test-version",
		KillAfter:      2 * time.Second,
	}
}

func helperCommand(scenario, value string) commandFactory {
	return func(options Options) *exec.Cmd {
		cmd := exec.Command(os.Args[0], "-test.run=^TestBuildProcessHelper$")
		cmd.Env = append(os.Environ(),
			helperProcessEnv+"=1",
			"EJQUICK_TEST_BUILDPROCESS_SCENARIO="+scenario,
			"EJQUICK_TEST_BUILDPROCESS_VALUE="+value,
			"EJQUICK_TEST_BUILDPROCESS_VERSION="+options.ProductVersion,
			"EJQUICK_TEST_BUILDPROCESS_DICTIONARY="+options.Dictionary.String(),
		)
		return cmd
	}
}

func waitResult(t *testing.T, process *Process) Result {
	t.Helper()
	select {
	case <-process.Done():
		return process.Wait()
	case <-time.After(5 * time.Second):
		process.Cancel()
		t.Fatal("builder process did not finish")
		return Result{}
	}
}

func waitForEvent(t *testing.T, events <-chan Event, kind EventKind) Event {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case event := <-events:
			if event.Kind == kind {
				return event
			}
		case <-deadline:
			t.Fatalf("builder event %d did not arrive", kind)
		}
	}
}

func writeTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestBuildProcessHelper runs as a child process for supervisor tests.
func TestBuildProcessHelper(t *testing.T) {
	if os.Getenv(helperProcessEnv) != "1" {
		return
	}
	os.Exit(runBuildProcessHelper())
}

func runBuildProcessHelper() int {
	scenario := os.Getenv("EJQUICK_TEST_BUILDPROCESS_SCENARIO")
	value := os.Getenv("EJQUICK_TEST_BUILDPROCESS_VALUE")
	protocol := buildprotocol.Version
	version := os.Getenv("EJQUICK_TEST_BUILDPROCESS_VERSION")
	dictionaryName := os.Getenv("EJQUICK_TEST_BUILDPROCESS_DICTIONARY")
	switch scenario {
	case "product-mismatch":
		version = "other-version"
	case "protocol-mismatch":
		protocol++
	case "dictionary-mismatch":
		dictionaryName = dictionary.Waei.String()
	}

	encoder := json.NewEncoder(os.Stdout)
	decoder := json.NewDecoder(os.Stdin)
	if err := encoder.Encode(buildprotocol.Event{
		Protocol:       protocol,
		Event:          "ready",
		ProductVersion: version,
		Dictionary:     dictionaryName,
	}); err != nil {
		return 91
	}

	var command buildprotocol.Command
	if err := decoder.Decode(&command); err != nil {
		return 92
	}
	if scenario == "product-mismatch" || scenario == "protocol-mismatch" || scenario == "dictionary-mismatch" {
		if command.Command != "cancel" {
			return 93
		}
		_ = encoder.Encode(buildprotocol.Event{Protocol: buildprotocol.Version, Event: "cancelled"})
		return 130
	}
	if command.Protocol != buildprotocol.Version || command.Command != "start" {
		return 94
	}

	switch scenario {
	case "success":
		_ = encoder.Encode(buildprotocol.Event{Protocol: buildprotocol.Version, Event: "phase", Phase: "reading", State: "started"})
		_ = encoder.Encode(buildprotocol.Event{Protocol: buildprotocol.Version, Event: "progress", Phase: "reading", Lines: 11, Entries: 9, Skipped: 2})
		_ = encoder.Encode(buildprotocol.Event{
			Protocol: buildprotocol.Version, Event: "completed", SourceLines: 11,
			Entries: 9, Skipped: 2, DBSize: 1234, ElapsedMS: 1500,
		})
		return 0
	case "cancel":
		if err := decoder.Decode(&command); err != nil || command.Command != "cancel" {
			return 95
		}
		_ = encoder.Encode(buildprotocol.Event{Protocol: buildprotocol.Version, Event: "cancelled"})
		return 130
	case "malformed":
		_, _ = fmt.Fprintln(os.Stdout, "{not-json")
		if err := decoder.Decode(&command); err != nil || command.Command != "cancel" {
			return 96
		}
		return 2
	case "failed":
		_ = encoder.Encode(buildprotocol.Event{Protocol: buildprotocol.Version, Event: "failed", Code: value})
		return 2
	case "completed-nonzero":
		_ = encoder.Encode(buildprotocol.Event{Protocol: buildprotocol.Version, Event: "completed"})
		return 2
	case "died":
		_, _ = fmt.Fprint(os.Stderr, strings.Repeat("x", stderrTailBytes+1000), "stderr-end")
		return 23
	case "hang":
		_ = encoder.Encode(buildprotocol.Event{Protocol: buildprotocol.Version, Event: "temporary_created", Path: value})
		_ = encoder.Encode(buildprotocol.Event{Protocol: buildprotocol.Version, Event: "phase", Phase: "reading", State: "started"})
		if err := decoder.Decode(&command); err != nil || command.Command != "cancel" {
			return 97
		}
		for {
			time.Sleep(time.Hour)
		}
	default:
		return 98
	}
}
