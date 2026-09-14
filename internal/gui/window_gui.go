//go:build gui

package gui

import (
	"fmt"
	"runtime"
	"sync"

	qt "github.com/mappu/miqt/qt6"
	"github.com/mappu/miqt/qt6/mainthread"

	"github.com/simosako/ejquick/internal/buildinfo"
	"github.com/simosako/ejquick/internal/config"
	"github.com/simosako/ejquick/internal/dictionary"
	"github.com/simosako/ejquick/internal/gui/controller"
	"github.com/simosako/ejquick/internal/gui/startup"
	"github.com/simosako/ejquick/internal/logging"
	"github.com/simosako/ejquick/internal/normalize"
)

const settingsVersion = 1

type mainWindow struct {
	main       *qt.QMainWindow
	central    *qt.QWidget
	combo      *qt.QComboBox
	comboModel *qt.QStandardItemModel
	query      *qt.QLineEdit
	count      *qt.QLabel
	list       *qt.QListView
	listModel  *qt.QStringListModel
	splitter   *qt.QSplitter
	stack      *qt.QStackedWidget
	detailPage *qt.QWidget
	headword   *qt.QLabel
	body       *qt.QPlainTextEdit
	message    *qt.QLabel
	messageLog *qt.QPushButton

	buildAction      *qt.QAction
	quitAction       *qt.QAction
	copyAction       *qt.QAction
	focusAction      *qt.QAction
	clearAction      *qt.QAction
	dictionaryAction map[dictionary.Type]*qt.QAction
	openLogAction    *qt.QAction

	controller *controller.Controller
	dispatcher *qtDispatcher
	databases  *startup.Databases
	logger     *logging.Logger
	settings   *qt.QSettings
	icon       *qt.QIcon

	preedit      bool
	syncing      bool
	shutdownOnce sync.Once
}

func newMainWindow(cfg *config.Config, databases *startup.Databases, logger *logging.Logger, icon *qt.QIcon) *mainWindow {
	w := &mainWindow{
		main:             qt.NewQMainWindow2(),
		databases:        databases,
		logger:           logger,
		icon:             icon,
		dictionaryAction: make(map[dictionary.Type]*qt.QAction, len(dictionary.All)),
	}
	w.main.SetWindowTitle(productName)
	w.main.SetMinimumSize2(720, 480)
	if icon != nil && !icon.IsNull() {
		w.main.SetWindowIcon(icon)
	}
	w.settings = qt.NewQSettings8(applicationID, "ejquick", w.main.QObject)

	w.createActions()
	w.createMenus()
	w.createContent()
	w.connectSignals()
	w.configureController(cfg)
	w.restoreWindowState()
	w.renderInitialState()
	return w
}

func (w *mainWindow) createContent() {
	w.central = qt.NewQWidget(w.main.QWidget)
	root := qt.NewQVBoxLayout2()
	root.SetContentsMargins(8, 8, 8, 8)
	root.SetSpacing(8)
	w.central.SetLayout(root.QLayout)
	w.main.SetCentralWidget(w.central)

	header := qt.NewQHBoxLayout2()
	header.SetSpacing(8)
	root.AddLayout(header.QLayout)

	w.combo = qt.NewQComboBox(w.central)
	w.combo.SetAccessibleName("Dictionary")
	w.combo.SetSizeAdjustPolicy(qt.QComboBox__AdjustToContents)
	w.comboModel = qt.NewQStandardItemModel3(w.main.QObject)
	for _, dict := range dictionary.All {
		item := qt.NewQStandardItem2(controller.DictionaryName(dict))
		status := w.databases.Statuses[dict]
		item.SetEnabled(status.Available)
		item.SetEditable(false)
		if !status.Available {
			unavailable := controller.MessageForUnavailable(dict, status.Category)
			item.SetToolTip(unavailable.Text)
			item.SetAccessibleDescription(unavailable.Text)
		}
		w.comboModel.AppendRowWithItem(item)
	}
	w.combo.SetModel(w.comboModel.QAbstractItemModel)
	header.AddWidget(w.combo.QWidget)

	w.query = qt.NewQLineEdit(w.central)
	w.query.SetAccessibleName("Search")
	w.query.SetPlaceholderText("Search headwords")
	w.query.SetClearButtonEnabled(true)
	header.AddWidget2(w.query.QWidget, 1)

	w.count = qt.NewQLabel5("", w.central)
	w.count.SetMinimumWidth(112)
	w.count.SetAlignment(qt.AlignRight | qt.AlignVCenter)
	header.AddWidget(w.count.QWidget)

	w.splitter = qt.NewQSplitter4(qt.Horizontal, w.central)
	w.splitter.SetChildrenCollapsible(false)
	root.AddWidget2(w.splitter.QWidget, 1)

	w.list = qt.NewQListView(w.splitter.QWidget)
	w.list.SetMinimumWidth(240)
	w.list.SetAccessibleName("Search results")
	w.list.SetUniformItemSizes(true)
	w.list.SetSelectionMode(qt.QAbstractItemView__SingleSelection)
	w.list.SetSelectionBehavior(qt.QAbstractItemView__SelectRows)
	w.list.SetEditTriggers(qt.QAbstractItemView__NoEditTriggers)
	w.list.SetTextElideMode(qt.ElideRight)
	w.listModel = qt.NewQStringListModel3(w.main.QObject)
	w.list.SetModel(w.listModel.QAbstractItemModel)
	w.splitter.AddWidget(w.list.QWidget)

	w.stack = qt.NewQStackedWidget(w.splitter.QWidget)
	w.stack.SetMinimumWidth(320)
	w.splitter.AddWidget(w.stack.QWidget)
	w.splitter.SetStretchFactor(0, 1)
	w.splitter.SetStretchFactor(1, 2)

	w.detailPage = qt.NewQWidget(w.stack.QWidget)
	detailLayout := qt.NewQVBoxLayout2()
	detailLayout.SetContentsMargins(12, 8, 8, 8)
	detailLayout.SetSpacing(8)
	w.detailPage.SetLayout(detailLayout.QLayout)
	w.headword = qt.NewQLabel5("", w.detailPage)
	w.headword.SetWordWrap(true)
	w.headword.SetTextInteractionFlags(qt.TextSelectableByMouse | qt.TextSelectableByKeyboard)
	w.headword.SetAccessibleName("Selected headword")
	detailLayout.AddWidget(w.headword.QWidget)
	w.body = qt.NewQPlainTextEdit(w.detailPage)
	w.body.SetReadOnly(true)
	w.body.SetUndoRedoEnabled(false)
	w.body.SetLineWrapMode(qt.QPlainTextEdit__WidgetWidth)
	w.body.SetAccessibleName("Dictionary entry")
	w.body.SetContextMenuPolicy(qt.CustomContextMenu)
	detailLayout.AddWidget2(w.body.QWidget, 1)
	w.stack.AddWidget(w.detailPage)

	messagePage := qt.NewQWidget(w.stack.QWidget)
	messageLayout := qt.NewQVBoxLayout2()
	messageLayout.SetContentsMargins(24, 24, 24, 24)
	messageLayout.AddStretch()
	w.message = qt.NewQLabel5("", messagePage)
	w.message.SetWordWrap(true)
	w.message.SetAlignment(qt.AlignCenter)
	w.message.SetTextInteractionFlags(qt.TextSelectableByMouse | qt.TextSelectableByKeyboard)
	w.message.SetAccessibleName("Status")
	messageLayout.AddWidget(w.message.QWidget)
	w.messageLog = qt.NewQPushButton5("Open &Log", messagePage)
	messageLayout.AddWidget3(w.messageLog.QWidget, 0, qt.AlignHCenter)
	messageLayout.AddStretch()
	messagePage.SetLayout(messageLayout.QLayout)
	w.stack.AddWidget(messagePage)

	w.splitter.SetSizes([]int{320, 640})
}

func (w *mainWindow) configureController(cfg *config.Config) {
	if len(w.databases.Services) == 0 {
		return
	}
	services := make(map[dictionary.Type]controller.Searcher, len(w.databases.Services))
	for dict, service := range w.databases.Services {
		services[dict] = service
	}
	w.dispatcher = newQtDispatcher(func(recovered any) {
		w.handleCallbackPanic(recovered)
	})
	w.controller = controller.New(w.databases.Initial, services, cfg.Search.MaxResults, w.dispatcher)
	w.dispatcher.setHandler(w.applySearchEvent)
}

func (w *mainWindow) createActions() {
	parent := w.main.QObject
	w.buildAction = qt.NewQAction5("&Build Dictionary Database...", parent)
	// The process supervisor is added in the next implementation stage.
	w.buildAction.SetEnabled(false)

	w.quitAction = qt.NewQAction5("&Quit", parent)
	w.quitAction.SetMenuRole(qt.QAction__QuitRole)
	w.quitAction.SetShortcutsWithShortcuts(qt.QKeySequence__Quit)

	w.copyAction = qt.NewQAction5("Copy &Entire Entry", parent)
	setActionShortcut(w.copyAction, "Ctrl+Shift+C")
	w.copyAction.SetEnabled(false)

	w.focusAction = qt.NewQAction5("&Focus Search", parent)
	setActionShortcut(w.focusAction, "Ctrl+L")

	w.clearAction = qt.NewQAction5("&Clear Search", parent)
	setActionShortcut(w.clearAction, "Esc")
	w.clearAction.SetEnabled(false)

	group := qt.NewQActionGroup(parent)
	group.SetExclusive(true)
	for _, dict := range dictionary.All {
		action := qt.NewQAction5(controller.DictionaryName(dict), parent)
		action.SetCheckable(true)
		status := w.databases.Statuses[dict]
		action.SetEnabled(status.Available)
		if !status.Available {
			action.SetToolTip(controller.MessageForUnavailable(dict, status.Category).Text)
		}
		group.AddAction(action)
		w.dictionaryAction[dict] = action
	}

	w.openLogAction = qt.NewQAction5("Open &Log", parent)
	_, logAvailable := w.logger.Path()
	w.openLogAction.SetEnabled(logAvailable)
}

func (w *mainWindow) createMenus() {
	menuBar := w.main.MenuBar()
	fileMenu := menuBar.AddMenuWithTitle("&File")
	fileMenu.AddAction(w.buildAction)
	fileMenu.AddSeparator()
	fileMenu.AddAction(w.quitAction)

	editMenu := menuBar.AddMenuWithTitle("&Edit")
	editMenu.AddAction(w.copyAction)
	editMenu.AddSeparator()
	editMenu.AddAction(w.focusAction)
	editMenu.AddAction(w.clearAction)

	dictionaryMenu := menuBar.AddMenuWithTitle("&Dictionary")
	for _, dict := range dictionary.All {
		dictionaryMenu.AddAction(w.dictionaryAction[dict])
	}

	helpMenu := menuBar.AddMenuWithTitle("&Help")
	shortcuts := qt.NewQAction5("&Keyboard Shortcuts", w.main.QObject)
	helpMenu.AddAction(shortcuts)
	helpMenu.AddAction(w.openLogAction)
	helpMenu.AddSeparator()
	about := qt.NewQAction5("&About EJQuick", w.main.QObject)
	about.SetMenuRole(qt.QAction__AboutRole)
	helpMenu.AddAction(about)

	shortcuts.OnTriggered(w.safeCallback(w.showKeyboardShortcuts))
	about.OnTriggered(w.safeCallback(w.showAbout))
}

func (w *mainWindow) connectSignals() {
	w.quitAction.OnTriggered(w.safeCallback(func() { w.beginShutdown(0) }))
	w.copyAction.OnTriggered(w.safeCallback(w.copyEntireEntry))
	w.focusAction.OnTriggered(w.safeCallback(func() {
		if w.preedit || w.controller == nil {
			return
		}
		w.query.SetFocus()
		w.query.SelectAll()
	}))
	w.clearAction.OnTriggered(w.safeCallback(func() {
		if !w.preedit {
			w.clearSearch()
		}
	}))
	w.openLogAction.OnTriggered(w.safeCallback(w.openLog))
	w.messageLog.OnClicked(w.safeCallback(func() { w.openLogAction.Trigger() }))
	w.body.OnCustomContextMenuRequested(func(position *qt.QPoint) {
		defer w.recoverCallback()
		menu := w.body.CreateStandardContextMenuWithPosition(position)
		defer menu.Delete()
		menu.AddSeparator()
		menu.AddAction(w.copyAction)
		globalPosition := w.body.MapToGlobalWithQPoint(position)
		defer globalPosition.Delete()
		menu.ExecWithPos(globalPosition)
	})

	for _, dict := range dictionary.All {
		dict := dict
		w.dictionaryAction[dict].OnTriggered(w.safeCallback(func() {
			if !w.preedit {
				w.switchDictionary(dict)
			}
		}))
	}

	w.combo.OnCurrentIndexChanged(func(index int) {
		defer w.recoverCallback()
		if w.syncing || w.preedit || index < 0 || index >= len(dictionary.All) {
			return
		}
		w.switchDictionary(dictionary.All[index])
	})
	w.query.OnTextEdited(func(text string) {
		defer w.recoverCallback()
		w.queryEdited(text)
	})
	w.query.OnInputMethodEvent(func(super func(*qt.QInputMethodEvent), event *qt.QInputMethodEvent) {
		defer w.recoverCallback()
		super(event)
		w.preedit = event.PreeditString() != ""
	})
	w.query.OnKeyPressEvent(func(super func(*qt.QKeyEvent), event *qt.QKeyEvent) {
		defer w.recoverCallback()
		if w.preedit || !w.handleSearchKey(event) {
			super(event)
		}
	})
	w.list.SelectionModel().OnCurrentChanged(func(current, _ *qt.QModelIndex) {
		defer w.recoverCallback()
		if w.controller != nil && current.IsValid() && w.controller.SelectRow(current.Row()) {
			w.renderSelection()
		}
	})
	w.main.OnCloseEvent(func(_ func(*qt.QCloseEvent), event *qt.QCloseEvent) {
		defer w.recoverCallback()
		if runtime.GOOS == "darwin" {
			if w.controller != nil {
				w.controller.Clear()
			}
			event.Accept()
			return
		}
		event.Ignore()
		w.beginShutdown(0)
	})
}

func (w *mainWindow) show() {
	w.main.Show()
	if w.controller != nil {
		w.query.SetFocus()
	}
}

func (w *mainWindow) renderInitialState() {
	initial := w.databases.Initial
	w.syncDictionaryControls(initial)
	available := len(w.databases.Services)
	w.combo.SetEnabled(available > 1)
	w.query.SetEnabled(available > 0)
	w.list.SetEnabled(available > 0)
	w.focusAction.SetEnabled(available > 0)
	if w.controller == nil {
		w.renderUnavailableDatabases()
		return
	}
	w.renderSearchState()
}

func (w *mainWindow) queryEdited(text string) {
	if w.controller == nil || !w.controller.SetQuery(text) {
		return
	}
	snapshot := w.controller.Snapshot()
	w.clearAction.SetEnabled(snapshot.Query != "")
	if snapshot.Content != controller.ContentPending && snapshot.Content != controller.ContentResults {
		w.renderSearchState()
	}
}

func (w *mainWindow) applySearchEvent(event controller.Event) {
	if w.controller == nil || !w.controller.Apply(event) {
		return
	}
	snapshot := w.controller.Snapshot()
	if event.Err != nil {
		w.logger.Error("gui search: dictionary=%s request=%d: %v", event.Dictionary, event.RequestID, event.Err)
	} else {
		normalized, _ := normalize.Normalize(event.Dictionary, snapshot.Query)
		w.logger.Debug("gui search: dictionary=%s request=%d query=%q results=%d elapsed=%s",
			event.Dictionary, event.RequestID, normalized, len(event.Entries), event.Elapsed)
	}
	w.renderSearchState()
}

func (w *mainWindow) renderSearchState() {
	if w.controller == nil {
		return
	}
	snapshot := w.controller.Snapshot()
	w.clearAction.SetEnabled(snapshot.Query != "")
	switch snapshot.Content {
	case controller.ContentResults:
		headwords := make([]string, len(snapshot.Entries))
		for i := range snapshot.Entries {
			headwords[i] = snapshot.Entries[i].Headword
		}
		w.listModel.SetStringList(headwords)
		w.setCount(len(headwords))
		w.setCurrentRow(snapshot.Selected)
		w.renderSelection()
	case controller.ContentNoResults:
		w.clearResults()
		w.setMessage("No matching headwords", false)
	case controller.ContentSearchError:
		w.clearResults()
		w.setMessage("Search failed\n\nEdit the query to try again", true)
	case controller.ContentEmpty:
		w.clearResults()
		message := "Enter a search term\n\nUp/Down: Select result"
		if len(w.databases.Services) > 1 {
			message += "    Ctrl+Tab: Switch dictionary"
		}
		w.setMessage(message, false)
	}
}

func (w *mainWindow) renderSelection() {
	if w.controller == nil {
		return
	}
	entry, ok := w.controller.SelectedEntry()
	w.copyAction.SetEnabled(ok)
	if !ok {
		w.headword.Clear()
		w.body.Clear()
		return
	}
	w.headword.SetText(entry.Headword)
	w.body.SetPlainText(entry.Body)
	w.stack.SetCurrentWidget(w.detailPage)
}

func (w *mainWindow) clearResults() {
	w.listModel.SetStringList(nil)
	w.count.SetText("")
	w.count.SetAccessibleName("")
	w.copyAction.SetEnabled(false)
	w.headword.Clear()
	w.body.Clear()
}

func (w *mainWindow) setCount(count int) {
	if count <= 0 {
		w.count.SetText("")
		w.count.SetAccessibleName("")
		return
	}
	if w.controller != nil && count == w.controller.MaxResults() {
		w.count.SetText(fmt.Sprintf("Showing first %d", count))
	} else {
		w.count.SetText(fmt.Sprintf("%d shown", count))
	}
	w.count.SetAccessibleName(fmt.Sprintf("Search results shown: %d", count))
}

func (w *mainWindow) setMessage(text string, showLog bool) {
	w.message.SetText(text)
	w.message.SetAccessibleDescription(text)
	_, logAvailable := w.logger.Path()
	w.messageLog.SetVisible(showLog && logAvailable)
	w.stack.SetCurrentIndex(1)
}

func (w *mainWindow) renderUnavailableDatabases() {
	var messages []string
	showLog := false
	order := []dictionary.Type{w.databases.Initial, w.databases.Initial.Other()}
	for _, dict := range order {
		status := w.databases.Statuses[dict]
		unavailable := controller.MessageForUnavailable(dict, status.Category)
		messages = append(messages, unavailable.Text)
		showLog = showLog || unavailable.OpenLog
	}
	w.setMessage(messages[0]+"\n\n"+messages[1], showLog)
}

func (w *mainWindow) switchDictionary(dict dictionary.Type) {
	if w.controller == nil || !w.controller.SwitchDictionary(dict) {
		current := w.databases.Initial
		if w.controller != nil {
			current = w.controller.Snapshot().Dictionary
		}
		w.syncDictionaryControls(current)
		return
	}
	w.query.Clear()
	w.list.VerticalScrollBar().SetValue(0)
	w.body.VerticalScrollBar().SetValue(0)
	w.syncDictionaryControls(dict)
	w.renderSearchState()
	w.query.SetFocus()
}

func (w *mainWindow) syncDictionaryControls(dict dictionary.Type) {
	w.syncing = true
	defer func() { w.syncing = false }()
	index := 0
	if dict == dictionary.Waei {
		index = 1
	}
	w.combo.SetCurrentIndex(index)
	for _, candidate := range dictionary.All {
		w.dictionaryAction[candidate].SetChecked(candidate == dict)
	}
}

func (w *mainWindow) clearSearch() {
	if w.controller == nil || !w.controller.Clear() {
		return
	}
	w.query.Clear()
	w.list.VerticalScrollBar().SetValue(0)
	w.body.VerticalScrollBar().SetValue(0)
	w.renderSearchState()
	w.query.SetFocus()
}

func (w *mainWindow) handleSearchKey(event *qt.QKeyEvent) bool {
	if w.controller == nil {
		return false
	}
	key := qt.Key(event.Key())
	modifiers := event.Modifiers()
	switch {
	case modifiers == qt.NoModifier && key == qt.Key_Up:
		w.moveSelection(-1)
	case modifiers == qt.NoModifier && key == qt.Key_Down:
		w.moveSelection(1)
	case modifiers == qt.ControlModifier && key == qt.Key_P:
		w.moveSelection(-1)
	case modifiers == qt.ControlModifier && key == qt.Key_N:
		w.moveSelection(1)
	case modifiers == qt.NoModifier && key == qt.Key_PageUp:
		w.scrollDetail(-1)
	case modifiers == qt.NoModifier && key == qt.Key_PageDown:
		w.scrollDetail(1)
	case modifiers == qt.NoModifier && key == qt.Key_Escape:
		w.clearSearch()
	case modifiers == qt.ControlModifier && key == qt.Key_Tab:
		if len(w.databases.Services) > 1 {
			w.switchDictionary(w.controller.Snapshot().Dictionary.Other())
		}
	case modifiers == qt.ControlModifier && key == qt.Key_L:
		w.query.SetFocus()
		w.query.SelectAll()
	case modifiers == qt.ControlModifier|qt.ShiftModifier && key == qt.Key_C:
		w.copyEntireEntry()
	default:
		return false
	}
	event.Accept()
	return true
}

func (w *mainWindow) moveSelection(delta int) {
	if w.controller.MoveSelection(delta) {
		w.setCurrentRow(w.controller.Snapshot().Selected)
		w.renderSelection()
	}
}

func (w *mainWindow) setCurrentRow(row int) {
	if row < 0 {
		w.list.SelectionModel().Clear()
		return
	}
	parent := qt.NewQModelIndex()
	index := w.listModel.Index(row, 0, parent)
	parent.Delete()
	defer index.Delete()
	w.list.SelectionModel().SetCurrentIndex(index,
		qt.QItemSelectionModel__ClearAndSelect|qt.QItemSelectionModel__Rows)
	w.list.ScrollTo(index, qt.QAbstractItemView__EnsureVisible)
}

func (w *mainWindow) scrollDetail(direction int) {
	scrollbar := w.body.VerticalScrollBar()
	scrollbar.SetValue(scrollbar.Value() + direction*scrollbar.PageStep())
}

func (w *mainWindow) copyEntireEntry() {
	if w.preedit || w.controller == nil {
		return
	}
	entry, ok := w.controller.SelectedEntry()
	if !ok {
		return
	}
	qt.QGuiApplication_Clipboard().SetText(entry.Headword + "\n" + entry.Body)
}

func (w *mainWindow) openLog() {
	path, ok := w.logger.Path()
	if !ok {
		return
	}
	if !openLocalPath(path) {
		showOpenPathError("Open Log", path)
	}
}

func (w *mainWindow) showKeyboardShortcuts() {
	const shortcuts = `Up / Ctrl+P        Previous result
Down / Ctrl+N      Next result
PageUp / PageDown  Scroll entry
Ctrl+L             Focus search
Escape             Clear search
Ctrl+Tab            Switch dictionary
Ctrl+Shift+C        Copy entire entry
Ctrl+Q              Quit`
	box := qt.NewQMessageBox3(qt.QMessageBox__Information, "EJQuick Keyboard Shortcuts", shortcuts)
	defer box.Delete()
	box.SetTextFormat(qt.PlainText)
	box.Exec()
}

func (w *mainWindow) showAbout() {
	dialog := qt.NewQDialog(w.main.QWidget)
	defer dialog.Delete()
	dialog.SetWindowTitle("About EJQuick")
	dialog.SetWindowModality(qt.WindowModal)
	layout := qt.NewQVBoxLayout2()
	layout.SetContentsMargins(24, 20, 24, 20)
	layout.SetSpacing(10)
	dialog.SetLayout(layout.QLayout)
	if w.icon != nil && !w.icon.IsNull() {
		iconLabel := qt.NewQLabel(dialog.QWidget)
		pixmap := w.icon.Pixmap2(48, 48)
		iconLabel.SetPixmap(pixmap)
		pixmap.Delete()
		iconLabel.SetAlignment(qt.AlignHCenter)
		layout.AddWidget(iconLabel.QWidget)
	}
	metadata := qt.NewQLabel5(fmt.Sprintf(
		"EJQuick\nVersion %s\nCopyright © 2026 EJQuick contributors\n<a href=\"https://github.com/simosako/ejquick\">https://github.com/simosako/ejquick</a>",
		buildinfo.Version), dialog.QWidget)
	metadata.SetAlignment(qt.AlignCenter)
	metadata.SetTextInteractionFlags(qt.TextSelectableByMouse | qt.TextSelectableByKeyboard | qt.LinksAccessibleByMouse | qt.LinksAccessibleByKeyboard)
	metadata.SetOpenExternalLinks(false)
	metadata.OnLinkActivated(func(link string) {
		defer w.recoverCallback()
		url := qt.NewQUrl3(link)
		defer url.Delete()
		if !qt.QDesktopServices_OpenUrl(url) {
			showOpenPathError("Open Project Website", link)
		}
	})
	layout.AddWidget(metadata.QWidget)
	closeButton := qt.NewQPushButton5("&Close", dialog.QWidget)
	closeButton.SetDefault(true)
	closeButton.OnClicked(func() { dialog.Accept() })
	layout.AddWidget3(closeButton.QWidget, 0, qt.AlignHCenter)
	dialog.Exec()
}

func (w *mainWindow) restoreWindowState() {
	w.main.Resize(960, 640)
	w.splitter.SetSizes([]int{320, 640})
	version := settingsValue(w.settings, "gui/stateVersion")
	if version == nil {
		return
	}
	validVersion := version.IsValid() && version.ToInt() == settingsVersion
	version.Delete()
	if !validVersion {
		return
	}
	if geometry := settingsValue(w.settings, "mainWindow/geometry"); geometry != nil {
		if data := geometry.ToByteArray(); len(data) != 0 {
			w.main.RestoreGeometry(data)
		}
		geometry.Delete()
	}
	if splitter := settingsValue(w.settings, "mainWindow/splitter"); splitter != nil {
		if data := splitter.ToByteArray(); len(data) != 0 {
			w.splitter.RestoreState(data)
		}
		splitter.Delete()
	}
	w.main.SetMinimumSize2(720, 480)
	w.list.SetMinimumWidth(240)
	w.stack.SetMinimumWidth(320)
}

func (w *mainWindow) saveWindowState() {
	settingsSetValue(w.settings, "gui/stateVersion", qt.NewQVariant4(settingsVersion))
	settingsSetValue(w.settings, "mainWindow/geometry", qt.NewQVariant12(w.main.SaveGeometry()))
	settingsSetValue(w.settings, "mainWindow/splitter", qt.NewQVariant12(w.splitter.SaveState()))
	w.settings.Sync()
	if w.settings.Status() != qt.QSettings__NoError {
		w.logger.Error("gui shutdown: save window settings status=%d", w.settings.Status())
	}
}

func (w *mainWindow) beginShutdown(exitCode int) {
	w.shutdownOnce.Do(func() {
		w.saveWindowState()
		w.central.SetEnabled(false)
		w.main.MenuBar().SetEnabled(false)
		if w.dispatcher != nil {
			w.dispatcher.close()
		}
		if w.controller != nil {
			w.controller.BeginClose()
		}
		go func() {
			if w.controller != nil {
				w.controller.Wait()
			}
			if w.dispatcher != nil {
				w.dispatcher.wait()
			}
			mainthread.Start(func() {
				if err := w.databases.Close(); err != nil {
					w.logger.Error("gui shutdown: %v", err)
					exitCode = 2
				}
				w.main.Delete()
				qt.QCoreApplication_ExitWithRetcode(exitCode)
			})
		}()
	})
}

func (w *mainWindow) handleCallbackPanic(recovered any) {
	w.logger.Error("gui callback panic: %v", recovered)
	w.beginShutdown(2)
}

func (w *mainWindow) safeCallback(callback func()) func() {
	return func() {
		defer w.recoverCallback()
		callback()
	}
}

func (w *mainWindow) recoverCallback() {
	if recovered := recover(); recovered != nil {
		w.handleCallbackPanic(recovered)
	}
}

func setActionShortcut(action *qt.QAction, shortcut string) {
	sequence := qt.NewQKeySequence2(shortcut)
	action.SetShortcut(sequence)
	sequence.Delete()
}

func settingsValue(settings *qt.QSettings, key string) *qt.QVariant {
	view := qt.NewQAnyStringView3(key)
	defer view.Delete()
	return settings.ValueWithKey(*view)
}

func settingsSetValue(settings *qt.QSettings, key string, value *qt.QVariant) {
	view := qt.NewQAnyStringView3(key)
	settings.SetValue(*view, value)
	view.Delete()
	value.Delete()
}
