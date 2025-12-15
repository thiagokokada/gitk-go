package gui

import (
	. "modernc.org/tk9.0"
)

type appWidgets struct {
	status          *TLabelWidget
	repoLabel       *TLabelWidget
	filterEntry     *TEntryWidget
	reloadButton    *TButtonWidget
	treeView        *TTreeviewWidget
	treeContextMenu *MenuWidget
	diffDetail      *TextWidget
	diffFileList    *ListboxWidget
	diffContextMenu *MenuWidget
	shortcutsWindow *ToplevelWidget
}
