// Package logging provides the EJQuick append-only log file used by the
// search application. Logs are best effort: a broken log file never
// prevents the application from running.
package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// Filename is the log file name inside the application directory.
const Filename = "ejquick.log"

// Logger writes timestamped lines to a log file (or io.Discard when
// logging could not be set up). It is safe for concurrent use.
type Logger struct {
	mu    sync.Mutex
	w     io.Writer
	file  *os.File
	debug bool
}

// Nop returns a logger that discards everything.
func Nop() *Logger { return &Logger{w: io.Discard} }

// DefaultPath returns the platform-conventional log file path:
//
//	Linux:   $XDG_STATE_HOME/ejquick/ejquick.log or
//	         $HOME/.local/state/ejquick/ejquick.log
//	macOS:   $HOME/Library/Application Support/ejquick/ejquick.log
//	Windows: %AppData%\ejquick\ejquick.log
func DefaultPath() (string, error) {
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("AppData")
		if appData == "" {
			return "", fmt.Errorf("AppData is not set")
		}
		return filepath.Join(appData, "ejquick", Filename), nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "ejquick", Filename), nil
	default:
		if state := os.Getenv("XDG_STATE_HOME"); state != "" {
			return filepath.Join(state, "ejquick", Filename), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "state", "ejquick", Filename), nil
	}
}

// Open resolves DefaultPath, creates the directory and file, and returns
// a logger appending to it. On any failure it reports the problem on
// stderr once and returns a discarding logger, so callers can proceed.
func Open(debug bool) *Logger {
	path, err := DefaultPath()
	if err != nil {
		warnNoLog(err)
		return Nop()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		warnNoLog(err)
		return Nop()
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		warnNoLog(err)
		return Nop()
	}
	return &Logger{w: f, file: f, debug: debug}
}

// OpenFile returns a logger appending to an explicit file path, creating
// it when missing. Used by tests.
func OpenFile(path string, debug bool) (*Logger, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return &Logger{w: f, file: f, debug: debug}, nil
}

// Close releases the underlying file, if any.
func (l *Logger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	return l.file.Close()
}

// DebugEnabled reports whether debug logging is on.
func (l *Logger) DebugEnabled() bool { return l != nil && l.debug }

// Error appends an ERROR line.
func (l *Logger) Error(format string, args ...any) {
	l.write("ERROR", format, args...)
}

// Debug appends a DEBUG line when debug logging is enabled.
func (l *Logger) Debug(format string, args ...any) {
	if !l.DebugEnabled() {
		return
	}
	l.write("DEBUG", format, args...)
}

func (l *Logger) write(level, format string, args ...any) {
	if l == nil || l.w == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(l.w, "%s %s %s\n", time.Now().Format(time.RFC3339), level, msg)
}

// warnNoLog prints a single stderr line explaining that logging is
// unavailable; the application continues without a log file.
func warnNoLog(err error) {
	fmt.Fprintf(os.Stderr, "ejquick: logging unavailable: %v\n", err)
}
