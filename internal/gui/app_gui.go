//go:build gui

// Package gui implements the Qt 6 Widgets frontend.
package gui

import (
	"errors"
	"os"
	"runtime"

	qt "github.com/mappu/miqt/qt6"

	"github.com/simosako/ejquick/internal/buildinfo"
	"github.com/simosako/ejquick/internal/gui/startup"
	"github.com/simosako/ejquick/internal/logging"
)

const (
	applicationID = "io.github.simosako.ejquick"
	productName   = "EJQuick"
)

// Options contains process-level GUI startup inputs.
type Options struct {
	Arguments    []string
	ConfigPath   string
	ExplicitPath bool
	// SmokeTest runs a bounded synthetic UI flow for CI verification.
	SmokeTest bool
	Logger    *logging.Logger
}

// Run initializes QApplication once, loads configuration and databases, and
// runs the Qt event loop.
func Run(options Options) (exitCode int) {
	logger := options.Logger
	if logger == nil {
		logger = logging.Nop()
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			logger.Error("gui fatal: %v", recovered)
			exitCode = 2
		}
	}()

	arguments := options.Arguments
	if len(arguments) == 0 {
		arguments = []string{"ejquick-gui"}
	}
	// Wayland and portal integrations consume application identity during
	// QApplication construction.
	qt.QCoreApplication_SetOrganizationName("simosako")
	qt.QCoreApplication_SetOrganizationDomain("github.com/simosako")
	qt.QCoreApplication_SetApplicationName("ejquick")
	qt.QCoreApplication_SetApplicationVersion(buildinfo.Version)
	qt.QGuiApplication_SetApplicationDisplayName(productName)
	qt.QGuiApplication_SetDesktopFileName(applicationID)

	application := qt.NewQApplication(arguments)
	defer deleteQtWrapper(application)
	qt.QGuiApplication_SetQuitOnLastWindowClosed(false)
	logger.Debug("gui startup: platform=%q QT_QPA_PLATFORM=%q QT_IM_MODULE=%q",
		qt.QGuiApplication_PlatformName(), os.Getenv("QT_QPA_PLATFORM"), os.Getenv("QT_IM_MODULE"))

	icon := qt.QIcon_FromTheme("accessories-dictionary")
	defer deleteQtWrapper(icon)
	if !icon.IsNull() {
		qt.QGuiApplication_SetWindowIcon(icon)
	}

	loaded, ok := loadConfigWithDialog(options, logger)
	if !ok {
		return 2
	}
	databases := startup.OpenDatabases(loaded.Config, logger)
	defer func() {
		if err := databases.Close(); err != nil {
			logger.Error("gui shutdown: %v", err)
		}
	}()

	window := newMainWindow(loaded.Config, databases, logger, icon)
	window.show()
	if options.SmokeTest {
		window.startSmokeTest()
	}
	exitCode = qt.QApplication_Exec()
	logger.Debug("gui shutdown: event loop exited code=%d os=%s", exitCode, runtime.GOOS)
	return exitCode
}

func loadConfigWithDialog(options Options, logger *logging.Logger) (*startup.LoadedConfig, bool) {
	for {
		loaded, err := startup.LoadConfig(options.ConfigPath, options.ExplicitPath)
		if err == nil {
			return loaded, true
		}
		category := startup.ConfigCategoryOf(err)
		logger.Error("gui startup: load config category=%s: %v", category, err)
		var configErr *startup.ConfigError
		path := options.ConfigPath
		if errors.As(err, &configErr) {
			path = configErr.Path
		}
		if !showConfigErrorDialog(category, path, logger) {
			return nil, false
		}
	}
}
