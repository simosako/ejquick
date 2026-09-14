// Package buildprocess supervises the versioned ejquick-build child protocol
// without depending on Qt.
package buildprocess

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/simosako/ejquick/internal/buildprotocol"
	"github.com/simosako/ejquick/internal/dictionary"
)

const (
	defaultKillAfter = 10 * time.Second
	stderrTailBytes  = 4096
)

// Category is the closed set of GUI-facing builder failures.
type Category string

const (
	CategoryBinaryMissing   Category = "binary_missing"
	CategoryVersionMismatch Category = "version_mismatch"
	CategoryProtocolError   Category = "protocol_error"
	CategoryOutputBusy      Category = "output_busy"
	CategorySourceInvalid   Category = "source_invalid"
	CategoryBuildFailed     Category = "build_failed"
	CategoryProcessDied     Category = "process_died"
	CategoryUnknown         Category = "unknown"
)

// Error preserves a builder failure for logs and exposes a stable category.
type Error struct {
	Category Category
	Err      error
}

func (e *Error) Error() string { return e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

// CategoryOf returns the stable category carried by err, or CategoryUnknown.
func CategoryOf(err error) Category {
	var buildErr *Error
	if errors.As(err, &buildErr) && buildErr != nil {
		return buildErr.Category
	}
	return CategoryUnknown
}

// Options describes one child process invocation.
type Options struct {
	Binary         string
	Dictionary     dictionary.Type
	Source         string
	Output         string
	Force          bool
	ProductVersion string
	KillAfter      time.Duration
}

// Stats is the completed event summary.
type Stats struct {
	SourceLines int64
	Entries     int64
	Skipped     int64
	DBSize      int64
	Elapsed     time.Duration
}

// Outcome identifies the terminal process result.
type Outcome int

const (
	OutcomeSucceeded Outcome = iota
	OutcomeCanceled
	OutcomeFailed
)

// Result combines the terminal protocol event and operating-system process
// result. Stderr is a bounded tail intended only for logs.
type Result struct {
	Outcome    Outcome
	Category   Category
	Stats      Stats
	ExitCode   int
	Err        error
	Stderr     string
	HardKilled bool
	CleanupErr error
}

// EventKind identifies progress and final notifications.
type EventKind int

const (
	EventReady EventKind = iota
	EventPhase
	EventProgress
	EventFinished
)

// Event is an immutable notification delivered from a process goroutine.
type Event struct {
	Kind     EventKind
	Phase    string
	State    string
	Lines    int64
	Entries  int64
	Skipped  int64
	Finished Result
}

// Handler receives process events and must not block the pipe reader.
type Handler func(Event)

// Process owns one running builder child.
type Process struct {
	options Options
	handler Handler
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	done    chan struct{}

	mu            sync.Mutex
	result        Result
	finished      bool
	userCanceled  bool
	abortCategory Category
	hardKilled    bool
	temporaryPath string
	stopOnce      sync.Once
}

// Start validates obvious path errors and launches ejquick-build.
func Start(options Options, handler Handler) (*Process, error) {
	return start(options, handler, builderCommand)
}

// Validate checks paths and creates a missing output parent without launching
// the builder. Start repeats these checks to avoid relying on caller state.
func Validate(options Options) error {
	return validateForStart(&options)
}

func validateForStart(options *Options) error {
	if options.Binary == "" {
		return &Error{Category: CategoryBinaryMissing, Err: errors.New("builder binary path is empty")}
	}
	if _, err := exec.LookPath(options.Binary); err != nil {
		return &Error{Category: CategoryBinaryMissing, Err: fmt.Errorf("find builder %s: %w", options.Binary, err)}
	}
	if err := validateOptions(options); err != nil {
		return err
	}
	return nil
}

type commandFactory func(Options) *exec.Cmd

func start(options Options, handler Handler, factory commandFactory) (*Process, error) {
	if err := validateForStart(&options); err != nil {
		return nil, err
	}
	if options.KillAfter <= 0 {
		options.KillAfter = defaultKillAfter
	}
	cmd := factory(options)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, &Error{Category: CategoryBinaryMissing, Err: fmt.Errorf("builder stdout: %w", err)}
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdout.Close()
		return nil, &Error{Category: CategoryBinaryMissing, Err: fmt.Errorf("builder stderr: %w", err)}
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		_ = stdout.Close()
		_ = stderr.Close()
		return nil, &Error{Category: CategoryBinaryMissing, Err: fmt.Errorf("builder stdin: %w", err)}
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = stderr.Close()
		return nil, &Error{Category: CategoryBinaryMissing, Err: fmt.Errorf("start builder %s: %w", options.Binary, err)}
	}

	process := &Process{
		options: options,
		handler: handler,
		cmd:     cmd,
		stdin:   stdin,
		done:    make(chan struct{}),
	}
	go process.run(stdout, stderr)
	return process, nil
}

func builderCommand(options Options) *exec.Cmd {
	args := []string{
		"--machine-protocol", fmt.Sprint(buildprotocol.Version),
		"--type", options.Dictionary.String(),
		"--output", options.Output,
	}
	if options.Force {
		args = append(args, "--force")
	}
	args = append(args, "--", options.Source)
	return exec.Command(options.Binary, args...)
}

func validateOptions(options *Options) *Error {
	if !dictionary.IsValid(options.Dictionary) {
		return &Error{Category: CategoryUnknown, Err: fmt.Errorf("invalid dictionary %q", options.Dictionary)}
	}
	if options.Source == "" {
		return &Error{Category: CategorySourceInvalid, Err: errors.New("source path must not be empty")}
	}
	if options.Output == "" {
		return &Error{Category: CategoryBuildFailed, Err: errors.New("output path must not be empty")}
	}
	source, err := filepath.Abs(options.Source)
	if err != nil {
		return &Error{Category: CategorySourceInvalid, Err: fmt.Errorf("resolve source: %w", err)}
	}
	output, err := filepath.Abs(options.Output)
	if err != nil {
		return &Error{Category: CategoryBuildFailed, Err: fmt.Errorf("resolve output: %w", err)}
	}
	if pathsEqual(source, output) {
		return &Error{Category: CategorySourceInvalid, Err: errors.New("source and output paths are the same")}
	}
	info, err := os.Stat(source)
	if err != nil {
		return &Error{Category: CategorySourceInvalid, Err: fmt.Errorf("stat source: %w", err)}
	}
	if !info.Mode().IsRegular() {
		return &Error{Category: CategorySourceInvalid, Err: errors.New("source is not a regular file")}
	}
	if outputInfo, statErr := os.Stat(output); statErr == nil && os.SameFile(info, outputInfo) {
		return &Error{Category: CategorySourceInvalid, Err: errors.New("source and output refer to the same file")}
	} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return &Error{Category: CategoryBuildFailed, Err: fmt.Errorf("stat output: %w", statErr)}
	}
	file, err := os.Open(source)
	if err != nil {
		return &Error{Category: CategorySourceInvalid, Err: fmt.Errorf("open source: %w", err)}
	}
	if err := file.Close(); err != nil {
		return &Error{Category: CategorySourceInvalid, Err: fmt.Errorf("close source: %w", err)}
	}
	parent := filepath.Dir(output)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return &Error{Category: CategoryBuildFailed, Err: fmt.Errorf("create output directory: %w", err)}
	}
	parentInfo, err := os.Stat(parent)
	if err != nil {
		return &Error{Category: CategoryBuildFailed, Err: fmt.Errorf("stat output directory: %w", err)}
	}
	if !parentInfo.IsDir() {
		return &Error{Category: CategoryBuildFailed, Err: errors.New("output parent is not a directory")}
	}
	options.Source = source
	options.Output = output
	return nil
}

func (p *Process) run(stdout, stderr io.Reader) {
	stderrDone := make(chan string, 1)
	go func() {
		tail := &tailWriter{limit: stderrTailBytes}
		_, _ = io.CopyBuffer(tail, stderr, make([]byte, 32*1024))
		stderrDone <- tail.String()
	}()

	terminal, stats, protocolErr := p.readEvents(stdout)
	if protocolErr != nil {
		p.requestStop(CategoryProtocolError, false)
	}
	waitErr := p.cmd.Wait()
	p.mu.Lock()
	stdin := p.stdin
	p.stdin = nil
	p.mu.Unlock()
	if stdin != nil {
		_ = stdin.Close()
	}
	stderrTail := <-stderrDone

	p.mu.Lock()
	userCanceled := p.userCanceled
	abortCategory := p.abortCategory
	hardKilled := p.hardKilled
	temporaryPath := p.temporaryPath
	p.mu.Unlock()

	result := Result{
		ExitCode:   exitCode(waitErr),
		Err:        errors.Join(protocolErr, waitErr),
		Stderr:     stderrTail,
		HardKilled: hardKilled,
	}
	switch {
	case abortCategory != "":
		result.Outcome = OutcomeFailed
		result.Category = abortCategory
	case terminal.Event == "completed" && waitErr == nil:
		result.Outcome = OutcomeSucceeded
		result.Stats = stats
	case userCanceled:
		result.Outcome = OutcomeCanceled
	case terminal.Event == "failed" && result.ExitCode == 2:
		result.Outcome = OutcomeFailed
		result.Category = categoryFromChild(terminal.Code)
	default:
		result.Outcome = OutcomeFailed
		result.Category = CategoryProcessDied
	}
	if hardKilled {
		result.CleanupErr = cleanupTemporary(temporaryPath, p.options.Output)
	}

	p.mu.Lock()
	p.result = result
	p.finished = true
	p.mu.Unlock()
	p.emit(Event{Kind: EventFinished, Finished: result})
	close(p.done)
}

func (p *Process) readEvents(stdout io.Reader) (buildprotocol.Event, Stats, error) {
	reader := bufio.NewReaderSize(stdout, buildprotocol.MaxLineBytes+1)
	first := true
	terminalSeen := false
	var terminal buildprotocol.Event
	var stats Stats
	for {
		line, err := reader.ReadSlice('\n')
		if errors.Is(err, bufio.ErrBufferFull) || len(line) > buildprotocol.MaxLineBytes {
			return terminal, stats, errors.New("builder event exceeds 64 KiB")
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return terminal, stats, fmt.Errorf("read builder event: %w", err)
		}
		if errors.Is(err, io.EOF) && len(line) == 0 {
			break
		}
		line = bytes.TrimSuffix(line, []byte{'\n'})
		if len(line) == 0 {
			return terminal, stats, errors.New("empty builder event")
		}
		var event buildprotocol.Event
		if decodeErr := json.Unmarshal(line, &event); decodeErr != nil {
			return terminal, stats, fmt.Errorf("decode builder event: %w", decodeErr)
		}
		if event.Protocol != buildprotocol.Version {
			p.requestStop(CategoryVersionMismatch, false)
			return terminal, stats, fmt.Errorf("unsupported builder protocol %d", event.Protocol)
		}
		if terminalSeen {
			return terminal, stats, errors.New("builder sent an event after its terminal event")
		}
		if first {
			first = false
			if event.Event != "ready" {
				return terminal, stats, fmt.Errorf("first builder event is %q, want ready", event.Event)
			}
			if event.ProductVersion != p.options.ProductVersion {
				p.requestStop(CategoryVersionMismatch, false)
				return terminal, stats, fmt.Errorf("builder product version is %q, want %q", event.ProductVersion, p.options.ProductVersion)
			}
			if event.Dictionary != p.options.Dictionary.String() {
				return terminal, stats, fmt.Errorf("builder dictionary is %q, want %q", event.Dictionary, p.options.Dictionary)
			}
			if err := p.writeCommand("start"); err != nil {
				return terminal, stats, fmt.Errorf("start builder protocol: %w", err)
			}
			p.emit(Event{Kind: EventReady})
			continue
		}

		switch event.Event {
		case "phase":
			if !validPhase(event.Phase) || !validPhaseState(event.State) {
				return terminal, stats, fmt.Errorf("invalid builder phase %q state %q", event.Phase, event.State)
			}
			p.emit(Event{Kind: EventPhase, Phase: event.Phase, State: event.State})
		case "progress":
			if !validPhase(event.Phase) || event.Lines < 0 || event.Entries < 0 || event.Skipped < 0 {
				return terminal, stats, errors.New("invalid builder progress event")
			}
			p.emit(Event{Kind: EventProgress, Phase: event.Phase, Lines: event.Lines, Entries: event.Entries, Skipped: event.Skipped})
		case "temporary_created":
			if event.Path == "" {
				return terminal, stats, errors.New("empty builder temporary path")
			}
			p.mu.Lock()
			p.temporaryPath = event.Path
			p.mu.Unlock()
		case "completed":
			if event.SourceLines < 0 || event.Entries < 0 || event.Skipped < 0 || event.DBSize < 0 || event.ElapsedMS < 0 {
				return terminal, stats, errors.New("invalid builder completed event")
			}
			terminalSeen = true
			terminal = event
			stats = Stats{
				SourceLines: event.SourceLines,
				Entries:     event.Entries,
				Skipped:     event.Skipped,
				DBSize:      event.DBSize,
				Elapsed:     time.Duration(event.ElapsedMS) * time.Millisecond,
			}
		case "failed", "cancelled":
			terminalSeen = true
			terminal = event
		default:
			return terminal, stats, fmt.Errorf("unknown builder event %q", event.Event)
		}
		if errors.Is(err, io.EOF) {
			break
		}
	}
	if first {
		return terminal, stats, errors.New("builder sent no events")
	}
	return terminal, stats, nil
}

func validPhase(phase string) bool {
	switch phase {
	case "reading", "btree", "fts", "vacuum", "analyze", "validation":
		return true
	default:
		return false
	}
}

func validPhaseState(state string) bool {
	return state == "started" || state == "done" || state == "failed"
}

func categoryFromChild(code string) Category {
	switch code {
	case "source_invalid":
		return CategorySourceInvalid
	case "output_busy":
		return CategoryOutputBusy
	case "build_failed":
		return CategoryBuildFailed
	case "protocol_error":
		return CategoryProtocolError
	default:
		return CategoryUnknown
	}
}

func (p *Process) writeCommand(command string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stdin == nil {
		return io.ErrClosedPipe
	}
	return json.NewEncoder(p.stdin).Encode(buildprotocol.Command{Protocol: buildprotocol.Version, Command: command})
}

// Cancel requests graceful cancellation and starts the hard-kill deadline.
func (p *Process) Cancel() {
	if p == nil {
		return
	}
	p.requestStop("", true)
}

func (p *Process) requestStop(category Category, user bool) {
	p.mu.Lock()
	if p.finished {
		p.mu.Unlock()
		return
	}
	if user {
		p.userCanceled = true
	}
	if category != "" && p.abortCategory == "" {
		p.abortCategory = category
	}
	p.mu.Unlock()

	p.stopOnce.Do(func() {
		_ = p.writeCommand("cancel")
		p.mu.Lock()
		stdin := p.stdin
		p.stdin = nil
		p.mu.Unlock()
		if stdin != nil {
			_ = stdin.Close()
		}
		go func() {
			timer := time.NewTimer(p.options.KillAfter)
			defer timer.Stop()
			select {
			case <-p.done:
				return
			case <-timer.C:
				p.mu.Lock()
				p.hardKilled = true
				process := p.cmd.Process
				p.mu.Unlock()
				if process != nil {
					_ = process.Kill()
				}
			}
		}()
	})
}

// Done is closed after the child has exited and cleanup is complete.
func (p *Process) Done() <-chan struct{} { return p.done }

// Wait blocks until Done and returns the terminal result.
func (p *Process) Wait() Result {
	<-p.done
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.result
}

// Result returns the terminal result when available.
func (p *Process) Result() (Result, bool) {
	if p == nil {
		return Result{}, false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.result, p.finished
}

func (p *Process) emit(event Event) {
	if p.handler != nil {
		p.handler(event)
	}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

func cleanupTemporary(temporaryPath, outputPath string) error {
	if temporaryPath == "" {
		return errors.New("builder did not report a temporary path")
	}
	if !filepath.IsAbs(temporaryPath) {
		return fmt.Errorf("refuse relative temporary path %q", temporaryPath)
	}
	temporary, err := filepath.Abs(temporaryPath)
	if err != nil {
		return fmt.Errorf("resolve temporary path: %w", err)
	}
	output, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("resolve output path: %w", err)
	}
	temporary = filepath.Clean(temporary)
	output = filepath.Clean(output)
	name := filepath.Base(temporary)
	if pathsEqual(temporary, output) || !pathsEqual(filepath.Dir(temporary), filepath.Dir(output)) ||
		!strings.HasPrefix(name, ".ejquick-build-") || !strings.HasSuffix(name, ".sqlite3") {
		return fmt.Errorf("refuse unexpected temporary path %q", temporary)
	}
	info, err := os.Lstat(temporary)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect temporary path: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("refuse non-regular temporary path %q", temporary)
	}
	if err := os.Remove(temporary); err != nil {
		return fmt.Errorf("remove temporary path: %w", err)
	}
	return nil
}

func pathsEqual(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// ResolveBuilderPath returns the packaged sibling ejquick-build path.
func ResolveBuilderPath() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve GUI executable: %w", err)
	}
	name := "ejquick-build"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(filepath.Dir(executable), name), nil
}

type tailWriter struct {
	limit int
	data  []byte
}

func (w *tailWriter) Write(data []byte) (int, error) {
	w.data = append(w.data, data...)
	if len(w.data) > w.limit {
		w.data = append(w.data[:0], w.data[len(w.data)-w.limit:]...)
	}
	return len(data), nil
}

func (w *tailWriter) String() string { return string(w.data) }
