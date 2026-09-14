//go:build gui

// Qt Widgets implementation of the main search window (design D3, D9
// through D19, D24, D42/D43). All code in this file runs on the Qt main
// thread; worker results arrive only through the dispatcher and the
// controller (design D37).
package gui

import (
	"fmt"
	"strings"

	qt "github.com/mappu/miqt/qt6"

	"github.com/simosako/ejquick/internal/buildinfo"
	"github.com/simosako/ejquick/internal/dictionary"
	"github.com/simosako/ejquick/internal/logging"
)

// Initial window geometry and pane sizes are the D16 common defaults, in
// logical pixels (design D50). QSettings persistence uses the D16 keys.
const (
	initialWidth  = 960
	initialHeight = 640
	minimumWidth  = 720
	minimumHeight = 480
	leftMinWidth  = 240
	rightMinWidth = 320

	settingsStateVersion = 1
	keyStateVersion      = "gui/stateVersion"
	keyGeometry          = "mainWindow/geometry"
	keySplitter          = "mainWindow/splitter"

	// countWidestText is the longest possible count label text (design
	// D17); the label reserves its width so the query field does not
	// shift when the count changes.
	countWidestText = "Showing first 500"

	detailPageIndex  = 0
	messagePageIndex = 1

	// comboItemEnabled is the Qt::ItemIsSelectable|Qt::ItemIsEnabled
	// flag set used with the item enabled role (Qt::UserRole - 1) to
	// disable individual combo box entries.
	comboItemEnabled = 33

	projectURL = "https://github.com/simosako/ejquick"
)

// mainWindow owns the widgets of the search UI and reflects controller
// views into them.
type mainWindow struct {
	window *qt.QMainWindow

	dictCombo  *qt.QComboBox
	searchEdit *qt.QLineEdit
	countLabel *qt.QLabel

	resultList *qt.QListView
	listModel  *qt.QStringListModel
	splitter   *qt.QSplitter

	stack         *qt.QStackedWidget
	headwordLabel *qt.QLabel
	bodyEdit      *qt.QPlainTextEdit
	messageLabel  *qt.QLabel
	openLogButton *qt.QPushButton

	// Actions shared by menus, shortcuts, and context menus (design
	// D18/D19).
	copyAction        *qt.QAction
	switchAction      *qt.QAction
	focusSearchAction *qt.QAction
	clearSearchAction *qt.QAction
	quitAction        *qt.QAction
	shortcutsAction   *qt.QAction
	aboutAction       *qt.QAction
	openLogAction     *qt.QAction
	dictActions       map[dictionary.Type]*qt.QAction

	controller *Controller
	logger     *logging.Logger
	icon       *qt.QIcon
	settings   *qt.QSettings
	dispatcher Dispatcher

	// preeditActive tracks an in-progress IME composition in the search
	// field; custom key handling is suspended while it is set (design
	// D12).
	preeditActive bool
	// updating guards programmatic widget changes against re-entrant
	// signal handlers (design D10/D19).
	updating bool
	// syncingSelection guards programmatic list selection changes.
	syncingSelection bool

	// Render cache to avoid needless widget churn.
	renderedHeadwords []string
	haveRenderedList  bool
	lastDetailID      int64
}

// windowUI carries the window's collaborators. A nil settings value
// disables persistence (tests).
type windowUI struct {
	controller *Controller
	logger     *logging.Logger
	icon       *qt.QIcon
	settings   *qt.QSettings
	dispatcher Dispatcher
}

// newMainWindow builds the two-pane search window (design D3/D11) and
// renders the controller's initial view.
func newMainWindow(ui windowUI) *mainWindow {
	w := &mainWindow{
		controller:   ui.controller,
		logger:       ui.logger,
		icon:         ui.icon,
		settings:     ui.settings,
		dispatcher:   ui.dispatcher,
		dictActions:  map[dictionary.Type]*qt.QAction{},
		lastDetailID: -1,
	}

	w.window = qt.NewQMainWindow2()
	w.window.QWidget.SetWindowTitle("EJQuick")
	w.window.QWidget.Resize(initialWidth, initialHeight)
	w.window.QWidget.SetMinimumSize2(minimumWidth, minimumHeight)
	if w.icon != nil && !w.icon.IsNull() {
		w.window.QWidget.SetWindowIcon(w.icon)
	}

	w.buildCentral()
	w.buildActions()
	w.buildMenuBar()
	w.wire()
	w.restoreSettings()
	w.render(w.controller.View())

	// The search field has focus from startup (design D12/D16).
	w.searchEdit.QWidget.SetFocus()

	return w
}

// buildCentral creates the header row and the two panes.
func (w *mainWindow) buildCentral() {
	central := qt.NewQWidget2()
	outer := qt.NewQVBoxLayout2()
	central.SetLayout(outer.QLayout)

	// Header row: dictionary selector, query field, count label (design
	// D9/D17).
	row := qt.NewQHBoxLayout2()
	outer.AddLayout(row.QLayout)
	w.dictCombo = qt.NewQComboBox2()
	for _, dt := range dictionary.All {
		w.dictCombo.AddItem(DictionaryLabel(dt))
	}
	row.AddWidget(w.dictCombo.QWidget)
	w.searchEdit = qt.NewQLineEdit2()
	w.searchEdit.SetPlaceholderText("Search")
	row.AddWidget2(w.searchEdit.QWidget, 1)
	w.countLabel = qt.NewQLabel3("")
	w.countLabel.SetMinimumWidth(
		w.countLabel.QWidget.FontMetrics().HorizontalAdvance(countWidestText) + 8)
	row.AddWidget(w.countLabel.QWidget)

	// Left pane: result list backed by a string list model (design D11).
	w.listModel = qt.NewQStringListModel2(nil)
	w.resultList = qt.NewQListView2()
	w.resultList.SetModel(w.listModel.QAbstractItemModel)
	w.resultList.SetEditTriggers(qt.QAbstractItemView__NoEditTriggers)
	w.resultList.SetSelectionMode(qt.QAbstractItemView__SingleSelection)
	w.resultList.SetSelectionBehavior(qt.QAbstractItemView__SelectRows)
	// One line per row with end elision (design D11).
	w.resultList.SetWordWrap(false)
	w.resultList.QWidget.SetMinimumWidth(leftMinWidth)

	// Right pane: stacked detail/message pages (design D15).
	w.stack = qt.NewQStackedWidget2()

	detailPage := qt.NewQWidget2()
	detailLayout := qt.NewQVBoxLayout2()
	detailPage.SetLayout(detailLayout.QLayout)
	w.headwordLabel = qt.NewQLabel3("")
	w.headwordLabel.SetWordWrap(true)
	// Headword is selectable for standard text copy (design D18).
	w.headwordLabel.SetTextInteractionFlags(qt.TextSelectableByMouse)
	w.headwordLabel.SetContextMenuPolicy(qt.CustomContextMenu)
	detailLayout.AddWidget(w.headwordLabel.QWidget)
	w.bodyEdit = qt.NewQPlainTextEdit2()
	w.bodyEdit.SetReadOnly(true)
	w.bodyEdit.SetUndoRedoEnabled(false)
	w.bodyEdit.QWidget.SetMinimumWidth(rightMinWidth)
	detailLayout.AddWidget2(w.bodyEdit.QWidget, 1)
	w.stack.AddWidget(detailPage)

	messagePage := qt.NewQWidget2()
	messageLayout := qt.NewQVBoxLayout2()
	messagePage.SetLayout(messageLayout.QLayout)
	w.messageLabel = qt.NewQLabel3("")
	w.messageLabel.SetWordWrap(true)
	w.messageLabel.SetAlignment(qt.AlignTop | qt.AlignLeft)
	messageLayout.AddWidget2(w.messageLabel.QWidget, 1)
	w.openLogButton = qt.NewQPushButton(messagePage)
	w.openLogButton.SetText("Open Log")
	messageLayout.AddWidget(w.openLogButton.QWidget)
	w.stack.AddWidget(messagePage)

	w.splitter = qt.NewQSplitter3(qt.Horizontal)
	w.splitter.AddWidget(w.resultList.QWidget)
	w.splitter.AddWidget(w.stack.QWidget)
	w.stack.SetMinimumWidth(rightMinWidth)
	w.splitter.SetStretchFactor(0, 1)
	w.splitter.SetStretchFactor(1, 2)
	w.splitter.SetSizes([]int{initialWidth / 3, initialWidth * 2 / 3})
	outer.AddWidget2(w.splitter.QWidget, 1)

	w.window.SetCentralWidget(central)
}

// buildActions creates the shared actions of the fixed keymap (design
// D13/D18/D19). Handlers no-op while an IME composition is in progress
// so keys are never stolen from the input method (design D12).
func (w *mainWindow) buildActions() {
	parent := w.window.QWidget.QObject

	w.copyAction = qt.NewQAction5(keyDef(KeyCopyEntry).MenuText, parent)
	w.copyAction.SetShortcut(qt.NewQKeySequence2("Ctrl+Shift+C"))
	w.copyAction.OnTriggered(func() { w.copyEntry() })

	w.focusSearchAction = qt.NewQAction5(keyDef(KeyFocusSearch).MenuText, parent)
	w.focusSearchAction.SetShortcut(qt.NewQKeySequence2("Ctrl+L"))
	w.focusSearchAction.OnTriggered(func() { w.focusSearch() })

	w.clearSearchAction = qt.NewQAction5(keyDef(KeyClearSearch).MenuText, parent)
	w.clearSearchAction.SetShortcut(qt.NewQKeySequence2("Escape"))
	w.clearSearchAction.OnTriggered(func() { w.clearSearch() })

	w.switchAction = qt.NewQAction5(keyDef(KeySwitchDictionary).MenuText, parent)
	w.switchAction.SetShortcut(qt.NewQKeySequence2("Ctrl+Tab"))
	w.switchAction.OnTriggered(func() { w.switchDictionary() })

	w.quitAction = qt.NewQAction5(keyDef(KeyQuit).MenuText, parent)
	// Quit uses Qt's platform-standard sequence (Ctrl+Q / Cmd+Q).
	w.quitAction.SetShortcut(qt.NewQKeySequence6(qt.QKeySequence__Quit))
	w.quitAction.SetMenuRole(qt.QAction__QuitRole)
	w.quitAction.OnTriggered(func() { w.window.QWidget.Close() })

	w.shortcutsAction = qt.NewQAction5("&Keyboard Shortcuts", parent)
	w.shortcutsAction.OnTriggered(func() { w.showShortcuts() })

	w.openLogAction = qt.NewQAction5("&Open Log", parent)
	w.openLogAction.OnTriggered(func() { w.openLog() })

	w.aboutAction = qt.NewQAction5("&About EJQuick", parent)
	w.aboutAction.SetMenuRole(qt.QAction__AboutRole)
	w.aboutAction.OnTriggered(func() { w.showAbout() })

	group := qt.NewQActionGroup(parent)
	group.SetExclusive(true)
	for _, dt := range dictionary.All {
		dt := dt
		act := qt.NewQAction5(dictionaryMenuText(dt), parent)
		act.SetCheckable(true)
		group.AddAction(act)
		act.OnTriggered(func() { w.switchDictionaryTo(dt) })
		w.dictActions[dt] = act
	}
}

// buildMenuBar lays out the standard menu bar (design D19). The File
// menu gains "Build Dictionary Database..." with the builder milestone
// (design D5/D34).
func (w *mainWindow) buildMenuBar() {
	bar := w.window.MenuBar()

	fileMenu := bar.AddMenuWithTitle("&File")
	fileMenu.QWidget.AddAction(w.quitAction)

	editMenu := bar.AddMenuWithTitle("&Edit")
	editMenu.QWidget.AddAction(w.copyAction)
	editMenu.AddSeparator()
	editMenu.QWidget.AddAction(w.focusSearchAction)
	editMenu.QWidget.AddAction(w.clearSearchAction)

	dictMenu := bar.AddMenuWithTitle("&Dictionary")
	dictMenu.QWidget.AddAction(w.dictActions[dictionary.Eiwa])
	dictMenu.QWidget.AddAction(w.dictActions[dictionary.Waei])

	helpMenu := bar.AddMenuWithTitle("&Help")
	helpMenu.QWidget.AddAction(w.shortcutsAction)
	helpMenu.QWidget.AddAction(w.openLogAction)
	helpMenu.AddSeparator()
	helpMenu.QWidget.AddAction(w.aboutAction)
}

// dictionaryMenuText returns the Dictionary menu label with a mnemonic.
func dictionaryMenuText(dt dictionary.Type) string {
	switch dt {
	case dictionary.Eiwa:
		return "&English–Japanese"
	case dictionary.Waei:
		return "&Japanese–English"
	}
	return DictionaryLabel(dt)
}

// wire connects widget signals to the controller.
func (w *mainWindow) wire() {
	// Incremental search: every user edit starts a new request (design
	// D14, no debounce).
	w.searchEdit.OnTextEdited(func(text string) {
		w.controller.SetQuery(text)
		w.render(w.controller.View())
	})

	// Input-centric navigation (design D12/D13). Custom handling applies
	// only without an IME composition; the base class always runs.
	w.searchEdit.OnKeyPressEvent(func(super func(*qt.QKeyEvent), ev *qt.QKeyEvent) {
		if !w.preeditActive {
			key := ev.Key()
			ctrl := ev.Modifiers()&qt.ControlModifier != 0
			switch {
			case key == int(qt.Key_Up) || (ctrl && key == int(qt.Key_P)):
				w.moveSelection(-1)
				return
			case key == int(qt.Key_Down) || (ctrl && key == int(qt.Key_N)):
				w.moveSelection(1)
				return
			case key == int(qt.Key_PageUp):
				w.scrollDetailPage(-1)
				return
			case key == int(qt.Key_PageDown):
				w.scrollDetailPage(1)
				return
			}
		}
		super(ev)
	})
	w.searchEdit.OnInputMethodEvent(func(super func(*qt.QInputMethodEvent), ev *qt.QInputMethodEvent) {
		super(ev)
		w.preeditActive = ev.PreeditString() != ""
	})

	// Dictionary switching through the combo box (design D9/D10).
	w.dictCombo.OnCurrentIndexChanged(func(index int) {
		if w.updating || index < 0 || index >= len(dictionary.All) {
			return
		}
		w.switchDictionaryTo(dictionary.All[index])
	})

	// Selection changes from the list (mouse or focus traversal).
	if sel := w.resultList.SelectionModel(); sel != nil {
		sel.OnCurrentRowChanged(func(current, previous *qt.QModelIndex) {
			if w.syncingSelection || current == nil || !current.IsValid() {
				return
			}
			row := current.Row()
			if row < 0 {
				return
			}
			w.controller.SelectRow(row)
			w.render(w.controller.View())
		})
	}

	// Context menus offering Copy Entire Entry alongside standard text
	// operations (design D18).
	w.bodyEdit.OnContextMenuEvent(func(super func(*qt.QContextMenuEvent), ev *qt.QContextMenuEvent) {
		menu := w.bodyEdit.CreateStandardContextMenuWithPosition(ev.Pos())
		if menu != nil {
			menu.AddSeparator()
			menu.QWidget.AddAction(w.copyAction)
			menu.ExecWithPos(ev.GlobalPos())
		}
	})
	w.headwordLabel.OnCustomContextMenuRequested(func(pos *qt.QPoint) {
		menu := qt.NewQMenu(w.headwordLabel.QWidget)
		menu.QWidget.AddAction(w.copyAction)
		menu.ExecWithPos(w.headwordLabel.QWidget.MapToGlobalWithQPoint(pos))
	})

	w.openLogButton.OnClicked(func() { w.openLog() })

	// Shutdown (design D38, M2 scope: search workers only): persist the
	// window state, then stop accepting new events and cancel work. The
	// controller finishes after the event loop exits.
	w.window.OnCloseEvent(func(super func(*qt.QCloseEvent), ev *qt.QCloseEvent) {
		w.saveSettings()
		w.controller.BeginShutdown()
		if d, ok := w.dispatcher.(*qtDispatcher); ok {
			d.Close()
		}
		super(ev)
	})
}

// render reflects a controller view into the widgets. It is idempotent
// and cheap enough to call for every state change.
func (w *mainWindow) render(v View) {
	// Dictionary controls (design D9/D39/D45).
	w.updating = true
	w.dictCombo.QWidget.SetEnabled(v.DictionaryEnabled)
	for i, item := range v.DictionaryItems {
		flags := 0
		if item.Enabled {
			flags = comboItemEnabled
		}
		// The enabled role (Qt::UserRole - 1) disables individual items;
		// the tooltip doubles as the accessible description of the
		// canonical reason (design D39/D45).
		w.dictCombo.SetItemData2(i, qt.NewQVariant4(flags), int(qt.UserRole)-1)
		w.dictCombo.SetItemData2(i, qt.NewQVariant11(item.Tooltip), int(qt.ToolTipRole))
		w.dictCombo.SetItemData2(i, qt.NewQVariant11(item.Tooltip), int(qt.AccessibleDescriptionRole))
		if act := w.dictActions[item.Type]; act != nil {
			act.SetEnabled(item.Enabled)
			act.SetChecked(item.Type == v.Dictionary)
		}
		if item.Type == v.Dictionary && w.dictCombo.CurrentIndex() != i {
			w.dictCombo.SetCurrentIndex(i)
		}
	}
	w.updating = false

	// Query field: never rewritten during an IME composition (design
	// D12).
	w.searchEdit.QWidget.SetEnabled(v.SearchEnabled)
	if !w.preeditActive && w.searchEdit.Text() != v.Query {
		w.searchEdit.SetText(v.Query)
	}

	// Count label (design D17).
	w.countLabel.SetText(v.CountText)
	if v.CountText == "" {
		w.countLabel.QWidget.SetAccessibleName("")
	} else {
		w.countLabel.QWidget.SetAccessibleName(
			fmt.Sprintf("Search results shown: %d", len(v.Headwords)))
	}

	// Result list: the model is replaced only when the result set
	// actually changed (design D11).
	if !w.haveRenderedList || !equalStrings(w.renderedHeadwords, v.Headwords) {
		w.listModel.SetStringList(v.Headwords)
		w.renderedHeadwords = append([]string(nil), v.Headwords...)
		w.haveRenderedList = true
	}
	w.setCurrentRow(v.SelectedRow)

	// Right pane (design D15).
	if v.Page == pageDetail && v.Entry != nil {
		if w.stack.CurrentIndex() != detailPageIndex {
			w.stack.SetCurrentIndex(detailPageIndex)
		}
		if w.headwordLabel.Text() != v.Entry.Headword {
			w.headwordLabel.SetText(v.Entry.Headword)
		}
		if w.lastDetailID != v.Entry.ID {
			w.lastDetailID = v.Entry.ID
			w.bodyEdit.SetPlainText(v.Entry.Body)
			// New entry: detail reading starts at the top (design D11).
			if bar := w.bodyEdit.VerticalScrollBar(); bar != nil {
				bar.SetValue(0)
			}
		}
	} else {
		if w.stack.CurrentIndex() != messagePageIndex {
			w.stack.SetCurrentIndex(messagePageIndex)
		}
		w.messageLabel.SetText(strings.Join(v.Message.Lines, "\n"))
		w.openLogButton.QWidget.SetVisible(v.Message.ShowOpenLog)
	}

	w.copyAction.SetEnabled(v.CopyEnabled)
	w.switchAction.SetEnabled(v.SwitchEnabled)
}

// setCurrentRow moves the list selection without stealing focus (design
// D12): selection changes go through the selection model only.
func (w *mainWindow) setCurrentRow(row int) {
	sel := w.resultList.SelectionModel()
	if sel == nil {
		return
	}
	w.syncingSelection = true
	defer func() { w.syncingSelection = false }()
	if row < 0 {
		sel.ClearSelection()
		sel.ClearCurrentIndex()
		return
	}
	// miqt dereferences the parent pointer, so pass a real (invalid)
	// root index instead of nil.
	idx := w.listModel.Index(row, 0, qt.NewQModelIndex())
	if idx != nil && idx.IsValid() {
		sel.SetCurrentIndex(idx, qt.QItemSelectionModel__ClearAndSelect)
	}
}

// moveSelection moves the list selection by delta rows.
func (w *mainWindow) moveSelection(delta int) {
	if w.controller.MoveSelection(delta) {
		w.render(w.controller.View())
	}
}

// scrollDetailPage scrolls the detail body by one page (design D13).
func (w *mainWindow) scrollDetailPage(dir int) {
	bar := w.bodyEdit.VerticalScrollBar()
	if bar == nil {
		return
	}
	if dir < 0 {
		bar.TriggerAction(qt.QAbstractSlider__SliderPageStepSub)
	} else {
		bar.TriggerAction(qt.QAbstractSlider__SliderPageStepAdd)
	}
}

// focusSearch implements Ctrl+L (design D13).
func (w *mainWindow) focusSearch() {
	if w.preeditActive {
		return
	}
	w.searchEdit.QWidget.SetFocus()
	w.searchEdit.SelectAll()
}

// clearSearch implements Escape (design D13).
func (w *mainWindow) clearSearch() {
	if w.preeditActive {
		return
	}
	w.controller.ClearSearch()
	w.render(w.controller.View())
	w.searchEdit.QWidget.SetFocus()
}

// switchDictionary implements Ctrl+Tab (design D10/D13): the query and
// search state are cleared and focus returns to the search field.
func (w *mainWindow) switchDictionary() {
	if w.preeditActive {
		return
	}
	w.controller.SwitchDictionary()
	w.render(w.controller.View())
	w.searchEdit.QWidget.SetFocus()
}

// switchDictionaryTo switches to a specific dictionary (combo box and
// Dictionary menu, design D9/D10).
func (w *mainWindow) switchDictionaryTo(dt dictionary.Type) {
	if w.preeditActive {
		return
	}
	if dt == w.controller.Dictionary() {
		return
	}
	w.controller.SwitchToDictionary(dt)
	w.render(w.controller.View())
	w.searchEdit.QWidget.SetFocus()
}

// copyEntry implements Copy Entire Entry (design D18): the clipboard
// receives headword + "\n" + body, from the controller's entry, never
// from the rendered widgets.
func (w *mainWindow) copyEntry() {
	if w.preeditActive {
		return
	}
	text, ok := w.controller.CopySelection()
	if !ok {
		return
	}
	qt.QGuiApplication_Clipboard().SetText(text)
}

// openLog opens the shared log file in the system text viewer (design
// D24).
func (w *mainWindow) openLog() {
	path, ok := w.logger.Path()
	if !ok {
		return
	}
	openExternalPath(path, w.window.QWidget)
}

// showAbout presents the compact About dialog (design D42/D43).
func (w *mainWindow) showAbout() {
	dlg := qt.NewQDialog(w.window.QWidget)
	dlg.SetWindowTitle("About EJQuick")
	layout := qt.NewQVBoxLayout2()
	dlg.SetLayout(layout.QLayout)

	if w.icon != nil && !w.icon.IsNull() {
		iconLabel := qt.NewQLabel(dlg.QWidget)
		iconLabel.SetPixmap(w.icon.PixmapWithExtent(64))
		layout.AddWidget(iconLabel.QWidget)
	}
	for _, text := range []string{
		"EJQuick",
		"Version " + buildinfo.Version,
		"Copyright © 2026 EJQuick contributors",
	} {
		line := qt.NewQLabel(dlg.QWidget)
		line.SetText(text)
		layout.AddWidget(line.QWidget)
	}
	url := qt.NewQLabel(dlg.QWidget)
	url.SetText(fmt.Sprintf(`<a href="%s">%s</a>`, projectURL, projectURL))
	// The URL is opened only on explicit activation; the dialog itself
	// never performs network access (design D43).
	url.SetOpenExternalLinks(true)
	url.SetTextInteractionFlags(qt.LinksAccessibleByMouse | qt.TextSelectableByMouse)
	layout.AddWidget(url.QWidget)

	layout.AddStretch()
	closeButton := qt.NewQPushButton(dlg.QWidget)
	closeButton.SetText("Close")
	closeButton.OnClicked(dlg.Accept)
	layout.AddWidget(closeButton.QWidget)

	dlg.Exec()
}

// showShortcuts presents the Keyboard Shortcuts help dialog (design
// D19); the content is generated from the same keymap table as the
// implemented shortcuts (design D13).
func (w *mainWindow) showShortcuts() {
	dlg := qt.NewQDialog(w.window.QWidget)
	dlg.SetWindowTitle("Keyboard Shortcuts")
	layout := qt.NewQVBoxLayout2()
	dlg.SetLayout(layout.QLayout)

	lines := make([]string, 0, len(Keymap))
	for _, d := range Keymap {
		lines = append(lines, fmt.Sprintf("%-18s %s", d.Keys, d.Label))
	}
	text := qt.NewQPlainTextEdit(dlg.QWidget)
	text.SetReadOnly(true)
	text.SetPlainText(strings.Join(lines, "\n"))
	layout.AddWidget2(text.QWidget, 1)

	closeButton := qt.NewQPushButton(dlg.QWidget)
	closeButton.SetText("Close")
	closeButton.OnClicked(dlg.Accept)
	layout.AddWidget(closeButton.QWidget)

	dlg.QWidget.Resize(460, 340)
	dlg.Exec()
}

// saveSettings persists the window geometry and splitter state (design
// D16).
func (w *mainWindow) saveSettings() {
	if w.settings == nil {
		return
	}
	w.settings.SetValue(*qt.NewQAnyStringView3(keyStateVersion), qt.NewQVariant4(settingsStateVersion))
	w.settings.SetValue(*qt.NewQAnyStringView3(keyGeometry), qt.NewQVariant12(w.window.QWidget.SaveGeometry()))
	w.settings.SetValue(*qt.NewQAnyStringView3(keySplitter), qt.NewQVariant12(w.splitter.SaveState()))
	w.settings.Sync()
}

// restoreSettings reloads persisted GUI state. Invalid or missing values
// leave the D16 defaults in place; sizes below the minimum fall back to
// the default geometry.
func (w *mainWindow) restoreSettings() {
	if w.settings == nil {
		return
	}
	if ver := w.settings.ValueWithKey(*qt.NewQAnyStringView3(keyStateVersion)); !ver.IsValid() || ver.ToInt() != settingsStateVersion {
		return
	}
	if geo := w.settings.ValueWithKey(*qt.NewQAnyStringView3(keyGeometry)); geo.IsValid() {
		w.window.QWidget.RestoreGeometry(geo.ToByteArray())
		if w.window.QWidget.Width() < minimumWidth || w.window.QWidget.Height() < minimumHeight {
			w.window.QWidget.Resize(initialWidth, initialHeight)
		}
	}
	if st := w.settings.ValueWithKey(*qt.NewQAnyStringView3(keySplitter)); st.IsValid() {
		w.splitter.RestoreState(st.ToByteArray())
	}
}

// equalStrings compares two string slices.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
