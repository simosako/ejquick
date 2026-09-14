//go:build gui

package gui

import (
	qt "github.com/mappu/miqt/qt6"
)

// Initial window geometry and pane sizes are the D16 common defaults.
// They are logical pixels (design D50); QSettings persistence and
// splitter state restore arrive with the main search implementation.
const (
	initialWidth  = 960
	initialHeight = 640
	minimumWidth  = 720
	minimumHeight = 480
	leftMinWidth  = 240
	rightMinWidth = 320
)

// mainWindow holds the widgets of the M1 skeleton so later milestones
// can wire them up without changing construction. The search row and
// panes are present but disabled: M1 opens no databases yet.
type mainWindow struct {
	window     *qt.QMainWindow
	dictCombo  *qt.QComboBox
	searchEdit *qt.QLineEdit
	resultList *qt.QListView
	listModel  *qt.QStringListModel
	detail     *qt.QPlainTextEdit
}

// newMainWindow builds the two-pane window skeleton (design D3).
func newMainWindow(icon *qt.QIcon) *mainWindow {
	window := qt.NewQMainWindow2()
	window.QWidget.SetWindowTitle("EJQuick")
	window.QWidget.Resize(initialWidth, initialHeight)
	window.QWidget.SetMinimumSize2(minimumWidth, minimumHeight)
	if icon != nil && !icon.IsNull() {
		window.QWidget.SetWindowIcon(icon)
	}

	central := qt.NewQWidget2()
	outer := qt.NewQVBoxLayout2()
	central.SetLayout(outer.QLayout)

	// Top search row (design D9/D11). The dictionary combo box stays on
	// the left of the query field; both stay disabled until the search
	// implementation lands.
	row := qt.NewQHBoxLayout2()
	outer.AddLayout(row.QLayout)
	dictCombo := qt.NewQComboBox2()
	dictCombo.SetEnabled(false)
	row.AddWidget(dictCombo.QWidget)
	searchEdit := qt.NewQLineEdit2()
	searchEdit.SetPlaceholderText("Search")
	searchEdit.SetEnabled(false)
	row.AddWidget(searchEdit.QWidget)

	// Two-pane results area (design D11): a list view backed by a string
	// list model on the left, a read-only plain text detail view on the
	// right, in a 1:2 horizontal splitter.
	splitter := qt.NewQSplitter3(qt.Horizontal)
	listModel := qt.NewQStringListModel2(nil)
	resultList := qt.NewQListView2()
	resultList.SetModel(listModel.QAbstractItemModel)
	resultList.QWidget.SetMinimumWidth(leftMinWidth)
	splitter.AddWidget(resultList.QWidget)
	detail := qt.NewQPlainTextEdit2()
	detail.SetReadOnly(true)
	detail.QWidget.SetMinimumWidth(rightMinWidth)
	splitter.AddWidget(detail.QWidget)
	splitter.SetStretchFactor(0, 1)
	splitter.SetStretchFactor(1, 2)
	outer.AddWidget2(splitter.QWidget, 1)

	window.SetCentralWidget(central)

	return &mainWindow{
		window:     window,
		dictCombo:  dictCombo,
		searchEdit: searchEdit,
		resultList: resultList,
		listModel:  listModel,
		detail:     detail,
	}
}
