package view

import (
	"fmt"

	"github.com/thiagokokada/gitk-go/internal/gui/model"
	tk "modernc.org/tk9.0"
)

func (a *App) RenderDiffFileList(fileList model.DiffViewModel) {
	if a.DiffFileList == nil {
		return
	}
	a.DiffFileList.Configure(tk.State("normal"))
	a.DiffFileList.Delete("1.0", tk.END)
	for i, row := range fileList.Rows {
		if i > 0 {
			a.DiffFileList.Insert(tk.END, "\n")
		}
		lineNo := i + 1
		a.DiffFileList.Insert(tk.END, row.Label)
		a.applyDiffFileRowTags(lineNo, row)
	}
	a.DiffFileList.Configure(tk.State("disabled"))
}

func (a *App) applyDiffFileRowTags(lineNo int, row model.DiffViewRow) {
	a.DiffFileList.TagAdd(
		"diffFileAddCount",
		fmt.Sprintf("%d.%d", lineNo, row.AddStart),
		fmt.Sprintf("%d.%d", lineNo, row.AddEnd),
	)
	a.DiffFileList.TagAdd(
		"diffFileDelCount",
		fmt.Sprintf("%d.%d", lineNo, row.DelStart),
		fmt.Sprintf("%d.%d", lineNo, row.DelEnd),
	)
}

func (a *App) DiffFileListIndexAtY(y int, sectionCount int) (int, bool) {
	if a.DiffFileList == nil {
		return 0, false
	}
	line, ok := TextIndexLineNumber(a.DiffFileList.Index(fmt.Sprintf("@0,%d", y)))
	if !ok {
		return 0, false
	}
	idx := line - 1
	if idx < 0 || idx >= sectionCount {
		return 0, false
	}
	return idx, true
}

func (a *App) SetDiffFileListSelection(idx int) {
	if a.DiffFileList == nil {
		return
	}
	a.DiffFileList.TagRemove("diffFileSelected", "1.0", tk.END)
	line := idx + 1
	a.DiffFileList.TagAdd("diffFileSelected", textIndex(line, 0), textEndIndex(line))
	a.DiffFileList.See(textIndex(line, 0))
}
