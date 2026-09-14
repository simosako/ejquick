//go:build gui

package gui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	qt "github.com/mappu/miqt/qt6"

	"github.com/simosako/ejquick/internal/buildinfo"
	"github.com/simosako/ejquick/internal/dictionary"
	"github.com/simosako/ejquick/internal/gui/buildprocess"
	"github.com/simosako/ejquick/internal/gui/controller"
	"github.com/simosako/ejquick/internal/gui/startup"
)

type builderDialogState int

const (
	builderSetup builderDialogState = iota
	builderPreparing
	builderRunning
	builderCanceling
	builderReopening
	builderSucceeded
	builderFailed
)

type builderRequest struct {
	dictionary dictionary.Type
	source     string
	output     string
	force      bool
	options    buildprocess.Options
}

type builderDialog struct {
	owner  *mainWindow
	dialog *qt.QDialog
	stack  *qt.QStackedWidget
	state  builderDialogState

	setupPage   *qt.QWidget
	dictionary  *qt.QComboBox
	source      *qt.QLineEdit
	output      *qt.QLabel
	replace     *qt.QCheckBox
	setupNotice *qt.QLabel
	buildButton *qt.QPushButton

	progressPage *qt.QWidget
	title        *qt.QLabel
	progress     *qt.QProgressBar
	phase        *qt.QLabel
	counts       *qt.QLabel
	cancelButton *qt.QPushButton
	backButton   *qt.QPushButton
	logButton    *qt.QPushButton
	closeButton  *qt.QPushButton

	outputExists  bool
	outputBlocked bool
	cancelAsked   bool
	request       builderRequest
}

func newBuilderDialog(owner *mainWindow, initial dictionary.Type) *builderDialog {
	d := &builderDialog{owner: owner, state: builderSetup}
	d.dialog = qt.NewQDialog(owner.main.QWidget)
	d.dialog.SetWindowTitle("Build Dictionary Database")
	d.dialog.SetWindowModality(qt.WindowModal)
	d.dialog.SetMinimumWidth(620)
	d.dialog.Resize(680, 420)
	if owner.icon != nil && !owner.icon.IsNull() {
		d.dialog.SetWindowIcon(owner.icon)
	}

	root := qt.NewQVBoxLayout2()
	root.SetContentsMargins(16, 16, 16, 16)
	root.SetSpacing(12)
	d.dialog.SetLayout(root.QLayout)
	d.stack = qt.NewQStackedWidget(d.dialog.QWidget)
	root.AddWidget2(d.stack.QWidget, 1)

	d.createSetupPage()
	d.createProgressPage()
	d.connectSignals()

	index := 0
	if initial == dictionary.Waei {
		index = 1
	}
	d.dictionary.SetCurrentIndex(index)
	d.updateSetupForDictionary()
	return d
}

func (d *builderDialog) createSetupPage() {
	d.setupPage = qt.NewQWidget(d.stack.QWidget)
	layout := qt.NewQVBoxLayout2()
	layout.SetSpacing(12)
	d.setupPage.SetLayout(layout.QLayout)

	intro := qt.NewQLabel5("Build a searchable database from a purchased dictionary TXT file.", d.setupPage)
	intro.SetWordWrap(true)
	layout.AddWidget(intro.QWidget)

	form := qt.NewQFormLayout2()
	form.SetSpacing(10)
	d.dictionary = qt.NewQComboBox(d.setupPage)
	d.dictionary.SetAccessibleName("Dictionary")
	for _, dict := range dictionary.All {
		d.dictionary.AddItem(controller.DictionaryName(dict))
	}
	form.AddRow3("Dictionary:", d.dictionary.QWidget)

	sourceRow := qt.NewQWidget(d.setupPage)
	sourceLayout := qt.NewQHBoxLayout2()
	sourceLayout.SetContentsMargins(0, 0, 0, 0)
	sourceLayout.SetSpacing(8)
	sourceRow.SetLayout(sourceLayout.QLayout)
	d.source = qt.NewQLineEdit(sourceRow)
	d.source.SetAccessibleName("Source TXT file")
	d.source.SetPlaceholderText("Select the purchased TXT file")
	sourceLayout.AddWidget2(d.source.QWidget, 1)
	browse := qt.NewQPushButton5("&Browse...", sourceRow)
	sourceLayout.AddWidget(browse.QWidget)
	form.AddRow3("Source TXT:", sourceRow)

	d.output = qt.NewQLabel5("", d.setupPage)
	d.output.SetWordWrap(true)
	d.output.SetTextInteractionFlags(qt.TextSelectableByMouse | qt.TextSelectableByKeyboard)
	d.output.SetAccessibleName("Output database")
	form.AddRow3("Output:", d.output.QWidget)
	layout.AddLayout(form.QLayout)

	d.replace = qt.NewQCheckBox4("Replace the existing dictionary database", d.setupPage)
	d.replace.SetAccessibleName("Replace the existing dictionary database")
	layout.AddWidget(d.replace.QWidget)

	d.setupNotice = qt.NewQLabel5("", d.setupPage)
	d.setupNotice.SetWordWrap(true)
	d.setupNotice.SetTextInteractionFlags(qt.TextSelectableByMouse | qt.TextSelectableByKeyboard)
	d.setupNotice.SetAccessibleName("Build status")
	layout.AddWidget(d.setupNotice.QWidget)
	layout.AddStretch()

	buttons := qt.NewQHBoxLayout2()
	buttons.AddStretch()
	cancel := qt.NewQPushButton5("&Cancel", d.setupPage)
	d.buildButton = qt.NewQPushButton5("&Build", d.setupPage)
	d.buildButton.SetDefault(true)
	buttons.AddWidget(cancel.QWidget)
	buttons.AddWidget(d.buildButton.QWidget)
	layout.AddLayout(buttons.QLayout)
	d.stack.AddWidget(d.setupPage)

	browse.OnClicked(d.safeCallback(d.browseSource))
	cancel.OnClicked(d.safeCallback(func() { d.dialog.Reject() }))
}

func (d *builderDialog) createProgressPage() {
	d.progressPage = qt.NewQWidget(d.stack.QWidget)
	layout := qt.NewQVBoxLayout2()
	layout.SetSpacing(12)
	d.progressPage.SetLayout(layout.QLayout)

	d.title = qt.NewQLabel5("Preparing dictionary build...", d.progressPage)
	d.title.SetWordWrap(true)
	d.title.SetAccessibleName("Build status")
	layout.AddWidget(d.title.QWidget)
	d.progress = qt.NewQProgressBar(d.progressPage)
	d.progress.SetRange(0, 0)
	d.progress.SetTextVisible(false)
	d.progress.SetAccessibleName("Dictionary build progress")
	layout.AddWidget(d.progress.QWidget)
	d.phase = qt.NewQLabel5("Preparing...", d.progressPage)
	d.phase.SetWordWrap(true)
	d.phase.SetTextInteractionFlags(qt.TextSelectableByMouse | qt.TextSelectableByKeyboard)
	layout.AddWidget(d.phase.QWidget)
	d.counts = qt.NewQLabel5("Lines read: 0\nEntries: 0\nSkipped: 0", d.progressPage)
	d.counts.SetTextInteractionFlags(qt.TextSelectableByMouse | qt.TextSelectableByKeyboard)
	d.counts.SetAccessibleName("Build counts")
	layout.AddWidget(d.counts.QWidget)
	layout.AddStretch()

	buttons := qt.NewQHBoxLayout2()
	buttons.AddStretch()
	d.logButton = qt.NewQPushButton5("Open &Log", d.progressPage)
	d.backButton = qt.NewQPushButton5("&Back to Setup", d.progressPage)
	d.cancelButton = qt.NewQPushButton5("&Cancel", d.progressPage)
	d.closeButton = qt.NewQPushButton5("&Close", d.progressPage)
	d.closeButton.SetDefault(true)
	buttons.AddWidget(d.logButton.QWidget)
	buttons.AddWidget(d.backButton.QWidget)
	buttons.AddWidget(d.cancelButton.QWidget)
	buttons.AddWidget(d.closeButton.QWidget)
	layout.AddLayout(buttons.QLayout)
	d.stack.AddWidget(d.progressPage)

	d.logButton.OnClicked(d.safeCallback(func() { d.owner.openLogAction.Trigger() }))
	d.backButton.OnClicked(d.safeCallback(d.backToSetup))
	d.cancelButton.OnClicked(d.safeCallback(d.requestCancel))
	d.closeButton.OnClicked(d.safeCallback(func() { d.dialog.Accept() }))
}

func (d *builderDialog) connectSignals() {
	d.dictionary.OnCurrentIndexChanged(func(int) {
		defer d.owner.recoverCallback()
		d.updateSetupForDictionary()
	})
	d.source.OnTextChanged(func(string) {
		defer d.owner.recoverCallback()
		if d.state == builderSetup {
			d.setupNotice.SetText("")
		}
		d.updateBuildEnabled()
	})
	d.replace.OnStateChanged(func(int) {
		defer d.owner.recoverCallback()
		d.updateBuildEnabled()
	})
	d.buildButton.OnClicked(d.safeCallback(d.startBuild))
	d.dialog.OnReject(func(super func()) {
		defer d.owner.recoverCallback()
		if d.interceptClose() {
			return
		}
		super()
	})
	d.dialog.OnCloseEvent(func(super func(*qt.QCloseEvent), event *qt.QCloseEvent) {
		defer d.owner.recoverCallback()
		if d.interceptClose() {
			event.Ignore()
			return
		}
		super(event)
	})
	d.dialog.OnFinished(func(int) {
		defer d.owner.recoverCallback()
		d.owner.builderDialogClosed(d)
		d.dialog.QObject.DeleteLater()
	})
}

func (d *builderDialog) open() {
	d.stack.SetCurrentWidget(d.setupPage)
	d.dialog.Open()
	d.source.SetFocus()
}

func (d *builderDialog) selectedDictionary() dictionary.Type {
	if d.dictionary.CurrentIndex() == 1 {
		return dictionary.Waei
	}
	return dictionary.Eiwa
}

func (d *builderDialog) updateSetupForDictionary() {
	dict := d.selectedDictionary()
	path := d.owner.config.Database(dict)
	d.outputExists = false
	d.outputBlocked = false
	d.replace.SetChecked(false)
	info, err := os.Lstat(path)
	switch {
	case err == nil:
		d.outputExists = true
		d.output.SetText(fmt.Sprintf("%s\nExisting file size: %d bytes", path, info.Size()))
	case errors.Is(err, os.ErrNotExist):
		d.output.SetText(path)
	default:
		d.outputBlocked = true
		d.output.SetText(path)
		d.setupNotice.SetText("The output database could not be inspected.")
	}
	d.replace.SetVisible(d.outputExists)
	if !d.outputBlocked {
		d.setupNotice.SetText("")
	}
	d.updateBuildEnabled()
}

func (d *builderDialog) updateBuildEnabled() {
	enabled := d.state == builderSetup && d.source.Text() != "" && !d.outputBlocked
	if d.outputExists && !d.replace.IsChecked() {
		enabled = false
	}
	d.buildButton.SetEnabled(enabled)
}

func (d *builderDialog) browseSource() {
	directory := ""
	if source := d.source.Text(); source != "" {
		directory = filepath.Dir(source)
	}
	path := qt.QFileDialog_GetOpenFileName4(
		d.dialog.QWidget,
		"Select Dictionary TXT File",
		directory,
		"Text files (*.TXT *.txt);;All files (*)",
	)
	if path != "" {
		d.source.SetText(path)
	}
}

func (d *builderDialog) startBuild() {
	if d.state != builderSetup {
		return
	}
	replaceChecked := d.replace.IsChecked()
	d.updateSetupForDictionary()
	if d.outputExists && replaceChecked {
		d.replace.SetChecked(true)
	}
	if d.source.Text() == "" || d.outputBlocked || (d.outputExists && !d.replace.IsChecked()) {
		d.updateBuildEnabled()
		return
	}
	dict := d.selectedDictionary()
	d.request = builderRequest{
		dictionary: dict,
		source:     d.source.Text(),
		output:     d.owner.config.Database(dict),
		force:      d.outputExists && d.replace.IsChecked(),
	}
	d.setupNotice.SetText("")
	d.owner.startBuilder(d)
}

func (d *builderDialog) showPreparing() {
	d.state = builderPreparing
	d.cancelAsked = false
	d.stack.SetCurrentWidget(d.progressPage)
	d.title.SetText("Preparing dictionary build...")
	d.progress.SetVisible(true)
	d.progress.SetRange(0, 0)
	d.phase.SetText("Waiting for active searches to stop...")
	d.counts.SetText("Lines read: 0\nEntries: 0\nSkipped: 0")
	d.counts.SetVisible(true)
	d.cancelButton.SetText("&Cancel")
	d.cancelButton.SetEnabled(true)
	d.cancelButton.SetVisible(true)
	d.backButton.SetVisible(false)
	d.logButton.SetVisible(false)
	d.closeButton.SetVisible(false)
}

func (d *builderDialog) showRunning() {
	d.state = builderRunning
	d.title.SetText("Building dictionary database...")
	d.phase.SetText("Starting dictionary builder...")
}

func (d *builderDialog) applyEvent(event buildprocess.Event) {
	switch event.Kind {
	case buildprocess.EventReady:
		if d.state == builderRunning {
			d.phase.SetText("Reading builder response...")
		}
	case buildprocess.EventPhase:
		if d.state == builderRunning {
			d.phase.SetText(builderPhaseText(event.Phase, event.State))
		}
	case buildprocess.EventProgress:
		if d.state == builderRunning {
			d.counts.SetText(fmt.Sprintf("Lines read: %d\nEntries: %d\nSkipped: %d", event.Lines, event.Entries, event.Skipped))
		}
	case buildprocess.EventFinished:
		d.owner.builderProcessFinished(d, event.Finished)
	}
}

func builderPhaseText(phase, state string) string {
	name := phase
	switch phase {
	case "reading":
		name = "Reading source text"
	case "btree":
		name = "Building headword index"
	case "fts":
		name = "Building full-text index"
	case "vacuum":
		name = "Compacting database"
	case "analyze":
		name = "Analyzing database"
	case "validation":
		name = "Validating database"
	}
	switch state {
	case "done":
		return name + " completed."
	case "failed":
		return name + " failed."
	default:
		return name + "..."
	}
}

func (d *builderDialog) requestCancel() {
	switch d.state {
	case builderPreparing:
		d.cancelAsked = true
		d.state = builderCanceling
		d.cancelButton.SetEnabled(false)
		d.cancelButton.SetText("Canceling...")
		d.phase.SetText("Canceling...")
	case builderRunning:
		d.cancelAsked = true
		d.state = builderCanceling
		d.cancelButton.SetEnabled(false)
		d.cancelButton.SetText("Canceling...")
		d.phase.SetText("Canceling...")
		d.owner.cancelBuilderProcess()
	}
}

func (d *builderDialog) interceptClose() bool {
	switch d.state {
	case builderPreparing, builderRunning:
		answer := qt.QMessageBox_Question6(
			d.dialog.QWidget,
			"Cancel Dictionary Build",
			"Cancel the dictionary database build?",
			qt.QMessageBox__Yes|qt.QMessageBox__No,
			qt.QMessageBox__No,
		)
		if answer == qt.QMessageBox__Yes {
			d.requestCancel()
		}
		return true
	case builderCanceling, builderReopening:
		return true
	default:
		return false
	}
}

func (d *builderDialog) showReopening() {
	d.state = builderReopening
	d.title.SetText("Checking dictionary database...")
	d.phase.SetText("Opening and validating the dictionary database...")
	d.progress.SetVisible(true)
	d.progress.SetRange(0, 0)
	d.cancelButton.SetVisible(false)
	d.backButton.SetVisible(false)
	d.logButton.SetVisible(false)
	d.closeButton.SetVisible(false)
}

func (d *builderDialog) showOutcome(result buildprocess.Result) {
	switch result.Outcome {
	case buildprocess.OutcomeSucceeded:
		d.state = builderSucceeded
		d.title.SetText("Dictionary database built successfully.")
		d.phase.SetText(fmt.Sprintf(
			"Source lines: %d\nEntries: %d\nSkipped: %d\nDatabase size: %d bytes\nElapsed: %s",
			result.Stats.SourceLines,
			result.Stats.Entries,
			result.Stats.Skipped,
			result.Stats.DBSize,
			formatBuildDuration(result.Stats.Elapsed),
		))
		d.progress.SetVisible(false)
		d.counts.SetVisible(false)
		d.cancelButton.SetVisible(false)
		d.backButton.SetVisible(false)
		d.logButton.SetVisible(false)
		d.closeButton.SetVisible(true)
		d.closeButton.SetFocus()
	case buildprocess.OutcomeCanceled:
		d.backToSetupWithNotice("Build canceled.")
	default:
		d.state = builderFailed
		d.title.SetText("Dictionary database build failed.")
		d.phase.SetText(buildprocess.Message(result.Category))
		d.progress.SetVisible(false)
		d.counts.SetVisible(false)
		d.cancelButton.SetVisible(false)
		d.backButton.SetVisible(true)
		_, logAvailable := d.owner.logger.Path()
		d.logButton.SetVisible(logAvailable)
		d.closeButton.SetVisible(false)
		d.backButton.SetFocus()
	}
}

func (d *builderDialog) backToSetup() {
	if d.state != builderFailed {
		return
	}
	d.backToSetupWithNotice(d.phase.Text())
}

func (d *builderDialog) backToSetupWithNotice(notice string) {
	replaceChecked := d.replace.IsChecked()
	d.state = builderSetup
	d.cancelAsked = false
	d.stack.SetCurrentWidget(d.setupPage)
	d.setupNotice.SetText(notice)
	d.updateSetupForDictionary()
	if d.outputExists && replaceChecked {
		d.replace.SetChecked(true)
	}
	if notice != "" {
		d.setupNotice.SetText(notice)
	}
	d.source.SetFocus()
}

func (d *builderDialog) safeCallback(callback func()) func() {
	return func() {
		defer d.owner.recoverCallback()
		callback()
	}
}

func formatBuildDuration(elapsed time.Duration) string {
	if elapsed < time.Second {
		return elapsed.Round(time.Millisecond).String()
	}
	return elapsed.Round(time.Second).String()
}

func (w *mainWindow) currentDictionary() dictionary.Type {
	if w.controller != nil {
		return w.controller.Snapshot().Dictionary
	}
	return w.databases.Initial
}

func (w *mainWindow) showBuilder(initial dictionary.Type) {
	if w.shuttingDown || w.builderDialog != nil {
		return
	}
	dialog := newBuilderDialog(w, initial)
	w.builderDialog = dialog
	w.buildAction.SetEnabled(false)
	dialog.open()
}

func (w *mainWindow) builderDialogClosed(dialog *builderDialog) {
	if w.builderDialog != dialog {
		return
	}
	w.builderDialog = nil
	if !w.shuttingDown {
		w.buildAction.SetEnabled(true)
		if w.controller != nil {
			w.query.SetFocus()
		}
	}
}

func (w *mainWindow) startBuilder(dialog *builderDialog) {
	if w.shuttingDown || w.builderDialog != dialog {
		return
	}
	binary, err := buildprocess.ResolveBuilderPath()
	if err != nil {
		w.builderStartFailed(dialog, buildprocess.CategoryBinaryMissing, err)
		return
	}
	dialog.request.options = buildprocess.Options{
		Binary:         binary,
		Dictionary:     dialog.request.dictionary,
		Source:         dialog.request.source,
		Output:         dialog.request.output,
		Force:          dialog.request.force,
		ProductVersion: buildinfo.Version,
	}
	if err := buildprocess.Validate(dialog.request.options); err != nil {
		w.builderStartFailed(dialog, buildprocess.CategoryOf(err), err)
		return
	}

	dialog.showPreparing()
	oldController := w.controller
	if oldController != nil {
		oldController.BeginClose()
	}
	w.buildWorkers.Add(1)
	go func() {
		defer w.buildWorkers.Done()
		if oldController != nil {
			oldController.Wait()
		}
		w.dispatcher.post(func() {
			w.continueBuilderStart(dialog, oldController)
		})
	}()
}

func (w *mainWindow) continueBuilderStart(dialog *builderDialog, oldController *controller.Controller) {
	if w.shuttingDown || w.builderDialog != dialog || w.controller != oldController {
		return
	}
	if dialog.cancelAsked {
		w.refreshDatabaseState(dialog.request.dictionary)
		dialog.showOutcome(buildprocess.Result{Outcome: buildprocess.OutcomeCanceled})
		return
	}
	w.controller = nil
	if err := w.databases.CloseDictionary(dialog.request.dictionary); err != nil {
		result := buildprocess.Result{
			Outcome:  buildprocess.OutcomeFailed,
			Category: buildprocess.CategoryBuildFailed,
			Err:      err,
		}
		w.logBuilderResult(dialog.request.dictionary, result)
		w.reopenBuiltDictionary(dialog, result)
		return
	}

	dialog.showRunning()
	process, err := buildprocess.Start(dialog.request.options, func(event buildprocess.Event) {
		w.dispatcher.post(func() {
			if w.builderDialog == dialog && !w.shuttingDown {
				dialog.applyEvent(event)
			}
		})
	})
	if err != nil {
		result := buildprocess.Result{
			Outcome:  buildprocess.OutcomeFailed,
			Category: buildprocess.CategoryOf(err),
			Err:      err,
		}
		w.logBuilderResult(dialog.request.dictionary, result)
		w.reopenBuiltDictionary(dialog, result)
		return
	}
	w.buildMu.Lock()
	w.buildProcess = process
	w.buildMu.Unlock()
}

func (w *mainWindow) builderStartFailed(dialog *builderDialog, category buildprocess.Category, err error) {
	result := buildprocess.Result{Outcome: buildprocess.OutcomeFailed, Category: category, Err: err}
	w.logBuilderResult(dialog.request.dictionary, result)
	dialog.stack.SetCurrentWidget(dialog.progressPage)
	dialog.showOutcome(result)
}

func (w *mainWindow) cancelBuilderProcess() {
	w.buildMu.Lock()
	process := w.buildProcess
	w.buildMu.Unlock()
	if process != nil {
		process.Cancel()
	}
}

func (w *mainWindow) builderProcessFinished(dialog *builderDialog, result buildprocess.Result) {
	if w.shuttingDown || w.builderDialog != dialog {
		return
	}
	w.buildMu.Lock()
	w.buildProcess = nil
	w.buildMu.Unlock()
	w.logBuilderResult(dialog.request.dictionary, result)
	w.reopenBuiltDictionary(dialog, result)
}

func (w *mainWindow) reopenBuiltDictionary(dialog *builderDialog, result buildprocess.Result) {
	dialog.showReopening()
	dict := dialog.request.dictionary
	w.buildWorkers.Add(1)
	go func() {
		defer w.buildWorkers.Done()
		service, status := startup.OpenDictionary(w.config, dict, w.logger)
		if !w.dispatcher.post(func() {
			if w.shuttingDown || w.builderDialog != dialog {
				if service != nil {
					_ = service.Close()
				}
				return
			}
			w.databases.SetDictionary(dict, service, status)
			w.refreshDatabaseState(dict)
			dialog.showOutcome(result)
		}) && service != nil {
			_ = service.Close()
		}
	}()
}

func (w *mainWindow) refreshDatabaseState(preferred dictionary.Type) {
	if w.databases.Services[preferred] != nil {
		w.databases.Initial = preferred
	} else if w.databases.Services[preferred.Other()] != nil {
		w.databases.Initial = preferred.Other()
	} else {
		w.databases.Initial = preferred
	}
	w.configureController(w.config)
	w.query.Clear()
	w.clearResults()
	for index, dict := range dictionary.All {
		status := w.databases.Statuses[dict]
		item := w.comboModel.Item(index)
		item.SetEnabled(status.Available)
		action := w.dictionaryAction[dict]
		action.SetEnabled(status.Available)
		if status.Available {
			item.SetToolTip("")
			item.SetAccessibleDescription("")
			action.SetToolTip("")
		} else {
			message := controller.MessageForUnavailable(dict, status.Category).Text
			item.SetToolTip(message)
			item.SetAccessibleDescription(message)
			action.SetToolTip(message)
		}
	}
	w.renderInitialState()
}

func (w *mainWindow) logBuilderResult(dict dictionary.Type, result buildprocess.Result) {
	switch result.Outcome {
	case buildprocess.OutcomeSucceeded:
		w.logger.Debug("gui builder: dictionary=%s completed entries=%d skipped=%d db_size=%d elapsed=%s",
			dict, result.Stats.Entries, result.Stats.Skipped, result.Stats.DBSize, result.Stats.Elapsed)
	case buildprocess.OutcomeCanceled:
		if result.CleanupErr != nil {
			w.logger.Error("gui builder: dictionary=%s canceled hard_killed=%t cleanup: %v",
				dict, result.HardKilled, result.CleanupErr)
		} else {
			w.logger.Debug("gui builder: dictionary=%s canceled hard_killed=%t", dict, result.HardKilled)
		}
	default:
		w.logger.Error("gui builder: dictionary=%s category=%s exit=%d hard_killed=%t: %v; stderr_tail=%q; cleanup=%v",
			dict, result.Category, result.ExitCode, result.HardKilled, result.Err, result.Stderr, result.CleanupErr)
	}
}
