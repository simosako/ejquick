//go:build gui

// Qt widget integration tests (design D35 layer 2). One QApplication is
// created on a locked OS thread for the whole process; widget tests are
// not run in parallel. The offscreen platform keeps the suite headless —
// the packaged native Wayland startup is verified by the Weston job
// (design D36).
package gui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	qt "github.com/mappu/miqt/qt6"

	"github.com/simosako/ejquick/internal/dictionary"
	"github.com/simosako/ejquick/internal/logging"
	"github.com/simosako/ejquick/internal/search"
)

func TestMain(m *testing.M) {
	// Tests never exercise IME or real input methods; offscreen only
	// substitutes for a display, which the design allows for widget
	// logic tests (design D35/D36).
	if os.Getenv("QT_QPA_PLATFORM") == "" {
		os.Setenv("QT_QPA_PLATFORM", "offscreen")
	}
	runtime.LockOSThread()
	qt.QCoreApplication_SetOrganizationName(organizationName)
	qt.QCoreApplication_SetApplicationName(applicationName)
	qt.NewQApplication(os.Args)
	os.Exit(m.Run())
}

// Note on QModelIndex parameters: MIQT v0.14.0 bindings dereference
// QModelIndex-by-value arguments in C, so a nil parent crashes. Always
// pass qt.NewQModelIndex() for the invalid root index.

// newTestWindow builds a window around a controller with fake services
// and synchronous event application. A nil service marks that
// dictionary unavailable.
func newTestWindow(t *testing.T, eiwa, waei Searcher) (*mainWindow, *Controller) {
	t.Helper()
	services, statuses := bothAvailable(eiwa, waei)
	c := NewController(ControllerConfig{
		Services: services, Statuses: statuses,
		Initial: dictionary.Eiwa, MaxResults: 50, Logger: logging.Nop(),
	})
	c.runWorker = func(f func()) { f() }
	c.post = func(ev UIEvent) { c.ApplyEvent(ev) }
	w := newMainWindow(windowUI{
		controller: c,
		logger:     logging.Nop(),
		// persistence is tested separately
		dispatcher: nil,
	})
	t.Cleanup(func() {
		c.Finalize()
	})
	return w, c
}

func TestWindowInitialRender(t *testing.T) {
	w, _ := newTestWindow(t, &fakeSearcher{}, &fakeSearcher{})

	if got := w.dictCombo.Count(); got != 2 {
		t.Errorf("combo items = %d, want 2", got)
	}
	if got := w.dictCombo.ItemText(0); got != "English–Japanese" {
		t.Errorf("combo item 0 = %q", got)
	}
	if got := w.dictCombo.ItemText(1); got != "Japanese–English" {
		t.Errorf("combo item 1 = %q", got)
	}
	if !w.dictCombo.QWidget.IsEnabled() {
		t.Error("dictionary combo should be enabled with both dictionaries")
	}
	if !w.searchEdit.QWidget.IsEnabled() {
		t.Error("search field should be enabled")
	}
	if w.countLabel.Text() != "" {
		t.Errorf("count label = %q, want empty", w.countLabel.Text())
	}
	if w.stack.CurrentIndex() != messagePageIndex {
		t.Errorf("stack page = %d, want message", w.stack.CurrentIndex())
	}
	if got := w.messageLabel.Text(); got == "" || got != "Enter a search term\nUp/Down: Select result  Ctrl+Tab: Switch dictionary" {
		t.Errorf("message = %q", got)
	}
	if w.copyAction.IsEnabled() {
		t.Error("copy action should be disabled without a selection")
	}
	if w.resultList.SelectionModel().CurrentIndex().Row() != -1 {
		t.Error("no row should be selected initially")
	}
}

func TestWindowSearchFlowRendersResults(t *testing.T) {
	eiwa := &fakeSearcher{results: map[string][]search.Entry{
		"care": {
			{ID: 1, Headword: "care", Body: "attention"},
			{ID: 2, Headword: "career", Body: "profession"},
		},
	}}
	w, c := newTestWindow(t, eiwa, nil)

	// Simulate the textEdited path: the query reaches the controller,
	// the synchronous fake completes it, and render reflects the view.
	c.SetQuery("care")
	w.render(c.View())

	if got := w.listModel.RowCount(qt.NewQModelIndex()); got != 2 {
		t.Fatalf("rows = %d, want 2", got)
	}
	if w.stack.CurrentIndex() != detailPageIndex {
		t.Errorf("stack page = %d, want detail", w.stack.CurrentIndex())
	}
	if got := w.headwordLabel.Text(); got != "care" {
		t.Errorf("headword = %q", got)
	}
	if got := w.bodyEdit.ToPlainText(); got != "attention" {
		t.Errorf("body = %q", got)
	}
	if got := w.countLabel.Text(); got != "2 shown" {
		t.Errorf("count = %q", got)
	}
	if !w.copyAction.IsEnabled() {
		t.Error("copy action should be enabled with a selection")
	}
	if row := w.resultList.SelectionModel().CurrentIndex().Row(); row != 0 {
		t.Errorf("selected row = %d, want 0", row)
	}

	// Copy Entire Entry puts headword + "\n" + body on the clipboard
	// (design D18).
	c.SelectRow(1)
	w.render(c.View())
	w.copyEntry()
	if got := qt.QGuiApplication_Clipboard().Text(); got != "career\nprofession" {
		t.Errorf("clipboard = %q", got)
	}

	// Navigation moves the selection without touching focus.
	w.moveSelection(-1)
	if got := w.headwordLabel.Text(); got != "care" {
		t.Errorf("headword after move = %q", got)
	}
}

func TestWindowDictionarySwitchClears(t *testing.T) {
	eiwa := &fakeSearcher{results: map[string][]search.Entry{
		"a": {{ID: 1, Headword: "a1", Body: "x"}},
	}}
	waei := &fakeSearcher{}
	w, c := newTestWindow(t, eiwa, waei)

	c.SetQuery("a")
	w.render(c.View())
	if w.stack.CurrentIndex() != detailPageIndex {
		t.Fatal("setup: expected detail page")
	}

	w.switchDictionaryTo(dictionary.Waei)
	if got := w.dictCombo.CurrentIndex(); got != 1 {
		t.Errorf("combo index = %d, want 1", got)
	}
	if got := w.searchEdit.Text(); got != "" {
		t.Errorf("query = %q, want cleared", got)
	}
	if w.stack.CurrentIndex() != messagePageIndex {
		t.Errorf("stack page = %d, want message after switch", w.stack.CurrentIndex())
	}
	if got := w.listModel.RowCount(qt.NewQModelIndex()); got != 0 {
		t.Errorf("rows = %d, want 0 after switch", got)
	}
	if got := w.countLabel.Text(); got != "" {
		t.Errorf("count = %q, want empty after switch", got)
	}
}

func TestWindowBothUnavailable(t *testing.T) {
	statuses := map[dictionary.Type]DictStatus{
		dictionary.Eiwa: {Available: false, Category: search.OpenMissing},
		dictionary.Waei: {Available: false, Category: search.OpenIncompatible},
	}
	c := NewController(ControllerConfig{
		Services: map[dictionary.Type]Searcher{}, Statuses: statuses,
		Initial: dictionary.Eiwa, MaxResults: 50, Logger: logging.Nop(),
	})
	w := newMainWindow(windowUI{controller: c, logger: logging.Nop()})
	defer c.Finalize()

	if w.searchEdit.QWidget.IsEnabled() {
		t.Error("search field should be disabled without dictionaries")
	}
	if w.dictCombo.QWidget.IsEnabled() {
		t.Error("dictionary combo should be disabled without dictionaries")
	}
	if w.stack.CurrentIndex() != messagePageIndex {
		t.Fatalf("stack page = %d, want message", w.stack.CurrentIndex())
	}
	msg := w.messageLabel.Text()
	for _, want := range []string{"English–Japanese", "Japanese–English", "was not found", "incompatible"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q missing %q", msg, want)
		}
	}
}

func TestWindowSettingsSaveRestore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.conf")

	mkSettings := func() *qt.QSettings {
		return qt.NewQSettings4(path, qt.QSettings__IniFormat)
	}

	w1, _ := newTestWindow(t, &fakeSearcher{}, &fakeSearcher{})
	w1.settings = mkSettings()
	w1.window.QWidget.Resize(800, 600)
	w1.saveSettings()

	c := NewController(ControllerConfig{
		Services: map[dictionary.Type]Searcher{
			dictionary.Eiwa: &fakeSearcher{},
		},
		Statuses: map[dictionary.Type]DictStatus{
			dictionary.Eiwa: {Available: true},
			dictionary.Waei: {Available: false, Category: search.OpenMissing},
		},
		Initial: dictionary.Eiwa, MaxResults: 50, Logger: logging.Nop(),
	})
	defer c.Finalize()
	w2 := newMainWindow(windowUI{controller: c, logger: logging.Nop(), settings: mkSettings()})

	if got := w2.window.QWidget.Width(); got != 800 {
		t.Errorf("restored width = %d, want 800", got)
	}
	if got := w2.window.QWidget.Height(); got != 600 {
		t.Errorf("restored height = %d, want 600", got)
	}
}
