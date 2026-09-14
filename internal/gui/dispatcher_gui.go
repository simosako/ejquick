//go:build gui

package gui

import (
	"sync"

	"github.com/mappu/miqt/qt6/mainthread"

	"github.com/simosako/ejquick/internal/gui/controller"
)

type qtDispatcher struct {
	mu        sync.Mutex
	cond      *sync.Cond
	accepting bool
	pending   int
	handler   func(controller.Event)
	onPanic   func(any)
}

func newQtDispatcher(onPanic func(any)) *qtDispatcher {
	dispatcher := &qtDispatcher{accepting: true, onPanic: onPanic}
	dispatcher.cond = sync.NewCond(&dispatcher.mu)
	return dispatcher
}

func (d *qtDispatcher) setHandler(handler func(controller.Event)) {
	d.mu.Lock()
	d.handler = handler
	d.mu.Unlock()
}

func (d *qtDispatcher) Post(event controller.Event) (accepted bool) {
	d.mu.Lock()
	if !d.accepting {
		d.mu.Unlock()
		return false
	}
	d.pending++
	d.mu.Unlock()

	started := false
	defer func() {
		if recovered := recover(); recovered != nil {
			if !started {
				d.done()
			}
			accepted = false
		}
	}()
	mainthread.Start(func() {
		defer d.done()
		defer func() {
			if recovered := recover(); recovered != nil && d.onPanic != nil {
				d.onPanic(recovered)
			}
		}()
		if !mainthread.IsCurrent() {
			panic("GUI event dispatched outside Qt main thread")
		}
		d.mu.Lock()
		handler := d.handler
		d.mu.Unlock()
		if handler != nil {
			handler(event)
		}
	})
	started = true
	return true
}

func (d *qtDispatcher) close() {
	d.mu.Lock()
	d.accepting = false
	d.mu.Unlock()
}

func (d *qtDispatcher) wait() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for d.pending != 0 {
		d.cond.Wait()
	}
}

func (d *qtDispatcher) done() {
	d.mu.Lock()
	d.pending--
	if d.pending == 0 {
		d.cond.Broadcast()
	}
	d.mu.Unlock()
}
