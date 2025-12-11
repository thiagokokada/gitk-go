package gui

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/thiagokokada/gitk-go/internal/git"

	. "modernc.org/tk9.0"
	evalext "modernc.org/tk9.0/extensions/eval"
	_ "modernc.org/tk9.0/themes/azure"
)

const (
	autoLoadThreshold  = 0.98
	moreIndicatorID    = "__more__"
	localUnstagedRowID = "__local_unstaged__"
	localStagedRowID   = "__local_staged__"
	diffDebounceDelay  = 60 * time.Millisecond
)

const (
	localUnstagedLabel = "Local uncommitted changes, not checked in to index"
	localStagedLabel   = "Local changes checked into index but not committed"
)

type Controller struct {
	svc       *git.Service
	repoPath  string
	batch     int
	themePref ThemePreference
	palette   colorPalette

	headRef string

	commits []*git.Entry
	visible []*git.Entry

	tree            *TTreeviewWidget
	treeMenu        *MenuWidget
	fileList        *ListboxWidget
	detail          *TextWidget
	status          *TLabelWidget
	filterEntry     *TEntryWidget
	fileSections    []git.FileSection
	branchLabels    map[string][]string
	contextTargetID string

	filterValue  string
	hasMore      bool
	loadingBatch bool

	selectedMu   sync.RWMutex
	selectedHash string

	shortcutsWin      *ToplevelWidget
	showLocalUnstaged bool
	showLocalStaged   bool

	localDiffMu sync.Mutex
	localDiffs  map[bool]*localDiffState

	diffLoadMu      sync.Mutex
	diffLoadTimer   *time.Timer
	pendingDiff     *git.Entry
	pendingDiffHash string
}

type localDiffState struct {
	sync.Mutex
	ready      bool
	loading    bool
	diff       string
	sections   []git.FileSection
	err        error
	generation int
}

type localDiffSnapshot struct {
	ready    bool
	loading  bool
	diff     string
	sections []git.FileSection
	err      error
}

func Run(repoPath string, batch int, pref ThemePreference) error {
	if err := InitializeExtension("eval"); err != nil && err != AlreadyInitialized {
		return fmt.Errorf("init eval extension: %v", err)
	}
	svc, err := git.Open(repoPath)
	if err != nil {
		return err
	}
	if batch <= 0 {
		batch = git.DefaultBatch
	}
	if pref < ThemeAuto || pref > ThemeDark {
		pref = ThemeAuto
	}
	app := &Controller{
		svc:        svc,
		repoPath:   svc.RepoPath(),
		batch:      batch,
		themePref:  pref,
		localDiffs: make(map[bool]*localDiffState),
	}
	return app.run()
}

func (a *Controller) run() error {
	a.palette = paletteForPreference(a.themePref)
	if a.palette.ThemeName != "" {
		ActivateTheme(a.palette.ThemeName)
	}
	applyAppIcon()
	if err := a.loadInitialCommits(); err != nil {
		return err
	}
	if err := a.loadBranchLabels(); err != nil {
		return err
	}
	a.buildUI()
	a.applyFilter(a.filterValue)
	a.refreshLocalChangesAsync(true)
	a.setStatus(a.statusSummary())
	App.WmTitle("gitk-go")
	App.SetResizable(true, true)
	App.Center().Wait()
	return nil
}

func (a *Controller) loadInitialCommits() error {
	entries, head, hasMore, err := a.svc.ScanCommits(0, a.batch)
	if err != nil {
		return err
	}
	a.commits = entries
	a.visible = entries
	a.headRef = head
	a.hasMore = hasMore
	if a.batch <= 0 {
		a.batch = git.DefaultBatch
	}
	return nil
}

func (a *Controller) loadBranchLabels() error {
	labels, err := a.svc.BranchLabels()
	if err != nil {
		return err
	}
	a.branchLabels = labels
	return nil
}

func (a *Controller) refreshLocalChangesAsync(prefetch bool) {
	go func() {
		var (
			status    git.LocalChanges
			repoReady bool
			err       error
		)
		if a.svc != nil {
			repoReady = true
			status, err = a.svc.LocalChanges()
		}
		if err != nil {
			log.Printf("local changes: %v", err)
			return
		}
		PostEvent(func() {
			a.applyLocalChangeStatus(status, repoReady, prefetch)
		}, false)
	}()
}

func (a *Controller) applyLocalChangeStatus(status git.LocalChanges, repoReady bool, prefetch bool) {
	if !repoReady {
		a.setLocalRowVisibility(false, false)
		a.setLocalRowVisibility(true, false)
		a.resetLocalDiffState(false)
		a.resetLocalDiffState(true)
		return
	}
	prevUnstaged := a.showLocalUnstaged
	prevStaged := a.showLocalStaged
	a.setLocalRowVisibility(false, status.HasWorktree)
	a.setLocalRowVisibility(true, status.HasStaged)
	if !status.HasWorktree {
		a.resetLocalDiffState(false)
	}
	if !status.HasStaged {
		a.resetLocalDiffState(true)
	}
	shouldLoadUnstaged := prefetch && status.HasWorktree
	shouldLoadStaged := prefetch && status.HasStaged
	if !prefetch {
		shouldLoadUnstaged = status.HasWorktree && !prevUnstaged
		shouldLoadStaged = status.HasStaged && !prevStaged
	}
	if shouldLoadUnstaged {
		a.ensureLocalDiffLoading(false, true)
	}
	if shouldLoadStaged {
		a.ensureLocalDiffLoading(true, true)
	}
}

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

func (a *Controller) bindShortcuts() {
	if a.tree == nil {
		return
	}
	bindNav := func(sequence string, handler func()) {
		Bind(App, sequence, Command(func() {
			if a.filterHasFocus() {
				return
			}
			handler()
		}))
	}
	bindAny := func(sequence string, handler func()) {
		Bind(App, sequence, Command(handler))
	}
	for _, sc := range a.shortcutBindings() {
		if sc.handler == nil {
			continue
		}
		for _, seq := range sc.sequences {
			if seq == "" {
				continue
			}
			if sc.navigation {
				bindNav(seq, sc.handler)
			} else {
				bindAny(seq, sc.handler)
			}
		}
	}
}

func (a *Controller) insertLocalRows() {
	if a.tree == nil {
		return
	}
	index := 0
	if a.showLocalUnstaged {
		vals := tclList("", localUnstagedLabel, "", "")
		a.tree.Insert("", index, Id(localUnstagedRowID), Values(vals), Tags("localUnstaged"))
		index++
	}
	if a.showLocalStaged {
		vals := tclList("", localStagedLabel, "", "")
		a.tree.Insert("", index, Id(localStagedRowID), Values(vals), Tags("localStaged"))
	}
}

type shortcutBinding struct {
	sequences   []string
	display     string
	description string
	category    string
	navigation  bool
	handler     func()
}

func (a *Controller) shortcutBindings() []shortcutBinding {
	return []shortcutBinding{
		{
			category:    "Commit list",
			display:     "p / k",
			description: "Move up one commit",
			sequences:   []string{"<KeyPress-p>", "<KeyPress-k>"},
			navigation:  true,
			handler:     func() { a.moveSelection(-1) },
		},
		{
			category:    "Commit list",
			display:     "n / j",
			description: "Move down one commit",
			sequences:   []string{"<KeyPress-n>", "<KeyPress-j>"},
			navigation:  true,
			handler:     func() { a.moveSelection(1) },
		},
		{
			category:    "Commit list",
			display:     "Home",
			description: "Jump to the first commit",
			sequences:   []string{"<KeyPress-Home>"},
			navigation:  true,
			handler:     a.selectFirst,
		},
		{
			category:    "Commit list",
			display:     "End",
			description: "Jump to the last loaded commit",
			sequences:   []string{"<KeyPress-End>"},
			navigation:  true,
			handler:     a.selectLast,
		},
		{
			category:    "Commit list",
			display:     "Ctrl/Cmd + Page Up",
			description: "Scroll commit list up a page",
			sequences:   []string{"<Control-Prior>", "<Command-Prior>"},
			navigation:  true,
			handler:     func() { a.scrollTreePages(-1) },
		},
		{
			category:    "Commit list",
			display:     "Ctrl/Cmd + Page Down",
			description: "Scroll commit list down a page",
			sequences:   []string{"<Control-Next>", "<Command-Next>"},
			navigation:  true,
			handler:     func() { a.scrollTreePages(1) },
		},
		{
			category:    "Diff view",
			display:     "Delete / Backspace / b",
			description: "Scroll diff up one page",
			sequences:   []string{"<KeyPress-Delete>", "<KeyPress-BackSpace>", "<KeyPress-b>", "<KeyPress-B>"},
			navigation:  true,
			handler:     func() { a.scrollDetailPages(-1) },
		},
		{
			category:    "Diff view",
			display:     "Space",
			description: "Scroll diff down one page",
			sequences:   []string{"<KeyPress-space>"},
			navigation:  true,
			handler:     func() { a.scrollDetailPages(1) },
		},
		{
			category:    "Diff view",
			display:     "U",
			description: "Scroll diff up 18 lines",
			sequences:   []string{"<KeyPress-u>", "<KeyPress-U>"},
			navigation:  true,
			handler:     func() { a.scrollDetailLines(-18) },
		},
		{
			category:    "Diff view",
			display:     "D",
			description: "Scroll diff down 18 lines",
			sequences:   []string{"<KeyPress-d>", "<KeyPress-D>"},
			navigation:  true,
			handler:     func() { a.scrollDetailLines(18) },
		},
		{
			category:    "General",
			display:     "/",
			description: "Focus the filter box",
			sequences:   []string{"<KeyPress-/>"},
			navigation:  false,
			handler: func() {
				if a.filterEntry == nil || a.filterHasFocus() {
					return
				}
				a.focusFilterEntry()
			},
		},
		{
			category:    "General",
			display:     "F5",
			description: "Reload commits",
			sequences:   []string{"<F5>"},
			navigation:  false,
			handler:     func() { a.reloadCommitsAsync() },
		},
		{
			category:    "General",
			display:     "F1",
			description: "Show shortcut list",
			sequences:   []string{"<F1>"},
			navigation:  false,
			handler:     func() { a.showShortcutsDialog() },
		},
		{
			category:    "General",
			display:     "Ctrl+Q",
			description: "Quit gitk-go",
			sequences:   []string{"<Control-KeyPress-q>"},
			navigation:  false,
			handler:     func() { Destroy(App) },
		},
	}
}

func (a *Controller) filterHasFocus() bool {
	if a.filterEntry == nil {
		return false
	}
	return Focus() == a.filterEntry.String()
}

func (a *Controller) showShortcutsDialog() {
	if a.shortcutsWin != nil {
		Destroy(a.shortcutsWin.Window)
		a.shortcutsWin = nil
	}
	dialog := App.Toplevel()
	a.shortcutsWin = dialog
	dialog.Window.WmTitle("Keyboard Shortcuts")
	WmTransient(dialog.Window, App)
	WmAttributes(dialog.Window, "-topmost", 1)

	frame := dialog.TFrame(Padding("12p"))
	Grid(frame, Row(0), Column(0), Sticky(NEWS))
	GridColumnConfigure(frame.Window, 0, Weight(1))
	GridRowConfigure(frame.Window, 1, Weight(1))

	header := frame.TLabel(Txt("Keyboard Shortcuts"), Anchor(W))
	Grid(header, Row(0), Column(0), Sticky(W), Pady("0 8p"))

	text := frame.Text(Width(62), Height(18), Wrap(WORD), Exportselection(false))
	text.Insert("1.0", a.shortcutsHelpText())
	text.Configure(State("disabled"))
	Grid(text, Row(1), Column(0), Sticky(NEWS))

	closeBtn := frame.TButton(Txt("Close"), Command(func() { Destroy(dialog.Window) }))
	Grid(closeBtn, Row(2), Column(0), Sticky(E), Pady("8p 0"))

	Bind(dialog.Window, "<Destroy>", Command(func() {
		if a.shortcutsWin == dialog {
			a.shortcutsWin = nil
		}
	}))
	dialog.Window.Center()
}

func (a *Controller) moveSelection(delta int) {
	if a.tree == nil {
		return
	}
	sel := a.tree.Selection("")
	if len(sel) > 0 {
		if a.handleSpecialRowNav(sel[0], delta) {
			return
		}
	}
	if len(a.visible) == 0 {
		return
	}
	idx := a.currentSelectionIndex() + delta
	if idx < 0 && delta < 0 {
		if a.showLocalStaged {
			a.selectSpecialRow(localStagedRowID)
			return
		}
		if a.showLocalUnstaged {
			a.selectSpecialRow(localUnstagedRowID)
			return
		}
		idx = 0
	}
	if idx < 0 {
		idx = 0
	}
	if idx >= len(a.visible) {
		idx = len(a.visible) - 1
	}
	a.selectTreeIndex(idx)
}

func (a *Controller) selectFirst() {
	if len(a.visible) == 0 {
		return
	}
	a.selectTreeIndex(0)
}

func (a *Controller) selectLast() {
	if len(a.visible) == 0 {
		return
	}
	a.selectTreeIndex(len(a.visible) - 1)
}

func (a *Controller) selectSpecialRow(id string) {
	if a.tree == nil {
		return
	}
	a.tree.Selection("set", id)
	a.tree.Focus(id)
	a.tree.See(id)
	switch id {
	case localUnstagedRowID:
		a.showLocalChanges(false)
	case localStagedRowID:
		a.showLocalChanges(true)
	}
}

func (a *Controller) currentSelectionIndex() int {
	if a.tree == nil {
		return 0
	}
	sel := a.tree.Selection("")
	if len(sel) == 0 || sel[0] == moreIndicatorID {
		return 0
	}
	if idx, err := strconv.Atoi(sel[0]); err == nil {
		return idx
	}
	return 0
}

func (a *Controller) selectTreeIndex(idx int) {
	if a.tree == nil || idx < 0 || idx >= len(a.visible) {
		return
	}
	id := strconv.Itoa(idx)
	a.tree.Selection("set", id)
	a.tree.Focus(id)
	a.tree.See(id)
	a.showCommitDetails(idx)
}

func (a *Controller) handleSpecialRowNav(id string, delta int) bool {
	if delta == 0 {
		return true
	}
	switch id {
	case localUnstagedRowID:
		if delta > 0 {
			if a.showLocalStaged {
				a.selectSpecialRow(localStagedRowID)
			} else if len(a.visible) > 0 {
				a.selectTreeIndex(0)
			}
		}
		return true
	case localStagedRowID:
		if delta < 0 {
			if a.showLocalUnstaged {
				a.selectSpecialRow(localUnstagedRowID)
			}
			return true
		}
		if delta > 0 {
			if len(a.visible) > 0 {
				a.selectTreeIndex(0)
			}
			return true
		}
		return true
	default:
		return false
	}
}

func (a *Controller) scrollTreePages(delta int) {
	if a.tree == nil || delta == 0 {
		return
	}
	if _, err := evalext.Eval(fmt.Sprintf("%s yview scroll %d pages", a.tree, delta)); err != nil {
		log.Printf("tree scroll: %v", err)
	}
}

func (a *Controller) scrollDetailPages(delta int) {
	a.scrollDetail(delta, "pages")
}

func (a *Controller) scrollDetailLines(delta int) {
	a.scrollDetail(delta, "units")
}

func (a *Controller) scrollDetail(delta int, unit string) {
	if a.detail == nil || delta == 0 {
		return
	}
	if _, err := evalext.Eval(fmt.Sprintf("%s yview scroll %d %s", a.detail, delta, unit)); err != nil {
		log.Printf("detail scroll: %v", err)
	}
}

func (a *Controller) focusFilterEntry() {
	if a.filterEntry == nil {
		return
	}
	if _, err := evalext.Eval(fmt.Sprintf("focus %s", a.filterEntry)); err != nil {
		log.Printf("focus filter: %v", err)
	}
	if _, err := evalext.Eval(fmt.Sprintf("%s selection range 0 end", a.filterEntry)); err != nil {
		log.Printf("select filter: %v", err)
	}
	if _, err := evalext.Eval(fmt.Sprintf("%s icursor end", a.filterEntry)); err != nil {
		log.Printf("cursor filter: %v", err)
	}
}

func (a *Controller) onTreeSelectionChanged() {
	if a.tree == nil {
		return
	}
	sel := a.tree.Selection("")
	if len(sel) == 0 {
		return
	}
	switch sel[0] {
	case moreIndicatorID:
		return
	case localUnstagedRowID:
		a.showLocalChanges(false)
		return
	case localStagedRowID:
		a.showLocalChanges(true)
		return
	}
	idx, err := strconv.Atoi(sel[0])
	if err != nil || idx < 0 || idx >= len(a.visible) {
		return
	}
	a.showCommitDetails(idx)
}

func (a *Controller) showCommitDetails(index int) {
	if index < 0 || index >= len(a.visible) {
		a.clearDetailText("Commit index out of range.")
		return
	}
	entry := a.visible[index]
	header := git.FormatCommitHeader(entry.Commit)
	hash := entry.Commit.Hash.String()
	a.setSelectedHash(hash)
	a.setFileSections(nil)
	a.writeDetailText(header+"\nLoading diff...", false)
	a.scheduleDiffLoad(entry, hash)
}

func (a *Controller) showLocalChanges(staged bool) {
	a.cancelPendingDiffLoad()
	a.renderLocalChanges(staged, true)
}

func (a *Controller) renderLocalChanges(staged bool, requestReload bool) {
	header := localUnstagedLabel
	if staged {
		header = localStagedLabel
	}
	snap := a.snapshotLocalDiff(staged)
	if requestReload && snap.ready {
		a.presentLocalDiff(header, snap)
		a.ensureLocalDiffLoading(staged, true)
		return
	}
	if requestReload {
		a.ensureLocalDiffLoading(staged, true)
		snap = a.snapshotLocalDiff(staged)
	} else if !snap.ready && !snap.loading {
		a.ensureLocalDiffLoading(staged, false)
		snap = a.snapshotLocalDiff(staged)
	}
	a.presentLocalDiff(header, snap)
}

func (a *Controller) presentLocalDiff(header string, snap localDiffSnapshot) {
	a.setSelectedHash("")
	if !snap.ready {
		a.clearDetailText(fmt.Sprintf("%s\nLoading local changes...", header))
		return
	}
	if snap.err != nil {
		a.clearDetailText(fmt.Sprintf("%s\nUnable to compute diff: %v", header, snap.err))
		return
	}
	diff := snap.diff
	if strings.TrimSpace(diff) == "" {
		a.clearDetailText(fmt.Sprintf("%s\nNo changes.", header))
		return
	}
	a.writeDetailText(diff, len(snap.sections) > 0)
	a.setFileSections(snap.sections)
}

func (a *Controller) snapshotLocalDiff(staged bool) localDiffSnapshot {
	state := a.localDiffState(staged, false)
	if state == nil {
		return localDiffSnapshot{}
	}
	state.Lock()
	defer state.Unlock()
	snap := localDiffSnapshot{
		ready:   state.ready,
		loading: state.loading,
		diff:    state.diff,
		err:     state.err,
	}
	if len(state.sections) > 0 {
		snap.sections = append([]git.FileSection(nil), state.sections...)
	}
	return snap
}

func (a *Controller) ensureLocalDiffLoading(staged bool, force bool) {
	state := a.localDiffState(staged, true)
	state.Lock()
	if state.loading {
		state.Unlock()
		return
	}
	if state.ready && !force {
		state.Unlock()
		return
	}
	state.loading = true
	state.ready = false
	state.diff = ""
	state.sections = nil
	state.err = nil
	state.generation++
	gen := state.generation
	state.Unlock()
	go a.computeLocalDiff(staged, gen)
}

func (a *Controller) computeLocalDiff(staged bool, gen int) {
	if a.svc == nil {
		return
	}
	diff, sections, err := a.svc.WorktreeDiff(staged)
	state := a.localDiffState(staged, true)
	state.Lock()
	if gen != state.generation {
		state.Unlock()
		return
	}
	state.loading = false
	state.ready = true
	state.diff = diff
	if len(sections) > 0 {
		state.sections = append([]git.FileSection(nil), sections...)
	} else {
		state.sections = nil
	}
	state.err = err
	state.Unlock()
	PostEvent(func() {
		a.onLocalDiffLoaded(staged)
	}, false)
}

func (a *Controller) resetLocalDiffState(staged bool) {
	state := a.localDiffState(staged, false)
	if state == nil {
		return
	}
	state.Lock()
	state.loading = false
	state.ready = false
	state.diff = ""
	state.sections = nil
	state.err = nil
	state.generation++
	state.Unlock()
}

func (a *Controller) localDiffState(staged bool, create bool) *localDiffState {
	a.localDiffMu.Lock()
	defer a.localDiffMu.Unlock()
	if a.localDiffs == nil {
		if !create {
			return nil
		}
		a.localDiffs = make(map[bool]*localDiffState)
	}
	if st, ok := a.localDiffs[staged]; ok {
		return st
	}
	if !create {
		return nil
	}
	st := &localDiffState{}
	a.localDiffs[staged] = st
	return st
}

func (a *Controller) onLocalDiffLoaded(staged bool) {
	snap := a.snapshotLocalDiff(staged)
	if snap.err == nil {
		if strings.TrimSpace(snap.diff) == "" {
			a.setLocalRowVisibility(staged, false)
		} else {
			a.setLocalRowVisibility(staged, true)
		}
	}
	targetID := localRowID(staged)
	if a.tree == nil {
		return
	}
	sel := a.tree.Selection("")
	if len(sel) == 0 || sel[0] != targetID {
		return
	}
	a.renderLocalChanges(staged, false)
}

func (a *Controller) setLocalRowVisibility(staged bool, show bool) {
	var current bool
	if staged {
		current = a.showLocalStaged
	} else {
		current = a.showLocalUnstaged
	}
	if current == show {
		return
	}
	if staged {
		a.showLocalStaged = show
	} else {
		a.showLocalUnstaged = show
	}
	if a.tree == nil {
		return
	}
	id := localRowID(staged)
	if show {
		if !a.treeItemExists(id) {
			a.insertSingleLocalRow(staged)
		}
		return
	}
	if a.treeItemExists(id) {
		a.tree.Delete(id)
	}
}

func (a *Controller) insertSingleLocalRow(staged bool) {
	if a.tree == nil {
		return
	}
	label := localRowLabel(staged)
	tag := localRowTag(staged)
	index := 0
	if staged && a.showLocalUnstaged {
		index = 1
	}
	vals := tclList("", label, "", "")
	a.tree.Insert("", index, Id(localRowID(staged)), Values(vals), Tags(tag))
}

func localRowID(staged bool) string {
	if staged {
		return localStagedRowID
	}
	return localUnstagedRowID
}

func localRowLabel(staged bool) string {
	if staged {
		return localStagedLabel
	}
	return localUnstagedLabel
}

func localRowTag(staged bool) string {
	if staged {
		return "localStaged"
	}
	return "localUnstaged"
}

func (a *Controller) treeItemExists(id string) bool {
	if a.tree == nil || id == "" {
		return false
	}
	out, err := evalext.Eval(fmt.Sprintf("%s exists %s", a.tree, id))
	if err != nil {
		log.Printf("tree exists %s: %v", id, err)
		return false
	}
	return strings.TrimSpace(out) == "1"
}

func (a *Controller) populateDiff(entry *git.Entry, hash string) {
	diff, sections, err := a.svc.Diff(entry.Commit)
	if err != nil {
		diff = fmt.Sprintf("Unable to compute diff: %v", err)
	}
	highlight := len(sections) > 0
	PostEvent(func() {
		if a.currentSelection() != hash {
			return
		}
		a.writeDetailText(diff, highlight)
		a.setFileSections(sections)
	}, false)
}

func (a *Controller) scheduleDiffLoad(entry *git.Entry, hash string) {
	if entry == nil {
		return
	}
	a.diffLoadMu.Lock()
	defer a.diffLoadMu.Unlock()
	a.pendingDiff = entry
	a.pendingDiffHash = hash
	if a.diffLoadTimer != nil {
		a.diffLoadTimer.Stop()
	}
	a.diffLoadTimer = time.AfterFunc(diffDebounceDelay, func() {
		a.diffLoadMu.Lock()
		pending := a.pendingDiff
		pendingHash := a.pendingDiffHash
		a.pendingDiff = nil
		a.pendingDiffHash = ""
		a.diffLoadTimer = nil
		a.diffLoadMu.Unlock()
		if pending == nil {
			return
		}
		go a.populateDiff(pending, pendingHash)
	})
}

func (a *Controller) cancelPendingDiffLoad() {
	a.diffLoadMu.Lock()
	if a.diffLoadTimer != nil {
		a.diffLoadTimer.Stop()
		a.diffLoadTimer = nil
	}
	a.pendingDiff = nil
	a.pendingDiffHash = ""
	a.diffLoadMu.Unlock()
}

func (a *Controller) reloadCommitsAsync() {
	if a.loadingBatch {
		return
	}
	a.loadingBatch = true
	currentFilter := a.filterValue
	go func(filter string) {
		entries, head, hasMore, err := a.svc.ScanCommits(0, a.batch)
		PostEvent(func() {
			a.loadingBatch = false
			if err != nil {
				msg := fmt.Sprintf("Failed to reload commits: %v", err)
				log.Print(msg)
				a.setStatus(msg)
				return
			}
			a.commits = entries
			a.visible = entries
			a.headRef = head
			a.hasMore = hasMore
			if err := a.loadBranchLabels(); err != nil {
				log.Printf("failed to refresh branch labels: %v", err)
			}
			a.applyFilter(filter)
			a.refreshLocalChangesAsync(true)
			a.setStatus(a.statusSummary())
		}, false)
	}(currentFilter)
}

func (a *Controller) loadMoreCommitsAsync(prefetch bool) {
	if a.loadingBatch || (!prefetch && !a.hasMore) {
		return
	}
	a.loadingBatch = true
	currentFilter := a.filterValue
	skip := len(a.commits)
	go func(filter string, skipCount int, background bool) {
		entries, _, hasMore, err := a.svc.ScanCommits(skipCount, a.batch)
		PostEvent(func() {
			a.loadingBatch = false
			if err != nil {
				msg := fmt.Sprintf("Failed to load more commits: %v", err)
				log.Print(msg)
				if !background {
					a.setStatus(msg)
				}
				return
			}
			if len(entries) == 0 {
				a.hasMore = false
				if !background {
					a.setStatus("No more commits available.")
				}
				return
			}
			a.commits = append(a.commits, entries...)
			a.hasMore = hasMore
			if err := a.loadBranchLabels(); err != nil {
				log.Printf("failed to refresh branch labels: %v", err)
			}
			a.applyFilter(filter)
			a.refreshLocalChangesAsync(false)
			a.setStatus(a.statusSummary())
			if background && a.hasMore {
				go a.loadMoreCommitsAsync(true)
			}
		}, false)
	}(currentFilter, skip, prefetch)
}

func (a *Controller) applyFilter(raw string) {
	a.filterValue = raw
	a.visible = filterEntries(a.commits, raw)
	if a.tree == nil {
		return
	}

	children := a.tree.Children("")
	if len(children) != 0 {
		args := make([]any, len(children))
		for i, child := range children {
			args[i] = child
		}
		a.tree.Delete(args...)
	}
	a.insertLocalRows()
	rows := buildTreeRows(a.visible, a.branchLabels)
	for _, row := range rows {
		vals := tclList(row.Graph, row.Commit, row.Author, row.Date)
		a.tree.Insert("", "end", Id(row.ID), Values(vals))
	}
	if a.hasMore && len(a.visible) > 0 {
		vals := tclList("", "Loading more commits...", "", "")
		a.tree.Insert("", "end", Id(moreIndicatorID), Values(vals))
	}

	if len(a.visible) == 0 {
		if len(a.commits) == 0 {
			a.clearDetailText("Repository has no commits yet.")
		} else {
			a.clearDetailText("No commits match the current filter.")
		}
		a.setStatus(a.statusSummary())
		return
	}

	firstID := strconv.Itoa(0)
	a.tree.Selection("set", firstID)
	a.tree.Focus(firstID)
	a.showCommitDetails(0)
	a.setStatus(a.statusSummary())
	a.scheduleAutoLoadCheck()
}

func (a *Controller) scheduleAutoLoadCheck() {
	if a.tree == nil || a.filterValue == "" || !a.hasMore {
		return
	}
	PostEvent(func() {
		a.maybeLoadMoreOnScroll()
	}, false)
}

func (a *Controller) maybeLoadMoreOnScroll() {
	if a.tree == nil || a.loadingBatch || !a.hasMore {
		return
	}
	if len(a.visible) == 0 {
		return
	}
	start, end, err := a.treeYviewRange()
	if err != nil {
		log.Printf("tree yview: %v", err)
		return
	}
	if a.filterValue == "" && len(a.visible) >= a.batch && start <= 0 && end >= 1 {
		return
	}
	if end >= autoLoadThreshold {
		a.loadMoreCommitsAsync(false)
	}
}

func (a *Controller) treeYviewRange() (float64, float64, error) {
	if a.tree == nil {
		return 0, 0, fmt.Errorf("tree widget not ready")
	}
	path := a.tree.String()
	if path == "" {
		return 0, 0, fmt.Errorf("tree widget has empty path")
	}
	out, err := evalext.Eval(fmt.Sprintf("%s yview", path))
	if err != nil {
		return 0, 0, err
	}
	fields := strings.Fields(strings.TrimSpace(out))
	if len(fields) < 2 {
		return 0, 0, fmt.Errorf("unexpected treeview yview output %q", out)
	}
	start, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, 0, err
	}
	end, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return 0, 0, err
	}
	return start, end, nil
}

func (a *Controller) clearDetailText(msg string) {
	if a.detail == nil {
		return
	}
	a.writeDetailText(msg, false)
	a.setFileSections(nil)
}

func (a *Controller) writeDetailText(content string, highlightDiff bool) {
	if a.detail == nil {
		return
	}
	a.detail.Configure(State(NORMAL))
	a.detail.Delete("1.0", END)
	a.detail.Insert("1.0", content)
	if highlightDiff {
		a.highlightDiffLines(content)
	} else {
		a.detail.TagRemove("diffAdd", "1.0", END)
		a.detail.TagRemove("diffDel", "1.0", END)
		a.detail.TagRemove("diffHeader", "1.0", END)
	}
	a.detail.Configure(State("disabled"))
}

func (a *Controller) highlightDiffLines(content string) {
	a.detail.TagRemove("diffAdd", "1.0", END)
	a.detail.TagRemove("diffDel", "1.0", END)
	a.detail.TagRemove("diffHeader", "1.0", END)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if len(line) == 0 {
			continue
		}
		tag := ""
		switch {
		case strings.HasPrefix(line, "diff --git"):
			tag = "diffHeader"
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			tag = "diffAdd"
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			tag = "diffDel"
		default:
			continue
		}
		start := fmt.Sprintf("%d.0", i+1)
		end := fmt.Sprintf("%d.end", i+1)
		a.detail.TagAdd(tag, start, end)
	}
}

func (a *Controller) setFileSections(sections []git.FileSection) {
	a.fileSections = sections
	if a.fileList == nil {
		return
	}
	a.fileList.Configure(State("normal"))
	a.fileList.Delete(0, END)
	if len(sections) == 0 {
		a.fileList.Insert(END, "(no files)")
		a.fileList.Configure(State("disabled"))
		return
	}
	for _, sec := range sections {
		a.fileList.Insert(END, sec.Path)
	}
	a.fileList.SelectionClear(0, END)
	a.fileList.Activate(0)
	a.fileList.Configure(State("normal"))
}

func (a *Controller) onFileSelectionChanged() {
	if len(a.fileSections) == 0 || a.fileList == nil {
		return
	}
	selection := a.fileList.Curselection()
	if len(selection) == 0 {
		return
	}
	idx := selection[0]
	if idx < 0 || idx >= len(a.fileSections) {
		return
	}
	a.scrollDiffToLine(a.fileSections[idx].Line)
}

func (a *Controller) scrollDiffToLine(line int) {
	if a.detail == nil || line <= 0 {
		return
	}
	defer func() {
		recover()
	}()
	totalLines := a.textLineCount()
	if totalLines <= 1 {
		a.detail.Yviewmoveto(0)
		return
	}
	fraction := float64(line-1) / float64(totalLines-1)
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	a.detail.Yviewmoveto(fraction)
}

func (a *Controller) textLineCount() int {
	if a.detail == nil {
		return 0
	}
	index := a.detail.Index(END)
	parts := strings.SplitN(index, ".", 2)
	if len(parts) == 0 {
		return 0
	}
	lines, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0
	}
	if lines > 0 {
		lines--
	}
	return lines
}

func (a *Controller) setSelectedHash(hash string) {
	a.selectedMu.Lock()
	defer a.selectedMu.Unlock()
	a.selectedHash = hash
}

func (a *Controller) currentSelection() string {
	a.selectedMu.RLock()
	defer a.selectedMu.RUnlock()
	return a.selectedHash
}

func (a *Controller) setStatus(msg string) {
	if a.status == nil {
		log.Print(msg)
		return
	}
	text := msg
	PostEvent(func() {
		a.status.Configure(Txt(text))
	}, false)
}

func (a *Controller) statusSummary() string {
	total := len(a.commits)
	visible := len(a.visible)
	head := a.headRef
	if head == "" {
		head = "HEAD"
	}
	filterDesc := strings.TrimSpace(a.filterValue)
	path := a.repoPath
	if path == "" && a.svc != nil {
		path = a.svc.RepoPath()
	}
	base := fmt.Sprintf("Showing %d/%d loaded commits on %s — %s", visible, total, head, path)
	if a.hasMore {
		base += " (more available)"
	}
	if filterDesc == "" {
		return base
	}
	return fmt.Sprintf("Filter %q — %s", filterDesc, base)
}

func commitListColumns(entry *git.Entry) (msg, author, when string) {
	firstLine := strings.SplitN(strings.TrimSpace(entry.Commit.Message), "\n", 2)[0]
	if len(firstLine) > 80 {
		firstLine = firstLine[:77] + "..."
	}
	msg = fmt.Sprintf("%s  %s", entry.Commit.Hash.String()[:7], firstLine)
	author = fmt.Sprintf("%s <%s>", entry.Commit.Author.Name, entry.Commit.Author.Email)
	when = entry.Commit.Author.When.Format("2006-01-02 15:04")
	return
}

func formatGraphValue(entry *git.Entry, labels []string) string {
	graph := strings.TrimRight(entry.Graph, " ")
	if graph == "" {
		graph = "*"
	}
	if len(labels) != 0 {
		label := fmt.Sprintf("[%s]", strings.Join(labels, ", "))
		if graph != "" {
			graph += " "
		}
		graph += label
	}
	return graph
}

func tclList(values ...string) string {
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = fmt.Sprintf("\"%s\"", escapeTclString(v))
	}
	return strings.Join(parts, " ")
}

func escapeTclString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
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

func filterEntries(entries []*git.Entry, query string) []*git.Entry {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return entries
	}
	var filtered []*git.Entry
	for _, entry := range entries {
		if strings.Contains(entry.SearchText, q) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func (a *Controller) shortcutsHelpText() string {
	var b strings.Builder
	currentCategory := ""
	for _, sc := range a.shortcutBindings() {
		if sc.category == "" || sc.display == "" || sc.description == "" {
			continue
		}
		if sc.category != currentCategory {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			currentCategory = sc.category
			b.WriteString(currentCategory)
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "  %s — %s\n", sc.display, sc.description)
	}
	return strings.TrimRight(b.String(), "\n")
}

type treeRow struct {
	ID     string
	Graph  string
	Commit string
	Author string
	Date   string
}

func buildTreeRows(entries []*git.Entry, labels map[string][]string) []treeRow {
	if len(entries) == 0 {
		return nil
	}
	rows := make([]treeRow, 0, len(entries))
	for i, entry := range entries {
		if entry == nil || entry.Commit == nil {
			continue
		}
		msg, author, when := commitListColumns(entry)
		graph := formatGraphValue(entry, labels[entry.Commit.Hash.String()])
		rows = append(rows, treeRow{
			ID:     strconv.Itoa(i),
			Graph:  graph,
			Commit: msg,
			Author: author,
			Date:   when,
		})
	}
	return rows
}
