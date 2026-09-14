//go:build gui

package gui

import (
	"os"

	qt "github.com/mappu/miqt/qt6"

	"github.com/simosako/ejquick/internal/gui/startup"
	"github.com/simosako/ejquick/internal/logging"
)

type configDialogChoice int

const (
	configDialogQuit configDialogChoice = iota
	configDialogRetry
	configDialogOpenPath
	configDialogOpenLog
)

func showConfigErrorDialog(category startup.ConfigCategory, path string, logger *logging.Logger) bool {
	for {
		_, statErr := os.Stat(path)
		pathAvailable := path != ""
		pathExists := statErr == nil
		_, logAvailable := logger.Path()
		message := startup.MessageForConfig(category, pathAvailable, pathExists, logAvailable)
		choice := executeConfigErrorDialog(message, path)
		switch choice {
		case configDialogRetry:
			return true
		case configDialogOpenPath:
			target := path
			if message.OpenTarget == startup.ConfigOpenParent {
				var ok bool
				target, ok = nearestExistingParent(path)
				if !ok {
					showOpenPathError("Open Configuration", path)
					continue
				}
			}
			if !openLocalPath(target) {
				showOpenPathError("Open Configuration", target)
			}
		case configDialogOpenLog:
			logPath, ok := logger.Path()
			if !ok || !openLocalPath(logPath) {
				showOpenPathError("Open Log", logPath)
			}
		default:
			return false
		}
	}
}

func executeConfigErrorDialog(message startup.ConfigMessage, path string) configDialogChoice {
	box := qt.NewQMessageBox3(qt.QMessageBox__Critical, "EJQuick Configuration", message.Text)
	defer deleteQtWrapper(box)
	box.SetTextFormat(qt.PlainText)
	box.SetTextInteractionFlags(qt.TextSelectableByMouse | qt.TextSelectableByKeyboard)
	if path != "" {
		box.SetInformativeText("Configuration path:\n" + path)
	}

	retry := box.AddButton2("&Retry", qt.QMessageBox__AcceptRole)
	box.SetDefaultButton(retry)
	var openPath, openLog *qt.QPushButton
	if message.OpenTarget == startup.ConfigOpenFile {
		openPath = box.AddButton2("Open Configuration &File", qt.QMessageBox__ActionRole)
	} else if message.OpenTarget == startup.ConfigOpenParent {
		openPath = box.AddButton2("Open Parent &Folder", qt.QMessageBox__ActionRole)
	}
	if message.OpenLog {
		openLog = box.AddButton2("Open &Log", qt.QMessageBox__ActionRole)
	}
	quit := box.AddButton2("&Quit", qt.QMessageBox__RejectRole)
	box.SetEscapeButton(quit.QAbstractButton)
	box.Exec()

	clicked := box.ClickedButton()
	switch {
	case sameButton(clicked, retry):
		return configDialogRetry
	case sameButton(clicked, openPath):
		return configDialogOpenPath
	case sameButton(clicked, openLog):
		return configDialogOpenLog
	default:
		return configDialogQuit
	}
}

func sameButton(clicked *qt.QAbstractButton, button *qt.QPushButton) bool {
	return clicked != nil && button != nil &&
		clicked.UnsafePointer() == button.QAbstractButton.UnsafePointer()
}

func openLocalPath(path string) bool {
	if path == "" {
		return false
	}
	url := qt.QUrl_FromLocalFile(path)
	defer deleteQtWrapper(url)
	return qt.QDesktopServices_OpenUrl(url)
}

func showOpenPathError(title, path string) {
	text := "The path could not be opened."
	if path != "" {
		text += "\n\n" + path
	}
	box := qt.NewQMessageBox3(qt.QMessageBox__Critical, title, text)
	defer deleteQtWrapper(box)
	box.SetTextFormat(qt.PlainText)
	box.SetTextInteractionFlags(qt.TextSelectableByMouse | qt.TextSelectableByKeyboard)
	box.Exec()
}
