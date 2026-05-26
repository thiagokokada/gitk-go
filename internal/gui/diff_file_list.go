package gui

import (
	"fmt"

	"github.com/thiagokokada/gitk-go/internal/git"
	"github.com/thiagokokada/gitk-go/internal/gui/model"
	"github.com/thiagokokada/gitk-go/internal/gui/view"
	. "modernc.org/tk9.0"
)

func (a *Controller) setFileSections(sections []git.FileSection) {
	if a.ui.DiffFileList == nil {
		return
	}
	fileList := model.NewDiffViewModel(sections)
	a.model.State.Diff.SetFileSections(fileList.Sections)
	a.ui.RenderDiffFileList(fileList)
	a.syncFileSelectionToDiff()
}

func (a *Controller) onFileSelectionChanged(e *Event) {
	idx, ok := a.diffFileListIndexAtY(e)
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
	a.ui.DiffFileList.TagRemove("diffFileSelected", "1.0", END)
	line := idx + 1
	a.ui.DiffFileList.TagAdd(
		"diffFileSelected",
		fmt.Sprintf("%d.0", line),
		fmt.Sprintf("%d.end", line),
	)
	a.ui.DiffFileList.See(fmt.Sprintf("%d.0", line))
	PostEvent(func() {
		a.model.State.Diff.FinishProgrammaticFileSelection()
	}, false)
}

func (a *Controller) diffFileListIndexAtY(e *Event) (int, bool) {
	if e == nil || a.ui.DiffFileList == nil {
		return 0, false
	}
	line, ok := view.TextIndexLineNumber(a.ui.DiffFileList.Index(fmt.Sprintf("@0,%d", e.Y)))
	if !ok {
		return 0, false
	}
	idx := line - 1
	if idx < 0 || idx >= len(a.model.State.Diff.FileSections) {
		return 0, false
	}
	return idx, true
}
