package gui

import (
	. "modernc.org/tk9.0"
)

type appWidgets struct {
	menubar             *MenuWidget
	fileMenu            *MenuWidget
	viewMenu            *MenuWidget
	helpMenu            *MenuWidget
	status              *TLabelWidget
	repoLabel           *TLabelWidget
	filterEntry         *TEntryWidget
	reloadButton        *TButtonWidget
	graphCanvas         *CanvasWidget
	treeView            *TTreeviewWidget
	treeContextMenu     *MenuWidget
	diffDetail          *TextWidget
	diffFileList        *ListboxWidget
	diffContextMenu     *MenuWidget
	diffFileContextMenu *MenuWidget
	shortcutsWindow     *ToplevelWidget
	branchWindow        *ToplevelWidget
}
