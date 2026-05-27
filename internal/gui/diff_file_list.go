package gui

import (
	"github.com/thiagokokada/gitk-go/internal/git"
	"github.com/thiagokokada/gitk-go/internal/gui/model"
	. "modernc.org/tk9.0"
)

func (a *Controller) setFileSections(sections []git.FileSection) {
	fileList := model.NewDiffViewModel(sections)
	if !a.ui.RenderDiffFileList(fileList) {
		return
	}
	a.model.State.Diff.SetFileSections(fileList.Sections)
	a.syncFileSelectionToDiff()
}

func (a *Controller) onFileSelectionChanged(e *Event) {
	if e == nil {
		return
	}
	idx, ok := a.ui.DiffFileListIndexAtY(e.Y, len(a.model.State.Diff.FileSections))
	if !ok {
		return
	}
	line, ok := a.model.State.Diff.BeginUserFileSelection(idx)
	if !ok {
		return
	}
	a.setFileListSelection(idx)
	a.ui.ScrollDiffToLine(line)
}

func (a *Controller) syncFileSelectionToDiff() {
	line := a.ui.DiffTopLine()
	idx, ok := a.model.State.Diff.SyncSelectionIndexForLine(line)
	if !ok {
		return
	}
	a.setFileListSelection(idx)
}

func (a *Controller) setFileListSelection(idx int) {
	if !a.model.State.Diff.SelectFileIndex(idx) {
		return
	}
	a.ui.SetDiffFileListSelection(idx)
	PostEvent(func() {
		a.model.State.Diff.FinishProgrammaticFileSelection()
	}, false)
}
