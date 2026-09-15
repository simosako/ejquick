//go:build gui

package gui

import (
	"fmt"
	"time"

	qt "github.com/mappu/miqt/qt6"

	"github.com/simosako/ejquick/internal/gui/controller"
)

const (
	guiSmokeQuery = "ejquick-smoke"
	guiSmokeBody  = "artificial GUI smoke entry"
)

type guiSmokeState struct {
	timer    *qt.QTimer
	started  bool
	deadline time.Time
}

// startSmokeTest schedules a synthetic search and clipboard flow after the
// window has entered the Qt event loop.
func (w *mainWindow) startSmokeTest() {
	state := &guiSmokeState{deadline: time.Now().Add(10 * time.Second)}
	timer := qt.NewQTimer2(w.main.QObject)
	state.timer = timer
	w.smoke = state
	timer.SetInterval(50)
	timer.OnTimeout(w.safeCallback(w.smokeTick))
	timer.Start2()
}

func (w *mainWindow) smokeTick() {
	state := w.smoke
	if state == nil || w.shuttingDown {
		return
	}
	if time.Now().After(state.deadline) {
		w.finishSmokeTest(1, "timed out waiting for the synthetic search")
		return
	}
	if w.controller == nil {
		w.finishSmokeTest(1, "no searchable database was opened")
		return
	}
	if !state.started {
		platform := qt.QGuiApplication_PlatformName()
		if platform != "wayland" {
			w.finishSmokeTest(1, fmt.Sprintf("Qt platform is %q, want %q", platform, "wayland"))
			return
		}
		if !w.main.QWidget.IsVisible() {
			w.finishSmokeTest(1, "main window is not visible")
			return
		}
		if !w.query.HasFocus() {
			w.finishSmokeTest(1, "search field did not receive focus")
			return
		}
		state.started = true
		w.query.SetText(guiSmokeQuery)
		w.queryEdited(guiSmokeQuery)
		return
	}

	snapshot := w.controller.Snapshot()
	switch snapshot.Content {
	case controller.ContentPending:
		return
	case controller.ContentNoResults:
		w.finishSmokeTest(1, "synthetic search returned no results")
		return
	case controller.ContentSearchError:
		w.finishSmokeTest(1, "synthetic search returned an error")
		return
	case controller.ContentResults:
		if err := w.verifySmokeResult(snapshot); err != nil {
			w.finishSmokeTest(1, err.Error())
			return
		}
		w.finishSmokeTest(0, "Wayland startup, search, model, clipboard, and shutdown passed")
	}
}

func (w *mainWindow) verifySmokeResult(snapshot controller.Snapshot) error {
	if snapshot.Query != guiSmokeQuery {
		return fmt.Errorf("controller query = %q, want %q", snapshot.Query, guiSmokeQuery)
	}
	if len(snapshot.Entries) != 1 || snapshot.Entries[0].Headword != guiSmokeQuery {
		return fmt.Errorf("controller results = %#v, want one synthetic entry", snapshot.Entries)
	}
	if w.query.Text() != guiSmokeQuery {
		return fmt.Errorf("search field = %q, want %q", w.query.Text(), guiSmokeQuery)
	}
	headwords := w.listModel.StringList()
	if len(headwords) != 1 || headwords[0] != guiSmokeQuery {
		return fmt.Errorf("result model = %#v, want one synthetic headword", headwords)
	}
	if w.headword.Text() != guiSmokeQuery {
		return fmt.Errorf("headword label = %q, want %q", w.headword.Text(), guiSmokeQuery)
	}
	if w.body.ToPlainText() != guiSmokeBody {
		return fmt.Errorf("body = %q, want %q", w.body.ToPlainText(), guiSmokeBody)
	}
	w.copyEntireEntry()
	wantClipboard := guiSmokeQuery + "\n" + guiSmokeBody
	if got := qt.QGuiApplication_Clipboard().Text(); got != wantClipboard {
		return fmt.Errorf("clipboard = %q, want %q", got, wantClipboard)
	}
	return nil
}

func (w *mainWindow) finishSmokeTest(exitCode int, message string) {
	state := w.smoke
	w.smoke = nil
	if state != nil && state.timer != nil {
		state.timer.Stop()
		deleteQtWrapper(state.timer)
	}
	if exitCode == 0 {
		w.logger.Debug("gui smoke: passed: %s", message)
	} else {
		w.logger.Error("gui smoke: failed: %s", message)
	}
	w.beginShutdown(exitCode)
}
