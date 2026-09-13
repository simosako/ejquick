package builder

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var errOutputBusy = errors.New("another dictionary database build is already running")

type outputLock struct {
	file *os.File
}

func (l *outputLock) close() error {
	if l == nil || l.file == nil {
		return nil
	}
	return releaseOutputLock(l)
}

// prepareOutput resolves the output parent once and acquires the persistent
// sibling lock. The returned canonical path is used for the entire run.
func prepareOutput(output string) (string, *outputLock, error) {
	abs, err := filepath.Abs(output)
	if err != nil {
		return "", nil, fmt.Errorf("resolve output path: %w", err)
	}
	parent := filepath.Dir(abs)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", nil, fmt.Errorf("create output directory: %w", err)
	}
	parent, err = filepath.EvalSymlinks(parent)
	if err != nil {
		return "", nil, fmt.Errorf("resolve output directory: %w", err)
	}
	canonical := filepath.Join(parent, filepath.Base(abs))

	lockPath := filepath.Join(parent, "."+filepath.Base(canonical)+".ejquick-build.lock")
	lock, err := acquireOutputLock(lockPath)
	if err != nil {
		if errors.Is(err, errOutputBusy) {
			return "", nil, classify(ErrorOutputBusy, err)
		}
		return "", nil, fmt.Errorf("lock output %s: %w", canonical, err)
	}
	return canonical, lock, nil
}

func rejectUnsafeOutput(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect output: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("output database must not be a symbolic link")
	}
	if !info.Mode().IsRegular() {
		return errors.New("output database must be a regular file")
	}
	return nil
}
