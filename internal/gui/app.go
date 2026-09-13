//go:build gui

package gui

import (
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

// Run initializes Qt, shows the main window, and runs the event loop
// until the last window is closed (design D4). It returns the process
// exit code: 0 for a normal GUI exit.
//
// Run must be called from the main goroutine before any other goroutine
// touches Qt. Later milestones dispatch worker results back to the Qt
// thread with miqt/qt6/mainthread (design D37) and shut down in two
// phases (design D38); the M1 skeleton has no workers yet.
func Run(cfg *config.Config, logger *logging.Logger) int {
	start := time.Now()
	qt.NewQApplication(os.Args)
	qt.QCoreApplication_SetOrganizationName(organizationName)
	qt.QCoreApplication_SetApplicationName(applicationName)
	qt.QCoreApplication_SetApplicationVersion(buildinfo.Version)
	qt.QGuiApplication_SetDesktopFileName(desktopFileName)

	// Design D42: resolve the system theme dictionary icon before the
	// first widget is created. A missing theme icon is not an error and
	// falls back to the platform default look. Design D26: the QPA
	// platform and input method are left to Qt and the desktop session.

	icon := qt.QIcon_FromTheme(iconName)
	mw := newMainWindow(icon)
	mw.window.Show()
	logger.Debug("gui: event loop entered elapsed=%s", time.Since(start).Round(time.Microsecond))
	return qt.QApplication_Exec()
}
