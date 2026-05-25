package gui

import (
	"fmt"
	"log/slog"
	"strings"

	. "modernc.org/tk9.0"

	"github.com/thiagokokada/gitk-go/internal/gui/model"
	"github.com/thiagokokada/gitk-go/internal/gui/tkutil"
	"github.com/thiagokokada/gitk-go/internal/gui/widgets"
)

func (a *Controller) buildUI() {
	a.initMenubar()
	GridColumnConfigure(App, 0, Weight(1))
	GridRowConfigure(App, 1, Weight(1))

	controls := a.buildControls()
	Grid(controls, Row(0), Column(0), Sticky(WE))

	mainPane := a.buildMainPane()
	Grid(mainPane, Row(1), Column(0), Sticky(NEWS), Padx("4p"), Pady("4p"))

	a.ui.Status = App.TLabel(Anchor(W), Relief(SUNKEN), Padding("4p"), Font(DefaultFont))
	Grid(a.ui.Status, Row(2), Column(0), Sticky(WE))

	a.clearDetailText("Select a commit to view its details.")
	a.bindShortcuts()
}

func (a *Controller) buildControls() *TFrameWidget {
	controls := App.TFrame(Padding("4p"))
	GridColumnConfigure(controls.Window, 1, Weight(1))

	a.ui.RepoLabel = controls.TLabel(Anchor(W), Font(DefaultFont))
	a.updateRepoLabel()
	Grid(a.ui.RepoLabel, Row(0), Column(0), Columnspan(4), Sticky(W))

	Grid(controls.TLabel(Txt("Filter:"), Anchor(E), Font(DefaultFont)), Row(1), Column(0), Sticky(E))
	a.ui.FilterEntry = controls.TEntry(Width(40), Textvariable(""))
	Grid(a.ui.FilterEntry, Row(1), Column(1), Sticky(WE), Padx("4p"))

	Bind(a.ui.FilterEntry, "<KeyRelease>", Command(func() {
		a.scheduleFilterApply(a.ui.FilterEntry.Textvariable())
	}))
	a.bindEmacsEntryShortcuts(a.ui.FilterEntry)

	clearBtn := controls.TButton(Txt("Clear"), Command(func() {
		a.ui.FilterEntry.Configure(Textvariable(""))
		a.applyFilterImmediate("")
	}))
	Grid(clearBtn, Row(1), Column(2), Sticky(E), Padx("4p"))
	a.ui.ReloadButton = controls.TButton(Txt("Reload"), Command(a.onReloadButton))
	Grid(a.ui.ReloadButton, Row(1), Column(3), Sticky(E))
	return controls
}

func (a *Controller) buildMainPane() *TPanedwindowWidget {
	pane := App.TPanedwindow(Orient(VERTICAL))
	listArea := pane.TFrame()
	diffArea := pane.TFrame()
	pane.Add(listArea.Window)
	pane.Add(diffArea.Window)

	a.buildCommitPane(listArea)
	a.buildDiffPane(diffArea)

	// Keep the first pane at ~25% of the total height on first configure.
	var (
		lastHeight  int
		stableCount int
		pending     bool
	)
	setSashOnce := func() {
		if pending {
			return
		}
		pending = true
		TclAfterIdle(func() {
			pending = false
			height := tkutil.Atoi(WinfoHeight(pane.Window))
			if height <= 1 {
				return
			}
			if height == lastHeight {
				stableCount++
			} else {
				stableCount = 0
				lastHeight = height
			}
			target := (height + 2) / 4
			tkutil.MustEvalf("%s sashpos 0 %d", pane, target)
			if stableCount >= 1 {
				Bind(pane, "<Configure>", "")
			}
		})
	}
	Bind(pane, "<Configure>", Command(func(e *Event) {
		setSashOnce()
	}))

	return pane
}

func (a *Controller) buildCommitPane(listArea *TFrameWidget) {
	GridRowConfigure(listArea.Window, 0, Weight(1))
	GridRowConfigure(listArea.Window, 1, Weight(0))
	GridColumnConfigure(listArea.Window, 0, Weight(1))
	GridColumnConfigure(listArea.Window, 1, Weight(0))

	treeScroll := listArea.TScrollbar()
	if a.cfg.graphCanvas {
		// Avoid setting Background(""): Tk treats it as an invalid color name.
		a.ui.GraphCanvas = listArea.Canvas(Width(120), Highlightthickness(0), Borderwidth(0))
	} else {
		a.ui.GraphCanvas = nil
	}
	a.ui.TreeView = listArea.TTreeview(
		Show("headings"),
		Columns("graph commit author date"),
		Selectmode("browse"),
		Height(18),
		Yscrollcommand(func(e *Event) {
			e.ScrollSet(treeScroll)
			a.maybeLoadMoreOnScroll()
			a.scheduleGraphCanvasDraw()
		}),
	)
	if a.cfg.graphCanvas {
		a.ui.TreeView.Column("graph", Anchor(W), Width(260), Stretch(false))
	} else {
		a.ui.TreeView.Column("graph", Anchor(W), Width(120), Stretch(false))
	}
	a.ui.TreeView.Column("commit", Anchor(W), Width(380))
	a.ui.TreeView.Column("author", Anchor(W), Width(280))
	a.ui.TreeView.Column("date", Anchor(W), Width(180))
	a.ui.TreeView.Heading("graph", Txt("Graph"))
	a.ui.TreeView.Heading("commit", Txt("Commit"))
	a.ui.TreeView.Heading("author", Txt("Author"))
	a.ui.TreeView.Heading("date", Txt("Date"))
	a.applyTreeRowStyles()
	Grid(a.ui.TreeView, Row(0), Column(0), Sticky(NEWS))
	Grid(treeScroll, Row(0), Column(1), Sticky(NS))
	treeScroll.Configure(Command(func(e *Event) {
		e.Yview(a.ui.TreeView)
		a.scheduleGraphCanvasDraw()
	}))

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

	Bind(a.ui.TreeView, "<<TreeviewSelect>>", Command(a.onTreeSelectionChanged))
	if a.cfg.graphCanvas {
		Bind(a.ui.TreeView, "<Configure>", Command(a.scheduleGraphCanvasDraw))
		// Column resizing uses click+drag on the header separator; it doesn't reliably
		// trigger <Configure>, so watch for B1 drag/release too.
		Bind(a.ui.TreeView, "<B1-Motion>", Command(a.scheduleGraphCanvasDraw))
		Bind(a.ui.TreeView, "<ButtonRelease-1>", Command(a.scheduleGraphCanvasDraw))
	}
	a.initTreeContextMenu()
	a.bindTreeContextMenu()
}

func (a *Controller) buildDiffPane(diffArea *TFrameWidget) {
	GridRowConfigure(diffArea.Window, 0, Weight(1))
	GridColumnConfigure(diffArea.Window, 0, Weight(1))

	diffPane := diffArea.TPanedwindow(Orient(HORIZONTAL))
	Grid(diffPane, Row(0), Column(0), Sticky(NEWS))

	textFrame := diffPane.TFrame()
	fileFrame := diffPane.TFrame()
	diffPane.Add(textFrame.Window, Weight(5))
	diffPane.Add(fileFrame.Window, Weight(1))

	GridRowConfigure(fileFrame.Window, 0, Weight(1))
	GridColumnConfigure(fileFrame.Window, 0, Weight(1))
	GridRowConfigure(textFrame.Window, 0, Weight(1))
	GridColumnConfigure(textFrame.Window, 0, Weight(1))

	detailYScroll := textFrame.TScrollbar(Command(func(e *Event) { e.Yview(a.ui.DiffDetail) }))
	detailXScroll := textFrame.TScrollbar(Orient(HORIZONTAL), Command(func(e *Event) { e.Xview(a.ui.DiffDetail) }))
	a.ui.DiffDetail = textFrame.Text(
		Wrap(NONE),
		Font(diffDetailFontSpec()...),
		Exportselection(false),
		Tabs("1c"),
	)
	a.ui.DiffDetail.Configure(Yscrollcommand(func(e *Event) {
		e.ScrollSet(detailYScroll)
		a.onDiffScrolled()
	}))
	a.ui.DiffDetail.Configure(Xscrollcommand(func(e *Event) { e.ScrollSet(detailXScroll) }))
	a.applyDiffTagStyles()
	Grid(a.ui.DiffDetail, Row(0), Column(0), Sticky(NEWS))
	Grid(detailYScroll, Row(0), Column(1), Sticky(NS))
	Grid(detailXScroll, Row(1), Column(0), Sticky(WE))
	a.ui.DiffDetail.Configure(State("disabled"))
	a.initDiffContextMenu()
	a.bindDiffContextMenu()

	fileScroll := fileFrame.TScrollbar()
	a.ui.DiffFileList = fileFrame.Text(
		Wrap(NONE),
		Width(40),
		Exportselection(false),
	)
	a.ui.DiffFileList.Configure(Yscrollcommand(func(e *Event) { e.ScrollSet(fileScroll) }))
	a.ui.DiffFileList.Configure(State("disabled"))
	Grid(a.ui.DiffFileList, Row(0), Column(0), Sticky(NEWS))
	Grid(fileScroll, Row(0), Column(1), Sticky(NS))
	fileScroll.Configure(Command(func(e *Event) { e.Yview(a.ui.DiffFileList) }))
	a.applyDiffFileListStyles()
	Bind(a.ui.DiffFileList, "<Button-1>", Command(a.onFileSelectionChanged))
	a.initDiffFileListContextMenu()
	a.bindDiffFileListContextMenu()
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
}

func (a *Controller) bindTreeContextMenu() {
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
}

func (a *Controller) bindDiffFileListContextMenu() {
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
}

func (a *Controller) bindDiffContextMenu() {
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
