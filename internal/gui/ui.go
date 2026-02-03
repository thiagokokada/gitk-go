package gui

import (
	"fmt"
	"log/slog"
	"strings"

	. "modernc.org/tk9.0"

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

	a.ui.status = App.TLabel(Anchor(W), Relief(SUNKEN), Padding("4p"), Font(DefaultFont))
	Grid(a.ui.status, Row(2), Column(0), Sticky(WE))

	a.clearDetailText("Select a commit to view its details.")
	a.bindShortcuts()
}

func (a *Controller) buildControls() *TFrameWidget {
	controls := App.TFrame(Padding("4p"))
	GridColumnConfigure(controls.Window, 1, Weight(1))

	a.ui.repoLabel = controls.TLabel(Anchor(W), Font(DefaultFont))
	a.updateRepoLabel()
	Grid(a.ui.repoLabel, Row(0), Column(0), Columnspan(4), Sticky(W))

	Grid(controls.TLabel(Txt("Filter:"), Anchor(E), Font(DefaultFont)), Row(1), Column(0), Sticky(E))
	a.ui.filterEntry = controls.TEntry(Width(40), Textvariable(""))
	Grid(a.ui.filterEntry, Row(1), Column(1), Sticky(WE), Padx("4p"))

	Bind(a.ui.filterEntry, "<KeyRelease>", Command(func() {
		a.scheduleFilterApply(a.ui.filterEntry.Textvariable())
	}))
	a.bindEmacsEntryShortcuts(a.ui.filterEntry)

	clearBtn := controls.TButton(Txt("Clear"), Command(func() {
		a.ui.filterEntry.Configure(Textvariable(""))
		a.applyFilterImmediate("")
	}))
	Grid(clearBtn, Row(1), Column(2), Sticky(E), Padx("4p"))
	a.ui.reloadButton = controls.TButton(Txt("Reload"), Command(a.onReloadButton))
	Grid(a.ui.reloadButton, Row(1), Column(3), Sticky(E))
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
		a.ui.graphCanvas = listArea.Canvas(Width(120), Highlightthickness(0), Borderwidth(0))
	} else {
		a.ui.graphCanvas = nil
	}
	a.ui.treeView = listArea.TTreeview(
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
		a.ui.treeView.Column("graph", Anchor(W), Width(260), Stretch(false))
	} else {
		a.ui.treeView.Column("graph", Anchor(W), Width(120), Stretch(false))
	}
	a.ui.treeView.Column("commit", Anchor(W), Width(380))
	a.ui.treeView.Column("author", Anchor(W), Width(280))
	a.ui.treeView.Column("date", Anchor(W), Width(180))
	a.ui.treeView.Heading("graph", Txt("Graph"))
	a.ui.treeView.Heading("commit", Txt("Commit"))
	a.ui.treeView.Heading("author", Txt("Author"))
	a.ui.treeView.Heading("date", Txt("Date"))
	a.applyTreeRowStyles()
	Grid(a.ui.treeView, Row(0), Column(0), Sticky(NEWS))
	Grid(treeScroll, Row(0), Column(1), Sticky(NS))
	treeScroll.Configure(Command(func(e *Event) {
		e.Yview(a.ui.treeView)
		a.scheduleGraphCanvasDraw()
	}))

	if a.cfg.graphCanvas {
		graphCanvas, err := widgets.NewGraphCanvas(a.ui.graphCanvas, a.ui.treeView)
		if err != nil {
			slog.Error("graph canvas init", slog.Any("error", err))
			a.state.tree.graphCanvas = nil
		} else {
			a.state.tree.graphCanvas = graphCanvas
		}
	} else {
		a.state.tree.graphCanvas = nil
	}
	if a.state.tree.graphCanvas != nil {
		a.bindGraphCanvasHandlers(a.state.tree.graphCanvas)
	}

	Bind(a.ui.treeView, "<<TreeviewSelect>>", Command(a.onTreeSelectionChanged))
	if a.cfg.graphCanvas {
		Bind(a.ui.treeView, "<Configure>", Command(a.scheduleGraphCanvasDraw))
		// Column resizing uses click+drag on the header separator; it doesn't reliably
		// trigger <Configure>, so watch for B1 drag/release too.
		Bind(a.ui.treeView, "<B1-Motion>", Command(a.scheduleGraphCanvasDraw))
		Bind(a.ui.treeView, "<ButtonRelease-1>", Command(a.scheduleGraphCanvasDraw))
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

	detailYScroll := textFrame.TScrollbar(Command(func(e *Event) { e.Yview(a.ui.diffDetail) }))
	detailXScroll := textFrame.TScrollbar(Orient(HORIZONTAL), Command(func(e *Event) { e.Xview(a.ui.diffDetail) }))
	a.ui.diffDetail = textFrame.Text(
		Wrap(NONE),
		Font(diffDetailFontSpec()...),
		Exportselection(false),
		Tabs("1c"),
	)
	a.ui.diffDetail.Configure(Yscrollcommand(func(e *Event) {
		e.ScrollSet(detailYScroll)
		a.onDiffScrolled()
	}))
	a.ui.diffDetail.Configure(Xscrollcommand(func(e *Event) { e.ScrollSet(detailXScroll) }))
	a.applyDiffTagStyles()
	Grid(a.ui.diffDetail, Row(0), Column(0), Sticky(NEWS))
	Grid(detailYScroll, Row(0), Column(1), Sticky(NS))
	Grid(detailXScroll, Row(1), Column(0), Sticky(WE))
	a.ui.diffDetail.Configure(State("disabled"))
	a.initDiffContextMenu()
	a.bindDiffContextMenu()

	fileScroll := fileFrame.TScrollbar()
	a.ui.diffFileList = fileFrame.Listbox(Exportselection(false), Width(40))
	a.ui.diffFileList.Configure(Yscrollcommand(func(e *Event) { e.ScrollSet(fileScroll) }))
	Grid(a.ui.diffFileList, Row(0), Column(0), Sticky(NEWS))
	Grid(fileScroll, Row(0), Column(1), Sticky(NS))
	fileScroll.Configure(Command(func(e *Event) { e.Yview(a.ui.diffFileList) }))
	Bind(a.ui.diffFileList, "<<ListboxSelect>>", Command(a.onFileSelectionChanged))
	a.initDiffFileListContextMenu()
	a.bindDiffFileListContextMenu()
}

func (a *Controller) showInitialLoadingRow() {
	if len(a.data.commits) != 0 || len(a.data.visible) != 0 {
		return
	}
	if a.state.tree.rows.hasSpecialItem(loadingIndicatorID) {
		return
	}
	a.ensureLoadingIndicatorRow()
	a.scheduleGraphCanvasDraw()
}

func (a *Controller) initTreeContextMenu() {
	menu := App.Menu(Tearoff(false))
	item := menu.AddCommand(Command(a.copySelectedCommitReference))
	menu.EntryConfigure(item, Lbl("Copy commit reference"))
	a.ui.treeContextMenu = menu
}

func (a *Controller) bindTreeContextMenu() {
	handler := func(e *Event) {
		a.showTreeContextMenu(e)
	}
	Bind(a.ui.treeView, "<Button-2>", Command(handler))
	Bind(a.ui.treeView, "<Button-3>", Command(handler))
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
	if a.ui.treeView == nil {
		return
	}
	item := strings.TrimSpace(a.ui.treeView.IdentifyItem(x, y))
	if item == "" {
		return
	}
	Focus(a.ui.treeView)
	a.ui.treeView.Selection("set", item)
	a.ui.treeView.Focus(item)
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
	item := strings.TrimSpace(a.ui.treeView.IdentifyItem(x, y))
	if _, ok := a.treeCommitIndex(item); !ok {
		return
	}
	a.ui.treeView.Selection("set", item)
	a.ui.treeView.Focus(item)
	a.state.tree.contextTargetID = item
	Popup(a.ui.treeContextMenu.Window, xRoot, yRoot, nil)
}

func (a *Controller) copySelectedCommitReference() {
	id := a.state.tree.contextTargetID
	if id == "" {
		if sel := a.ui.treeView.Selection(""); len(sel) > 0 {
			id = sel[0]
		}
	}
	idx, ok := a.treeCommitIndex(id)
	if !ok {
		return
	}
	entry := a.data.visible[idx]
	if entry == nil || entry.Commit == nil {
		return
	}
	hash := entry.Commit.Hash
	ClipboardClear()
	ClipboardAppend(hash)
	a.setStatus(fmt.Sprintf("Copied %s to clipboard.", hash))
}

func (a *Controller) updateRepoLabel() {
	label := fmt.Sprintf("Repository: %s", a.repo.path)
	a.ui.repoLabel.Configure(Txt(label))
}

func (a *Controller) initDiffFileListContextMenu() {
	menu := App.Menu(Tearoff(false))
	menu.AddCommand(Lbl("Copy file path"), Command(a.copySelectedDiffFilePath))
	a.ui.diffFileContextMenu = menu
}

func (a *Controller) bindDiffFileListContextMenu() {
	handler := func(e *Event) {
		a.showDiffFileListContextMenu(e)
	}
	Bind(a.ui.diffFileList, "<Button-2>", Command(handler))
	Bind(a.ui.diffFileList, "<Button-3>", Command(handler))
}

func (a *Controller) showDiffFileListContextMenu(e *Event) {
	if e == nil {
		return
	}
	idx := a.ui.diffFileList.Nearest(e.Y)
	if _, ok := diffFilePathForIndex(a.state.diff.fileSections, idx); !ok {
		return
	}
	a.ui.diffFileList.SelectionClear(0, END)
	a.ui.diffFileList.SelectionSet(idx)
	a.ui.diffFileList.Activate(idx)
	Popup(a.ui.diffFileContextMenu.Window, e.XRoot, e.YRoot, nil)
}

func (a *Controller) copySelectedDiffFilePath() {
	selection := a.ui.diffFileList.Curselection()
	if len(selection) == 0 {
		return
	}
	path, ok := diffFilePathForIndex(a.state.diff.fileSections, selection[0])
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
	a.ui.diffContextMenu = menu
}

func (a *Controller) bindDiffContextMenu() {
	handler := func(e *Event) {
		a.showDiffContextMenu(e)
	}
	Bind(a.ui.diffDetail, "<Button-2>", Command(handler))
	Bind(a.ui.diffDetail, "<Button-3>", Command(handler))
}

func (a *Controller) showDiffContextMenu(e *Event) {
	if e == nil {
		return
	}
	Popup(a.ui.diffContextMenu.Window, e.XRoot, e.YRoot, nil)
}

func (a *Controller) treeCommitIndex(id string) (int, bool) {
	_, idx, ok := a.commitEntryForTreeID(id)
	return idx, ok
}
