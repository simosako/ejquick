//go:build gui

// Qt main-thread dispatch for worker results (design D37): typed
// UIEvents posted through miqt's mainthread helper, with a closing state
// that rejects new posts and turns queued callbacks into no-ops so
// shutdown never touches widgets (design D38).
package gui

import (
	"sync"

	"github.com/mappu/miqt/qt6/mainthread"
)

// qtDispatcher delivers UIEvents from goroutines to the Qt main thread.
// Post never blocks: rejected events are dropped and their payloads
// become collectable (design D37).
type qtDispatcher struct {
	mu      sync.Mutex
	closing bool
	apply   func(UIEvent)
}

// newQtDispatcher wraps the single main-thread apply entry point.
func newQtDispatcher(apply func(UIEvent)) *qtDispatcher {
	return &qtDispatcher{apply: apply}
}

// Post queues ev for the Qt main thread. It returns false once the
// dispatcher has closed.
func (d *qtDispatcher) Post(ev UIEvent) bool {
	d.mu.Lock()
	if d.closing {
		d.mu.Unlock()
		return false
	}
	d.mu.Unlock()
	mainthread.Start(func() {
		// The dispatcher may close while this callback was queued; a
		// stale callback must not touch Qt objects (design D38).
		if d.isClosed() {
			return
		}
		d.apply(ev)
	})
	return true
}

// Close stops accepting events. Queued callbacks complete as no-ops.
func (d *qtDispatcher) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closing = true
}

func (d *qtDispatcher) isClosed() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.closing
}
