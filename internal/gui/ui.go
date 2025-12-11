package gui

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	. "modernc.org/tk9.0"
	evalext "modernc.org/tk9.0/extensions/eval"
)

func (a *Controller) buildUI() {
	GridColumnConfigure(App, 0, Weight(1))
	GridRowConfigure(App, 1, Weight(1))

	controls := App.TFrame(Padding("8p"))
	Grid(controls, Row(0), Column(0), Sticky(WE))
	GridColumnConfigure(controls.Window, 1, Weight(1))

	repoLabel := fmt.Sprintf("Repository: %s", a.repoPath)
	Grid(controls.TLabel(Txt(repoLabel), Anchor(W)), Row(0), Column(0), Columnspan(4), Sticky(W))

	Grid(controls.TLabel(Txt("Filter:"), Anchor(E)), Row(1), Column(0), Sticky(E))
	a.filterEntry = controls.TEntry(Width(40), Textvariable(""))
	Grid(a.filterEntry, Row(1), Column(1), Sticky(WE), Padx("4p"))

	Bind(a.filterEntry, "<KeyRelease>", Command(func() {
		a.applyFilter(a.filterEntry.Textvariable())
	}))

	clearBtn := controls.TButton(Txt("Clear"), Command(func() {
		a.filterEntry.Configure(Textvariable(""))
		a.applyFilter("")
	}))
	Grid(clearBtn, Row(1), Column(2), Sticky(E), Padx("4p"))
	Grid(controls.TButton(Txt("Reload"), Command(a.reloadCommitsAsync)), Row(1), Column(3), Sticky(E))

	pane := App.TPanedwindow(Orient(VERTICAL))
	Grid(pane, Row(1), Column(0), Sticky(NEWS), Padx("4p"), Pady("4p"))

	listArea := pane.TFrame()
	diffArea := pane.TFrame()
	pane.Add(listArea.Window)
	pane.Add(diffArea.Window)

	GridRowConfigure(listArea.Window, 0, Weight(1))
	GridRowConfigure(listArea.Window, 1, Weight(0))
	GridColumnConfigure(listArea.Window, 0, Weight(1))
	GridRowConfigure(diffArea.Window, 0, Weight(1))
	GridColumnConfigure(diffArea.Window, 0, Weight(1))

	treeScroll := listArea.TScrollbar()
	a.tree = listArea.TTreeview(
		Show("headings"),
		Columns("graph commit author date"),
		Selectmode("browse"),
		Height(18),
		Yscrollcommand(func(e *Event) {
			e.ScrollSet(treeScroll)
			a.maybeLoadMoreOnScroll()
		}),
	)
	a.tree.Column("graph", Anchor(W), Width(120))
	a.tree.Column("commit", Anchor(W), Width(380))
	a.tree.Column("author", Anchor(W), Width(280))
	a.tree.Column("date", Anchor(W), Width(180))
	a.tree.Heading("graph", Txt("Graph"))
	a.tree.Heading("commit", Txt("Commit"))
	a.tree.Heading("author", Txt("Author"))
	a.tree.Heading("date", Txt("Date"))
	unstagedColor := a.palette.LocalUnstagedRow
	if unstagedColor == "" {
		unstagedColor = "#fde2e1"
	}
	stagedColor := a.palette.LocalStagedRow
	if stagedColor == "" {
		stagedColor = "#e2f7e1"
	}
	a.tree.TagConfigure("localUnstaged", Background(unstagedColor))
	a.tree.TagConfigure("localStaged", Background(stagedColor))
	Grid(a.tree, Row(0), Column(0), Sticky(NEWS))
	Grid(treeScroll, Row(0), Column(1), Sticky(NS))
	treeScroll.Configure(Command(func(e *Event) { e.Yview(a.tree) }))

	Bind(a.tree, "<<TreeviewSelect>>", Command(a.onTreeSelectionChanged))
	a.initTreeContextMenu()
	a.bindTreeContextMenu()

	diffPane := diffArea.TPanedwindow(Orient(HORIZONTAL))
	Grid(diffPane, Row(0), Column(0), Sticky(NEWS))

	textFrame := diffPane.TFrame()
	fileFrame := diffPane.TFrame()
	diffPane.Add(textFrame.Window)
	diffPane.Add(fileFrame.Window)
	configurePane := func(window *Window, options string) {
		if _, err := evalext.Eval(fmt.Sprintf("%s pane %s %s", diffPane, window, options)); err != nil {
			log.Printf("pane %s %s: %v", window, options, err)
		}
	}
	configurePane(textFrame.Window, "-weight 5")
	configurePane(fileFrame.Window, "-weight 1")

	GridRowConfigure(fileFrame.Window, 0, Weight(1))
	GridColumnConfigure(fileFrame.Window, 0, Weight(1))
	GridRowConfigure(textFrame.Window, 0, Weight(1))
	GridColumnConfigure(textFrame.Window, 0, Weight(1))

	detailYScroll := textFrame.TScrollbar(Command(func(e *Event) { e.Yview(a.detail) }))
	detailXScroll := textFrame.TScrollbar(Orient(HORIZONTAL), Command(func(e *Event) { e.Xview(a.detail) }))
	a.detail = textFrame.Text(Wrap(NONE), Font(CourierFont(), 11), Exportselection(false), Tabs("1c"))
	a.detail.Configure(Yscrollcommand(func(e *Event) { e.ScrollSet(detailYScroll) }))
	a.detail.Configure(Xscrollcommand(func(e *Event) { e.ScrollSet(detailXScroll) }))
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
	a.detail.TagConfigure("diffAdd", Background(addColor))
	a.detail.TagConfigure("diffDel", Background(delColor))
	a.detail.TagConfigure("diffHeader", Background(headerColor))
	Grid(a.detail, Row(0), Column(0), Sticky(NEWS))
	Grid(detailYScroll, Row(0), Column(1), Sticky(NS))
	Grid(detailXScroll, Row(1), Column(0), Sticky(WE))
	a.detail.Configure(State("disabled"))

	fileScroll := fileFrame.TScrollbar()
	a.fileList = fileFrame.Listbox(Exportselection(false), Width(40))
	a.fileList.Configure(Yscrollcommand(func(e *Event) { e.ScrollSet(fileScroll) }))
	Grid(a.fileList, Row(0), Column(0), Sticky(NEWS))
	Grid(fileScroll, Row(0), Column(1), Sticky(NS))
	fileScroll.Configure(Command(func(e *Event) { e.Yview(a.fileList) }))
	Bind(a.fileList, "<<ListboxSelect>>", Command(a.onFileSelectionChanged))

	a.status = App.TLabel(Anchor(W), Relief(SUNKEN), Padding("4p"))
	Grid(a.status, Row(2), Column(0), Sticky(WE))

	a.clearDetailText("Select a commit to view its details.")
	a.bindShortcuts()
}

func (a *Controller) showInitialLoadingRow() {
	if a.tree == nil {
		return
	}
	if len(a.commits) != 0 || len(a.visible) != 0 {
		return
	}
	if a.treeItemExists(loadingIndicatorID) {
		return
	}
	vals := tclList("", "Loading commits...", "", "")
	a.tree.Insert("", "end", Id(loadingIndicatorID), Values(vals))
}

func (a *Controller) initTreeContextMenu() {
	menu := App.Menu(Tearoff(false))
	item := menu.AddCommand(Command(a.copySelectedCommitReference))
	a.configureMenuLabel(menu, item, "Copy commit reference")
	a.treeMenu = menu
}

func (a *Controller) bindTreeContextMenu() {
	if a.tree == nil {
		return
	}
	handler := func(e *Event) {
		a.showTreeContextMenu(e)
	}
	Bind(a.tree, "<Button-2>", Command(handler))
	Bind(a.tree, "<Button-3>", Command(handler))
}

func (a *Controller) showTreeContextMenu(e *Event) {
	if a.treeMenu == nil || a.tree == nil || e == nil {
		return
	}
	item := strings.TrimSpace(a.tree.IdentifyItem(e.X, e.Y))
	if _, ok := a.treeCommitIndex(item); !ok {
		return
	}
	a.tree.Selection("set", item)
	a.tree.Focus(item)
	a.contextTargetID = item
	Popup(a.treeMenu.Window, e.XRoot, e.YRoot, nil)
}

func (a *Controller) copySelectedCommitReference() {
	id := a.contextTargetID
	if id == "" && a.tree != nil {
		if sel := a.tree.Selection(""); len(sel) > 0 {
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

func (a *Controller) configureMenuLabel(menu *MenuWidget, item *MenuItem, text string) {
	if menu == nil || item == nil || text == "" {
		return
	}
	safe := escapeTclString(text)
	if _, err := evalext.Eval(fmt.Sprintf("%s entryconfigure %s -label {%s}", menu, item, safe)); err != nil {
		log.Printf("menu label (%s): %v", text, err)
	}
}
