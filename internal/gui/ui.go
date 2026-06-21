package gui

import (
	"fmt"
	"log/slog"
	"strings"

	tk "modernc.org/tk9.0"

	"github.com/thiagokokada/gitk-go/internal/gui/model"
	"github.com/thiagokokada/gitk-go/internal/gui/view"
	"github.com/thiagokokada/gitk-go/internal/gui/widgets"
)

func (a *Controller) buildUI() {
	a.initMenubar()
	a.ui.Build(view.Config{
		GraphCanvas:        a.cfg.graphCanvas,
		DiffDetailFontSpec: diffDetailFontSpec(),
	}, view.Handlers{
		ApplyFilter:       a.scheduleFilterApply,
		ClearFilter:       func() { a.applyFilterImmediate("") },
		Reload:            a.onReloadButton,
		TreeSelection:     a.onTreeSelectionChanged,
		TreeScrolled:      a.maybeLoadMoreOnScroll,
		DiffScrolled:      a.onDiffScrolled,
		FileSelection:     a.onFileSelectionChanged,
		ScheduleGraphDraw: a.scheduleGraphCanvasDraw,
	})

	a.updateRepoLabel()
	a.ui.BindFilterEntryShortcuts()
	a.applyTreeRowStyles()
	a.applyDiffTagStyles()
	a.applyDiffFileListStyles()
	a.initGraphCanvas()
	a.initContextMenus()
	a.clearDetailText("Select a commit to view its details.")
	a.bindShortcuts()
}

func (a *Controller) initGraphCanvas() {
	if a.cfg.graphCanvas {
		graphCanvas, err := widgets.NewGraphCanvas(a.ui.GraphCanvas, a.ui.TreeView)
		if err != nil {
			slog.Error("graph canvas init", slog.Any("error", err))
			a.runtime.graphCanvas = nil
		} else {
			a.runtime.graphCanvas = graphCanvas
		}
	} else {
		a.runtime.graphCanvas = nil
	}
	if a.runtime.graphCanvas != nil {
		a.bindGraphCanvasHandlers(a.runtime.graphCanvas)
	}
}

func (a *Controller) showInitialLoadingRow() {
	if len(a.model.Data.Commits) != 0 || len(a.model.Data.Visible) != 0 {
		return
	}
	if a.model.State.Tree.Rows.HasSpecialItem(model.LoadingIndicatorID) {
		return
	}
	a.ensureLoadingIndicatorRow()
	a.scheduleGraphCanvasDraw()
}

func (a *Controller) initContextMenus() {
	a.ui.InitContextMenus(view.ContextMenuHandlers{
		Tree:                            a.showTreeContextMenu,
		CopyCommitReference:             a.copySelectedCommitReference,
		Diff:                            a.showDiffContextMenu,
		CopySelection:                   func() { a.copyDetailSelection(false) },
		CopySelectionWithoutLineMarkers: func() { a.copyDetailSelection(true) },
		DiffFile:                        a.showDiffFileListContextMenu,
		CopyFilePath:                    a.copySelectedDiffFilePath,
	})
}

func (a *Controller) bindGraphCanvasHandlers(graphCanvas *widgets.GraphCanvas) {
	graphCanvas.SetHandlers(widgets.GraphCanvasHandlers{
		OnClick: func(x, y int) {
			a.handleGraphCanvasClick(x, y)
		},
		OnDoubleClick: func(x, y int) {
			a.handleGraphCanvasClick(x, y)
		},
		OnContextMenu: func(x, y, xRoot, yRoot int) {
			a.showTreeContextMenuAt(x, y, xRoot, yRoot)
		},
		OnWheel: func(delta int) {
			a.handleGraphCanvasWheel(delta)
		},
	})
}

func (a *Controller) handleGraphCanvasClick(x, y int) {
	item := a.ui.FocusTreeRowAt(x, y)
	if item == "" {
		return
	}
}

func (a *Controller) handleGraphCanvasWheel(delta int) {
	if delta == 0 {
		return
	}
	steps := delta / 120
	if steps == 0 {
		if delta > 0 {
			steps = 1
		} else {
			steps = -1
		}
	}
	a.scrollTreeUnits(-steps)
}

func (a *Controller) showTreeContextMenu(e *tk.Event) {
	if e == nil {
		return
	}
	a.showTreeContextMenuAt(e.X, e.Y, e.XRoot, e.YRoot)
}

func (a *Controller) showTreeContextMenuAt(x, y, xRoot, yRoot int) {
	item := a.ui.TreeRowAt(x, y)
	if _, ok := a.treeCommitIndex(item); !ok {
		return
	}
	a.ui.SelectTreeRow(item)
	a.model.State.Tree.ContextTargetID = item
	a.ui.PopupTreeContextMenu(xRoot, yRoot)
}

func (a *Controller) copySelectedCommitReference() {
	id := a.model.State.Tree.ContextTargetID
	if id == "" {
		id = a.ui.SelectedTreeRow()
	}
	idx, ok := a.treeCommitIndex(id)
	if !ok {
		return
	}
	entry := a.model.Data.Visible[idx]
	if entry.Commit.Hash == "" {
		return
	}
	hash := entry.Commit.Hash
	view.CopyToClipboard(hash)
	a.setStatus(fmt.Sprintf("Copied %s to clipboard.", hash))
}

func (a *Controller) updateRepoLabel() {
	label := fmt.Sprintf("Repository: %s", a.model.Repo.Path)
	a.ui.SetRepoLabel(label)
}

func (a *Controller) showDiffFileListContextMenu(e *tk.Event) {
	if e == nil {
		return
	}
	idx, ok := a.ui.DiffFileListIndexAtY(e.Y, len(a.model.State.Diff.FileSections))
	if !ok {
		return
	}
	if _, ok := model.DiffFilePathForIndex(a.model.State.Diff.FileSections, idx); !ok {
		return
	}
	a.setFileListSelection(idx)
	a.ui.PopupDiffFileContextMenu(e.XRoot, e.YRoot)
}

func (a *Controller) copySelectedDiffFilePath() {
	path, ok := a.model.State.Diff.SelectedFilePath()
	if !ok {
		return
	}
	view.CopyToClipboard(path)
	a.setStatus(fmt.Sprintf("Copied %s to clipboard.", path))
}

func (a *Controller) showDiffContextMenu(e *tk.Event) {
	if e == nil {
		return
	}
	a.ui.PopupDiffContextMenu(e.XRoot, e.YRoot)
}

func (a *Controller) treeCommitIndex(id string) (int, bool) {
	_, idx, ok := a.model.CommitEntryForTreeID(strings.TrimSpace(id))
	return idx, ok
}
