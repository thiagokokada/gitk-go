package gui

import (
	"fmt"
	"strconv"
	"strings"

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

	a.status = App.TLabel(Anchor(W), Relief(SUNKEN), Padding("4p"))
	Grid(a.status, Row(2), Column(0), Sticky(WE))

	a.clearDetailText("Select a commit to view its details.")
	a.bindShortcuts()
}

func (a *Controller) buildControls() *TFrameWidget {
	controls := App.TFrame(Padding("4p"))
	GridColumnConfigure(controls.Window, 1, Weight(1))

	a.repoLabel = controls.TLabel(Anchor(W))
	a.updateRepoLabel()
	Grid(a.repoLabel, Row(0), Column(0), Columnspan(4), Sticky(W))

	Grid(controls.TLabel(Txt("Filter:"), Anchor(E)), Row(1), Column(0), Sticky(E))
	a.filter.entry = controls.TEntry(Width(40), Textvariable(""))
	Grid(a.filter.entry, Row(1), Column(1), Sticky(WE), Padx("4p"))

	Bind(a.filter.entry, "<KeyRelease>", Command(func() {
		a.scheduleFilterApply(a.filter.entry.Textvariable())
	}))

	clearBtn := controls.TButton(Txt("Clear"), Command(func() {
		a.filter.entry.Configure(Textvariable(""))
		a.applyFilter("")
	}))
	Grid(clearBtn, Row(1), Column(2), Sticky(E), Padx("4p"))
	a.watch.button = controls.TButton(Txt("Reload"), Command(a.onReloadButton))
	Grid(a.watch.button, Row(1), Column(3), Sticky(E))
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
	// PostEvent doesn't work here because in X11 this widget seems to not
	// have finish initialisation when PostEvent runs
	tkMustEval(`
		bind %[1]s <Configure> {
			set h [winfo height %[1]s]
			if {$h > 1} {
				%[1]s sashpos 0 [expr {round($h * 0.25)}]
				bind %[1]s <Configure> {}
			}
		}
	`, pane)

	return pane
}

func (a *Controller) buildCommitPane(listArea *TFrameWidget) {
	GridRowConfigure(listArea.Window, 0, Weight(1))
	GridRowConfigure(listArea.Window, 1, Weight(0))
	GridColumnConfigure(listArea.Window, 0, Weight(1))

	treeScroll := listArea.TScrollbar()
	a.tree.widget = listArea.TTreeview(
		Show("headings"),
		Columns("graph commit author date"),
		Selectmode("browse"),
		Height(18),
		Yscrollcommand(func(e *Event) {
			e.ScrollSet(treeScroll)
			a.maybeLoadMoreOnScroll()
		}),
	)
	a.tree.widget.Column("graph", Anchor(W), Width(120))
	a.tree.widget.Column("commit", Anchor(W), Width(380))
	a.tree.widget.Column("author", Anchor(W), Width(280))
	a.tree.widget.Column("date", Anchor(W), Width(180))
	a.tree.widget.Heading("graph", Txt("Graph"))
	a.tree.widget.Heading("commit", Txt("Commit"))
	a.tree.widget.Heading("author", Txt("Author"))
	a.tree.widget.Heading("date", Txt("Date"))
	unstagedColor := a.palette.LocalUnstagedRow
	if unstagedColor == "" {
		unstagedColor = "#fde2e1"
	}
	stagedColor := a.palette.LocalStagedRow
	if stagedColor == "" {
		stagedColor = "#e2f7e1"
	}
	a.tree.widget.TagConfigure("localUnstaged", Background(unstagedColor))
	a.tree.widget.TagConfigure("localStaged", Background(stagedColor))
	Grid(a.tree.widget, Row(0), Column(0), Sticky(NEWS))
	Grid(treeScroll, Row(0), Column(1), Sticky(NS))
	treeScroll.Configure(Command(func(e *Event) { e.Yview(a.tree.widget) }))

	Bind(a.tree.widget, "<<TreeviewSelect>>", Command(a.onTreeSelectionChanged))
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

	detailYScroll := textFrame.TScrollbar(Command(func(e *Event) { e.Yview(a.diff.detail) }))
	detailXScroll := textFrame.TScrollbar(Orient(HORIZONTAL), Command(func(e *Event) { e.Xview(a.diff.detail) }))
	a.diff.detail = textFrame.Text(Wrap(NONE), Font(CourierFont(), 11), Exportselection(false), Tabs("1c"))
	a.diff.detail.Configure(Yscrollcommand(func(e *Event) {
		e.ScrollSet(detailYScroll)
		a.onDiffScrolled()
	}))
	a.diff.detail.Configure(Xscrollcommand(func(e *Event) { e.ScrollSet(detailXScroll) }))
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
	selBg := a.diff.detail.Selectbackground()
	selFg := a.diff.detail.Selectforeground()
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
	a.diff.detail.TagConfigure("diffAdd", tagOpts(addColor)...)
	a.diff.detail.TagConfigure("diffDel", tagOpts(delColor)...)
	a.diff.detail.TagConfigure("diffHeader", tagOpts(headerColor)...)
	Grid(a.diff.detail, Row(0), Column(0), Sticky(NEWS))
	Grid(detailYScroll, Row(0), Column(1), Sticky(NS))
	Grid(detailXScroll, Row(1), Column(0), Sticky(WE))
	a.diff.detail.Configure(State("disabled"))
	a.initDiffContextMenu()
	a.bindDiffContextMenu()

	fileScroll := fileFrame.TScrollbar()
	a.diff.fileList = fileFrame.Listbox(Exportselection(false), Width(40))
	a.diff.fileList.Configure(Yscrollcommand(func(e *Event) { e.ScrollSet(fileScroll) }))
	Grid(a.diff.fileList, Row(0), Column(0), Sticky(NEWS))
	Grid(fileScroll, Row(0), Column(1), Sticky(NS))
	fileScroll.Configure(Command(func(e *Event) { e.Yview(a.diff.fileList) }))
	Bind(a.diff.fileList, "<<ListboxSelect>>", Command(a.onFileSelectionChanged))
}

func (a *Controller) showInitialLoadingRow() {
	if a.tree.widget == nil {
		return
	}
	if len(a.commits) != 0 || len(a.visible) != 0 {
		return
	}
	if a.treeItemExists(loadingIndicatorID) {
		return
	}
	vals := []string{"", "Loading commits...", "", ""}
	a.tree.widget.Insert("", "end", Id(loadingIndicatorID), Values(vals))
}

func (a *Controller) initTreeContextMenu() {
	menu := App.Menu(Tearoff(false))
	item := menu.AddCommand(Command(a.copySelectedCommitReference))
	menu.EntryConfigure(item, Lbl("Copy commit reference"))
	a.tree.menu = menu
}

func (a *Controller) bindTreeContextMenu() {
	if a.tree.widget == nil {
		return
	}
	handler := func(e *Event) {
		a.showTreeContextMenu(e)
	}
	Bind(a.tree.widget, "<Button-2>", Command(handler))
	Bind(a.tree.widget, "<Button-3>", Command(handler))
}

func (a *Controller) showTreeContextMenu(e *Event) {
	if a.tree.menu == nil || a.tree.widget == nil || e == nil {
		return
	}
	item := strings.TrimSpace(a.tree.widget.IdentifyItem(e.X, e.Y))
	if _, ok := a.treeCommitIndex(item); !ok {
		return
	}
	a.tree.widget.Selection("set", item)
	a.tree.widget.Focus(item)
	a.tree.contextTargetID = item
	Popup(a.tree.menu.Window, e.XRoot, e.YRoot, nil)
}

func (a *Controller) copySelectedCommitReference() {
	id := a.tree.contextTargetID
	if id == "" && a.tree.widget != nil {
		if sel := a.tree.widget.Selection(""); len(sel) > 0 {
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
	if a.repoLabel == nil {
		return
	}
	label := fmt.Sprintf("Repository: %s", a.repoPath)
	a.repoLabel.Configure(Txt(label))
}

func (a *Controller) initDiffContextMenu() {
	menu := App.Menu(Tearoff(false))
	menu.AddCommand(Lbl("Copy selection"), Command(func() { a.copyDetailSelection(false) }))
	menu.AddCommand(Lbl("Copy selection without +/- markers"), Command(func() { a.copyDetailSelection(true) }))
	a.diff.menu = menu
}

func (a *Controller) bindDiffContextMenu() {
	if a.diff.detail == nil || a.diff.menu == nil {
		return
	}
	handler := func(e *Event) {
		a.showDiffContextMenu(e)
	}
	Bind(a.diff.detail, "<Button-2>", Command(handler))
	Bind(a.diff.detail, "<Button-3>", Command(handler))
}

func (a *Controller) showDiffContextMenu(e *Event) {
	if a.diff.menu == nil || a.diff.detail == nil || e == nil {
		return
	}
	Popup(a.diff.menu.Window, e.XRoot, e.YRoot, nil)
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
