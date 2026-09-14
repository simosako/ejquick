package builder

import (
	"errors"
	"fmt"
	"io"
)

// EventKind identifies a typed build progress event.
type EventKind string

const (
	EventPhase            EventKind = "phase"
	EventProgress         EventKind = "progress"
	EventSkipped          EventKind = "skipped"
	EventTemporaryCreated EventKind = "temporary_created"
	EventCompleted        EventKind = "completed"
)

// Phase identifies a stable machine-readable build phase.
type Phase string

const (
	PhaseReading    Phase = "reading"
	PhaseBTree      Phase = "btree"
	PhaseFTS        Phase = "fts"
	PhaseVacuum     Phase = "vacuum"
	PhaseAnalyze    Phase = "analyze"
	PhaseValidation Phase = "validation"
)

// PhaseState identifies whether a phase started, completed, or failed.
type PhaseState string

const (
	PhaseStarted PhaseState = "started"
	PhaseDone    PhaseState = "done"
	PhaseFailed  PhaseState = "failed"
)

// Event is an immutable progress value passed to a RunContext reporter.
// Err is intended for human diagnostics and must not be copied verbatim to
// a machine protocol or user interface.
type Event struct {
	Kind          EventKind
	Phase         Phase
	State         PhaseState
	Lines         int64
	Entries       int64
	Skipped       int64
	TemporaryPath string
	Reason        string
	Stats         Stats
	Err           error
}

// ErrorCode is a stable classification used by frontends and the machine
// protocol. The wrapped cause remains available to CLI callers and logs.
type ErrorCode string

const (
	ErrorSourceInvalid ErrorCode = "source_invalid"
	ErrorOutputBusy    ErrorCode = "output_busy"
	ErrorBuildFailed   ErrorCode = "build_failed"
)

// BuildError classifies a build failure without replacing its cause.
type BuildError struct {
	Code ErrorCode
	Err  error
}

func (e *BuildError) Error() string { return e.Err.Error() }
func (e *BuildError) Unwrap() error { return e.Err }

// CodeOf returns the stable error code for err.
func CodeOf(err error) ErrorCode {
	var buildErr *BuildError
	if errors.As(err, &buildErr) {
		return buildErr.Code
	}
	return ErrorBuildFailed
}

func classify(code ErrorCode, err error) error {
	if err == nil {
		return nil
	}
	var buildErr *BuildError
	if errors.As(err, &buildErr) {
		return err
	}
	return &BuildError{Code: code, Err: err}
}

type reporter func(Event) error

func emit(report reporter, event Event) error {
	if report == nil {
		return nil
	}
	if err := report(event); err != nil {
		return fmt.Errorf("report build progress: %w", err)
	}
	return nil
}

func humanReporter(w io.Writer) reporter {
	if w == nil {
		w = io.Discard
	}
	return func(event Event) error {
		switch event.Kind {
		case EventTemporaryCreated:
			return nil
		case EventProgress:
			_, _ = fmt.Fprintf(w, "Reading: lines=%d entries=%d skipped=%d\n",
				event.Lines, event.Entries, event.Skipped)
		case EventSkipped:
			_, _ = fmt.Fprintf(w, "Skipped line %d: %s\n", event.Lines, event.Reason)
		case EventPhase:
			name := humanPhaseName(event.Phase)
			switch event.State {
			case PhaseStarted:
				_, _ = fmt.Fprintf(w, "%s: start\n", name)
			case PhaseDone:
				if event.Phase == PhaseReading {
					_, _ = fmt.Fprintf(w, "Reading: done lines=%d entries=%d skipped=%d\n",
						event.Lines, event.Entries, event.Skipped)
				} else {
					_, _ = fmt.Fprintf(w, "%s: done\n", name)
				}
			case PhaseFailed:
				_, _ = fmt.Fprintf(w, "%s: failed: %v\n", name, event.Err)
			}
		case EventCompleted:
			writeSummary(w, event.Stats)
		}
		// Human-readable progress has historically been best effort.
		return nil
	}
}

func humanPhaseName(phase Phase) string {
	switch phase {
	case PhaseReading:
		return "Reading"
	case PhaseBTree:
		return "B-tree index"
	case PhaseFTS:
		return "FTS build"
	case PhaseVacuum:
		return "VACUUM"
	case PhaseAnalyze:
		return "ANALYZE"
	case PhaseValidation:
		return "Validation"
	default:
		return string(phase)
	}
}
