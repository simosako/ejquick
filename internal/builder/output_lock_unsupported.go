//go:build !linux && !darwin && !windows

package builder

import "errors"

func acquireOutputLock(string) (*outputLock, error) {
	return nil, errors.New("output locking is not supported on this platform")
}

func releaseOutputLock(lock *outputLock) error {
	err := lock.file.Close()
	lock.file = nil
	return err
}
