package gui

import (
	"fmt"

	"github.com/thiagokokada/gitk-go/internal/git"
	. "modernc.org/tk9.0"
)

func (a *Controller) setFileSections(sections []git.FileSection) {
	if a.ui.diffFileList == nil {
		return
	}
	model := newDiffViewModel(sections)
	a.model.state.diff.fileSections = model.sections
	a.model.state.diff.selectedFileIndex = -1
	a.ui.diffFileList.Configure(State("normal"))
	a.ui.diffFileList.Delete("1.0", END)
	for i, row := range model.rows {
		if i > 0 {
			a.ui.diffFileList.Insert(END, "\n")
		}
		lineNo := i + 1
		a.ui.diffFileList.Insert(END, row.label)
		a.applyDiffFileRowTags(lineNo, row)
	}
	a.ui.diffFileList.Configure(State("disabled"))
	a.syncFileSelectionToDiff()
}

func (a *Controller) applyDiffFileRowTags(lineNo int, row diffViewRow) {
	a.ui.diffFileList.TagAdd(
		"diffFileAddCount",
		fmt.Sprintf("%d.%d", lineNo, row.addStart),
		fmt.Sprintf("%d.%d", lineNo, row.addEnd),
	)
	a.ui.diffFileList.TagAdd(
		"diffFileDelCount",
		fmt.Sprintf("%d.%d", lineNo, row.delStart),
		fmt.Sprintf("%d.%d", lineNo, row.delEnd),
	)
}

func (a *Controller) onFileSelectionChanged(e *Event) {
	if a.model.state.diff.suppressFileSelection {
		return
	}
	if len(a.model.state.diff.fileSections) == 0 {
		return
	}
	idx, ok := a.diffFileListIndexAtY(e)
	if !ok {
		return
	}
	a.setFileListSelection(idx)
	line, ok := diffSectionLine(a.model.state.diff.fileSections, idx)
	if !ok {
		return
	}
	a.model.state.diff.skipNextSync = true
	a.scrollDiffToLine(line)
}

func (a *Controller) syncFileSelectionToDiff() {
	if len(a.model.state.diff.fileSections) == 0 {
		return
	}
	if a.model.state.diff.skipNextSync {
		return
	}
	line := a.diffTopLine()
	if line <= 0 {
		return
	}
	idx, ok := diffSectionIndexForLine(a.model.state.diff.fileSections, line)
	if !ok {
		return
	}
	a.setFileListSelection(idx)
}

func (a *Controller) setFileListSelection(idx int) {
	if idx < 0 || idx >= len(a.model.state.diff.fileSections) {
		return
	}
	if a.model.state.diff.selectedFileIndex == idx {
		return
	}
	a.model.state.diff.suppressFileSelection = true
	a.ui.diffFileList.TagRemove("diffFileSelected", "1.0", END)
	line := idx + 1
	a.ui.diffFileList.TagAdd(
		"diffFileSelected",
		fmt.Sprintf("%d.0", line),
		fmt.Sprintf("%d.end", line),
	)
	a.ui.diffFileList.See(fmt.Sprintf("%d.0", line))
	a.model.state.diff.selectedFileIndex = idx
	PostEvent(func() {
		a.model.state.diff.suppressFileSelection = false
	}, false)
}

func (a *Controller) diffFileListIndexAtY(e *Event) (int, bool) {
	if e == nil || a.ui.diffFileList == nil {
		return 0, false
	}
	line, ok := textIndexLineNumber(a.ui.diffFileList.Index(fmt.Sprintf("@0,%d", e.Y)))
	if !ok {
		return 0, false
	}
	idx := line - 1
	if idx < 0 || idx >= len(a.model.state.diff.fileSections) {
		return 0, false
	}
	return idx, true
}
