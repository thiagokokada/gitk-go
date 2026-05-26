package gui

import (
	"fmt"
	"log/slog"
	"strings"

	. "modernc.org/tk9.0"

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
	a.bindEmacsEntryShortcuts(a.ui.FilterEntry)
	a.applyTreeRowStyles()
	a.applyDiffTagStyles()
	a.applyDiffFileListStyles()
	a.initGraphCanvas()
	a.initTreeContextMenu()
	a.initDiffContextMenu()
	a.initDiffFileListContextMenu()
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

func (a *Controller) initTreeContextMenu() {
	menu := App.Menu(Tearoff(false))
	item := menu.AddCommand(Command(a.copySelectedCommitReference))
	menu.EntryConfigure(item, Lbl("Copy commit reference"))
	a.ui.TreeContextMenu = menu
	handler := func(e *Event) {
		a.showTreeContextMenu(e)
	}
	Bind(a.ui.TreeView, "<Button-2>", Command(handler))
	Bind(a.ui.TreeView, "<Button-3>", Command(handler))
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
	if a.ui.TreeView == nil {
		return
	}
	item := strings.TrimSpace(a.ui.TreeView.IdentifyItem(x, y))
	if item == "" {
		return
	}
	Focus(a.ui.TreeView)
	a.ui.TreeView.Selection("set", item)
	a.ui.TreeView.Focus(item)
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

func (a *Controller) showTreeContextMenu(e *Event) {
	if e == nil {
		return
	}
	a.showTreeContextMenuAt(e.X, e.Y, e.XRoot, e.YRoot)
}

func (a *Controller) showTreeContextMenuAt(x, y, xRoot, yRoot int) {
	item := strings.TrimSpace(a.ui.TreeView.IdentifyItem(x, y))
	if _, ok := a.treeCommitIndex(item); !ok {
		return
	}
	a.ui.TreeView.Selection("set", item)
	a.ui.TreeView.Focus(item)
	a.model.State.Tree.ContextTargetID = item
	Popup(a.ui.TreeContextMenu.Window, xRoot, yRoot, nil)
}

func (a *Controller) copySelectedCommitReference() {
	id := a.model.State.Tree.ContextTargetID
	if id == "" {
		if sel := a.ui.TreeView.Selection(""); len(sel) > 0 {
			id = sel[0]
		}
	}
	idx, ok := a.treeCommitIndex(id)
	if !ok {
		return
	}
	entry := a.model.Data.Visible[idx]
	if entry == nil || entry.Commit == nil {
		return
	}
	hash := entry.Commit.Hash
	ClipboardClear()
	ClipboardAppend(hash)
	a.setStatus(fmt.Sprintf("Copied %s to clipboard.", hash))
}

func (a *Controller) updateRepoLabel() {
	label := fmt.Sprintf("Repository: %s", a.model.Repo.Path)
	a.ui.RepoLabel.Configure(Txt(label))
}

func (a *Controller) initDiffFileListContextMenu() {
	menu := App.Menu(Tearoff(false))
	menu.AddCommand(Lbl("Copy file path"), Command(a.copySelectedDiffFilePath))
	a.ui.DiffFileContextMenu = menu
	handler := func(e *Event) {
		a.showDiffFileListContextMenu(e)
	}
	Bind(a.ui.DiffFileList, "<Button-2>", Command(handler))
	Bind(a.ui.DiffFileList, "<Button-3>", Command(handler))
}

func (a *Controller) showDiffFileListContextMenu(e *Event) {
	if e == nil {
		return
	}
	idx, ok := a.diffFileListIndexAtY(e)
	if !ok {
		return
	}
	if _, ok := model.DiffFilePathForIndex(a.model.State.Diff.FileSections, idx); !ok {
		return
	}
	a.setFileListSelection(idx)
	Popup(a.ui.DiffFileContextMenu.Window, e.XRoot, e.YRoot, nil)
}

func (a *Controller) copySelectedDiffFilePath() {
	path, ok := a.model.State.Diff.SelectedFilePath()
	if !ok {
		return
	}
	ClipboardClear()
	ClipboardAppend(path)
	a.setStatus(fmt.Sprintf("Copied %s to clipboard.", path))
}

func (a *Controller) initDiffContextMenu() {
	menu := App.Menu(Tearoff(false))
	menu.AddCommand(Lbl("Copy selection"), Command(func() { a.copyDetailSelection(false) }))
	menu.AddCommand(Lbl("Copy selection without +/- markers"), Command(func() { a.copyDetailSelection(true) }))
	a.ui.DiffContextMenu = menu
	handler := func(e *Event) {
		a.showDiffContextMenu(e)
	}
	Bind(a.ui.DiffDetail, "<Button-2>", Command(handler))
	Bind(a.ui.DiffDetail, "<Button-3>", Command(handler))
}

func (a *Controller) showDiffContextMenu(e *Event) {
	if e == nil {
		return
	}
	Popup(a.ui.DiffContextMenu.Window, e.XRoot, e.YRoot, nil)
}

func (a *Controller) treeCommitIndex(id string) (int, bool) {
	_, idx, ok := a.model.CommitEntryForTreeID(strings.TrimSpace(id))
	return idx, ok
}
