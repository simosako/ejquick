//go:build gui

package gui

import (
	"errors"
	"os"
	"time"

	qt "github.com/mappu/miqt/qt6"

	"github.com/simosako/ejquick/internal/buildinfo"
	"github.com/simosako/ejquick/internal/config"
	"github.com/simosako/ejquick/internal/logging"
)

// Application metadata fixed by design D30. The desktop file name is the
// Wayland application ID and must match the desktop entry installed by
// the D30 registration script. The organization and application names
// also fix the QSettings location used for GUI state (design D16).
const (
	organizationName = "io.github.simosako"
	applicationName  = "ejquick-gui"
	desktopFileName  = "io.github.simosako.ejquick"
	// iconName is the Freedesktop standard generic dictionary icon
	// (design D42).
	iconName = "accessories-dictionary"
)

// Run initializes Qt, loads the configuration (retrying through the
// startup dialog on failure, design D39/D46), opens the dictionary
// databases, shows the main window, and runs the event loop until the
// last window is closed (design D4). It returns the process exit code:
// 0 for a normal GUI exit, 2 when startup failed.
//
// Run must be called from the main goroutine. Worker results return to
// the Qt thread through miqt/qt6/mainthread (design D37); shutdown
// follows the two-phase scheme of design D38 scaled to M2 (search
// workers only): the close event cancels work and closes the
// dispatcher, and Finalize waits for workers and closes the services
// after the event loop exits.
func Run(opts Options, logger *logging.Logger) int {
	start := time.Now()
	skipPortalServices()
	// Qt documents these static setters as belonging before the
	// QGuiApplication is constructed; desktopFileName in particular is
	// read by the Wayland platform plugin as the application ID.
	qt.QCoreApplication_SetOrganizationName(organizationName)
	qt.QCoreApplication_SetApplicationName(applicationName)
	qt.QCoreApplication_SetApplicationVersion(buildinfo.Version)
	qt.QGuiApplication_SetDesktopFileName(desktopFileName)
	qt.NewQApplication(os.Args)

	// Design D39/D46: the configuration is loaded on the live
	// application so failures can offer an in-process retry dialog; a
	// missing default configuration file simply means defaults.
	cfg, ok := loadConfigInteractive(opts, logger)
	if !ok {
		return 2
	}

	// Design D42: resolve the system theme dictionary icon before the
	// first widget is created. A missing theme icon is not an error and
	// falls back to the platform default look. Design D26: the QPA
	// platform and input method are left to Qt and the desktop session.
	icon := qt.QIcon_FromTheme(iconName)

	// Design D40-C: both dictionary databases are opened and validated
	// synchronously before the window is created, so the first rendered
	// view is final and no loading state is needed.
	startup := openDictionaries(cfg, logger)
	controller := NewController(ControllerConfig{
		Services:   startup.Services,
		Statuses:   startup.Statuses,
		Initial:    startup.Initial,
		MaxResults: cfg.Search.MaxResults,
		Logger:     logger,
	})

	// Design D37: one dispatcher funnels typed events into a single
	// main-thread apply entry point.
	var w *mainWindow
	dispatcher := newQtDispatcher(func(ev UIEvent) {
		if w == nil {
			return
		}
		controller.ApplyEvent(ev)
		w.render(controller.View())
	})
	controller.SetDispatcher(dispatcher)

	// Design D16: GUI state lives in QSettings, separate from the
	// user-managed TOML configuration.
	settings := qt.NewQSettings7(organizationName, applicationName)
	w = newMainWindow(windowUI{
		controller: controller,
		logger:     logger,
		icon:       icon,
		settings:   settings,
		dispatcher: dispatcher,
	})

	w.window.Show()
	logger.Debug("gui: event loop entered elapsed=%s", time.Since(start).Round(time.Microsecond))
	code := qt.QApplication_Exec()

	// Design D38: after the event loop has exited, wait for workers and
	// release the dictionary services.
	controller.Finalize()
	return code
}

// loadConfigInteractive loads the configuration, showing the design D46
// startup dialog on failure. It returns ok=false when the user quits;
// the caller exits with code 2 (design D39).
func loadConfigInteractive(opts Options, logger *logging.Logger) (*config.Config, bool) {
	cfg, err := LoadConfig(opts.ConfigPath, opts.ConfigPathSet)
	for err != nil {
		category := ConfigUnknown
		path := opts.ConfigPath
		var cfgErr *ConfigError
		if errors.As(err, &cfgErr) {
			category = cfgErr.Category()
			path = cfgErr.Path()
		}
		// The raw error goes to the log exactly once per attempt; the
		// dialog shows only the category message and the path (design
		// D44/D46).
		logger.Error("startup: %v", err)
		if !runStartupDialog(category, path, logger) {
			return nil, false
		}
		cfg, err = LoadConfig(opts.ConfigPath, opts.ConfigPathSet)
	}
	return cfg, true
}

// runStartupDialog shows the configuration error dialog (design D46)
// and reports whether the user chose Retry.
func runStartupDialog(category ConfigCategory, path string, logger *logging.Logger) bool {
	dlg := qt.NewQDialog2()
	dlg.SetWindowTitle("EJQuick")
	layout := qt.NewQVBoxLayout2()
	dlg.SetLayout(layout.QLayout)

	message := qt.NewQLabel3(ConfigMessage(category))
	message.SetWordWrap(true)
	layout.AddWidget(message.QWidget)

	if path != "" {
		pathLabel := qt.NewQLabel3(path)
		// The path is selectable so it can be copied from the dialog
		// (design D46; the only place paths appear in the UI).
		pathLabel.SetTextInteractionFlags(qt.TextSelectableByMouse)
		layout.AddWidget(pathLabel.QWidget)
	}

	layout.AddStretch()

	buttons := qt.NewQHBoxLayout2()
	layout.AddLayout(buttons.QLayout)

	openLabel, openTarget := startupOpenAction(category, path)
	if openLabel != "" && openTarget != "" {
		openBtn := qt.NewQPushButton(dlg.QWidget)
		openBtn.SetText(openLabel)
		openBtn.OnClicked(func() { openExternalPath(openTarget, dlg.QWidget) })
		buttons.AddWidget(openBtn.QWidget)
	}
	if logPath, ok := logger.Path(); ok {
		logBtn := qt.NewQPushButton(dlg.QWidget)
		logBtn.SetText("Open Log")
		logBtn.OnClicked(func() { openExternalPath(logPath, dlg.QWidget) })
		buttons.AddWidget(logBtn.QWidget)
	}
	buttons.AddStretch()

	quitBtn := qt.NewQPushButton(dlg.QWidget)
	quitBtn.SetText("Quit")
	quitBtn.OnClicked(dlg.Reject)
	buttons.AddWidget(quitBtn.QWidget)
	retryBtn := qt.NewQPushButton(dlg.QWidget)
	retryBtn.SetText("Retry")
	retryBtn.SetDefault(true)
	retryBtn.OnClicked(dlg.Accept)
	buttons.AddWidget(retryBtn.QWidget)

	// Escape and window close reject the dialog: they mean Quit
	// (design D39/D46).
	return dlg.Exec() == int(qt.QDialog__Accepted)
}

// startupOpenAction returns the label and target of the contextual open
// action for the configuration dialog, or empty strings when the
// category has none (design D46).
func startupOpenAction(category ConfigCategory, path string) (label, target string) {
	switch ConfigOpenChoice(category, path) {
	case ConfigOpenFile:
		return "Open Configuration File", path
	case ConfigOpenParentFolder:
		dir := NearestExistingDir(path)
		if dir == "" {
			return "", ""
		}
		return "Open Parent Folder", dir
	}
	return "", ""
}

// openExternalPath opens a local file or directory in the associated
// system application (design D24); on failure it explains itself with a
// short message box showing the selectable path.
func openExternalPath(path string, parent *qt.QWidget) {
	if qt.QDesktopServices_OpenUrl(qt.QUrl_FromLocalFile(path)) {
		return
	}
	box := qt.NewQMessageBox6(qt.QMessageBox__Warning, "EJQuick",
		"The file could not be opened:\n"+path, qt.QMessageBox__Ok, parent)
	box.SetTextInteractionFlags(qt.TextSelectableByMouse)
	box.Exec()
}

// skipPortalServices opts out of Qt's XDG desktop portal services via
// the QT_NO_XDG_DESKTOP_PORTAL environment variable unless the user has
// already chosen a value.
//
// Since Qt 6.10, the Unix services plugin registers desktopFileName
// with org.freedesktop.host.portal.Registry at startup so portals can
// attribute their calls to the application. With xdg-desktop-portal
// >= 1.20 that registration routinely fails ("Could not register app
// ID: Connection already associated with an application ID" or "App
// info not found for ...") because the portal already caches every
// portal caller, or the .desktop entry is not installed (design D30
// ships that installer in a later milestone). Qt logs the failure as a
// qt.qpa.services warning on stderr even though nothing else is
// affected (upstream: flatpak/xdg-desktop-portal#1612).
//
// ejquick-gui never calls portals itself: design D24 opens external
// files through QDesktopServices, whose portal paths only activate
// inside Flatpak/Snap sandboxes, and the design leaves file dialogs and
// input methods to Qt and the desktop session (design D26). Skipping
// the portal services therefore changes no behavior; it only removes
// the failed registration attempt and its stderr warning.
func skipPortalServices() {
	if os.Getenv("QT_NO_XDG_DESKTOP_PORTAL") == "" {
		os.Setenv("QT_NO_XDG_DESKTOP_PORTAL", "1")
	}
}
