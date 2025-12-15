package gui

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	. "modernc.org/tk9.0"
)

func (a *Controller) buildUI() {
	a.initMenubar()
	GridColumnConfigure(App, 0, Weight(1))
	GridRowConfigure(App, 1, Weight(1))

	controls := a.buildControls()
	Grid(controls, Row(0), Column(0), Sticky(WE))

	mainPane := a.buildMainPane()
	Grid(mainPane, Row(1), Column(0), Sticky(NEWS), Padx("4p"), Pady("4p"))

	a.ui.status = App.TLabel(Anchor(W), Relief(SUNKEN), Padding("4p"))
	Grid(a.ui.status, Row(2), Column(0), Sticky(WE))

	a.clearDetailText("Select a commit to view its details.")
	a.bindShortcuts()
}

func (a *Controller) buildControls() *TFrameWidget {
	controls := App.TFrame(Padding("4p"))
	GridColumnConfigure(controls.Window, 1, Weight(1))

	a.ui.repoLabel = controls.TLabel(Anchor(W))
	a.updateRepoLabel()
	Grid(a.ui.repoLabel, Row(0), Column(0), Columnspan(4), Sticky(W))

	Grid(controls.TLabel(Txt("Filter:"), Anchor(E)), Row(1), Column(0), Sticky(E))
	a.ui.filterEntry = controls.TEntry(Width(40), Textvariable(""))
	Grid(a.ui.filterEntry, Row(1), Column(1), Sticky(WE), Padx("4p"))

	Bind(a.ui.filterEntry, "<KeyRelease>", Command(func() {
		a.scheduleFilterApply(a.ui.filterEntry.Textvariable())
	}))

	clearBtn := controls.TButton(Txt("Clear"), Command(func() {
		a.ui.filterEntry.Configure(Textvariable(""))
		a.applyFilter("")
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

	// calculates the first widget to be 25% of the total height of the,
	// widget, based in the <Configure> event (that triggers whenever the
	// widget configuration changes)
	PostEvent(
		func() {
			switch runtime.GOOS {
			case "darwin":
				<-time.After(10 * time.Millisecond)
			default:
			}
			tkMustEval(`
				bind %[1]s <Configure> {
					set h [winfo height %[1]s]
					if {$h > 1} {
						%[1]s sashpos 0 [expr {round($h * 0.25)}]
						bind %[1]s <Configure> {}
					}
				}
			`, pane)
		}, false,
	)

	return pane
}

func (a *Controller) buildCommitPane(listArea *TFrameWidget) {
	GridRowConfigure(listArea.Window, 0, Weight(1))
	GridRowConfigure(listArea.Window, 1, Weight(0))
	GridColumnConfigure(listArea.Window, 0, Weight(1))

	treeScroll := listArea.TScrollbar()
	a.ui.treeView = listArea.TTreeview(
		Show("headings"),
		Columns("graph commit author date"),
		Selectmode("browse"),
		Height(18),
		Yscrollcommand(func(e *Event) {
			e.ScrollSet(treeScroll)
			a.maybeLoadMoreOnScroll()
		}),
	)
	a.ui.treeView.Column("graph", Anchor(W), Width(120))
	a.ui.treeView.Column("commit", Anchor(W), Width(380))
	a.ui.treeView.Column("author", Anchor(W), Width(280))
	a.ui.treeView.Column("date", Anchor(W), Width(180))
	a.ui.treeView.Heading("graph", Txt("Graph"))
	a.ui.treeView.Heading("commit", Txt("Commit"))
	a.ui.treeView.Heading("author", Txt("Author"))
	a.ui.treeView.Heading("date", Txt("Date"))
	unstagedColor := a.palette.LocalUnstagedRow
	if unstagedColor == "" {
		unstagedColor = "#fde2e1"
	}
	stagedColor := a.palette.LocalStagedRow
	if stagedColor == "" {
		stagedColor = "#e2f7e1"
	}
	a.ui.treeView.TagConfigure("localUnstaged", Background(unstagedColor))
	a.ui.treeView.TagConfigure("localStaged", Background(stagedColor))
	Grid(a.ui.treeView, Row(0), Column(0), Sticky(NEWS))
	Grid(treeScroll, Row(0), Column(1), Sticky(NS))
	treeScroll.Configure(Command(func(e *Event) { e.Yview(a.ui.treeView) }))

	Bind(a.ui.treeView, "<<TreeviewSelect>>", Command(a.onTreeSelectionChanged))
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
	a.ui.diffDetail = textFrame.Text(Wrap(NONE), Font(CourierFont(), 11), Exportselection(false), Tabs("1c"))
	a.ui.diffDetail.Configure(Yscrollcommand(func(e *Event) {
		e.ScrollSet(detailYScroll)
		a.onDiffScrolled()
	}))
	a.ui.diffDetail.Configure(Xscrollcommand(func(e *Event) { e.ScrollSet(detailXScroll) }))
	addColor := a.palette.DiffAdd
	if addColor == "" {
		addColor = lightPalette.DiffAdd
	}
	delColor := a.palette.DiffDel
	if delColor == "" {
		delColor = lightPalette.DiffDel
	}
	headerColor := a.palette.DiffHeader
	if headerColor == "" {
		headerColor = lightPalette.DiffHeader
	}
	selBg := a.ui.diffDetail.Selectbackground()
	selFg := a.ui.diffDetail.Selectforeground()
	tagOpts := func(bg string) []Opt {
		opts := []Opt{Background(bg)}
		if selBg != "" {
			opts = append(opts, Selectbackground(selBg))
		}
		if selFg != "" {
			opts = append(opts, Selectforeground(selFg))
		}
		return opts
	}
	a.ui.diffDetail.TagConfigure("diffAdd", tagOpts(addColor)...)
	a.ui.diffDetail.TagConfigure("diffDel", tagOpts(delColor)...)
	a.ui.diffDetail.TagConfigure("diffHeader", tagOpts(headerColor)...)
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
}

func (a *Controller) showInitialLoadingRow() {
	if a.ui.treeView == nil {
		return
	}
	if len(a.commits) != 0 || len(a.visible) != 0 {
		return
	}
	if a.treeItemExists(loadingIndicatorID) {
		return
	}
	vals := []string{"", "Loading commits...", "", ""}
	a.ui.treeView.Insert("", "end", Id(loadingIndicatorID), Values(vals))
}

func (a *Controller) initTreeContextMenu() {
	menu := App.Menu(Tearoff(false))
	item := menu.AddCommand(Command(a.copySelectedCommitReference))
	menu.EntryConfigure(item, Lbl("Copy commit reference"))
	a.ui.treeContextMenu = menu
}

func (a *Controller) bindTreeContextMenu() {
	if a.ui.treeView == nil {
		return
	}
	handler := func(e *Event) {
		a.showTreeContextMenu(e)
	}
	Bind(a.ui.treeView, "<Button-2>", Command(handler))
	Bind(a.ui.treeView, "<Button-3>", Command(handler))
}

func (a *Controller) showTreeContextMenu(e *Event) {
	if a.ui.treeContextMenu == nil || a.ui.treeView == nil || e == nil {
		return
	}
	item := strings.TrimSpace(a.ui.treeView.IdentifyItem(e.X, e.Y))
	if _, ok := a.treeCommitIndex(item); !ok {
		return
	}
	a.ui.treeView.Selection("set", item)
	a.ui.treeView.Focus(item)
	a.tree.contextTargetID = item
	Popup(a.ui.treeContextMenu.Window, e.XRoot, e.YRoot, nil)
}

func (a *Controller) copySelectedCommitReference() {
	id := a.tree.contextTargetID
	if id == "" && a.ui.treeView != nil {
		if sel := a.ui.treeView.Selection(""); len(sel) > 0 {
			id = sel[0]
		}
	}
	idx, ok := a.treeCommitIndex(id)
	if !ok {
		return
	}
	entry := a.visible[idx]
	if entry == nil || entry.Commit == nil {
		return
	}
	hash := entry.Commit.Hash.String()
	ClipboardClear()
	ClipboardAppend(hash)
	a.setStatus(fmt.Sprintf("Copied %s to clipboard.", hash))
}

func (a *Controller) updateRepoLabel() {
	if a.ui.repoLabel == nil {
		return
	}
	label := fmt.Sprintf("Repository: %s", a.repoPath)
	a.ui.repoLabel.Configure(Txt(label))
}

func (a *Controller) initDiffContextMenu() {
	menu := App.Menu(Tearoff(false))
	menu.AddCommand(Lbl("Copy selection"), Command(func() { a.copyDetailSelection(false) }))
	menu.AddCommand(Lbl("Copy selection without +/- markers"), Command(func() { a.copyDetailSelection(true) }))
	a.ui.diffContextMenu = menu
}

func (a *Controller) bindDiffContextMenu() {
	if a.ui.diffDetail == nil || a.ui.diffContextMenu == nil {
		return
	}
	handler := func(e *Event) {
		a.showDiffContextMenu(e)
	}
	Bind(a.ui.diffDetail, "<Button-2>", Command(handler))
	Bind(a.ui.diffDetail, "<Button-3>", Command(handler))
}

func (a *Controller) showDiffContextMenu(e *Event) {
	if a.ui.diffContextMenu == nil || a.ui.diffDetail == nil || e == nil {
		return
	}
	Popup(a.ui.diffContextMenu.Window, e.XRoot, e.YRoot, nil)
}

func (a *Controller) treeCommitIndex(id string) (int, bool) {
	if id == "" {
		return 0, false
	}
	idx, err := strconv.Atoi(id)
	if err != nil || idx < 0 || idx >= len(a.visible) {
		return 0, false
	}
	return idx, true
}
