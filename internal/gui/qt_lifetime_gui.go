//go:build gui

package gui

import "runtime"

type qtDeletable interface {
	Delete()
}

// deleteQtWrapper disarms a possible MIQT value finalizer before explicitly
// deleting the wrapper. MIQT's Delete methods do not disarm that finalizer.
func deleteQtWrapper[T qtDeletable](value T) {
	runtime.SetFinalizer(value, nil)
	value.Delete()
}
