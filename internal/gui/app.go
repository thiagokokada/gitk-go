package gui

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/thiagokokada/gitk-go/internal/git"

	. "modernc.org/tk9.0"
	evalext "modernc.org/tk9.0/extensions/eval"
	_ "modernc.org/tk9.0/themes/azure"
)

const (
	autoLoadThreshold = 0.98
	moreIndicatorID   = "__more__"
)

type Controller struct {
	svc      *git.Service
	repoPath string
	batch    int

	headRef string

	commits []*git.Entry
	visible []*git.Entry

	tree         *TTreeviewWidget
	fileList     *ListboxWidget
	detail       *TextWidget
	status       *TLabelWidget
	filterEntry  *TEntryWidget
	fileSections []git.FileSection
	branchLabels map[string][]string

	filterValue  string
	hasMore      bool
	loadingBatch bool

	selectedMu   sync.RWMutex
	selectedHash string
}

func Run(repoPath string, batch int) error {
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
	app := &Controller{svc: svc, repoPath: svc.RepoPath(), batch: batch}
	return app.run()
}

func (a *Controller) run() error {
	if err := a.loadInitialCommits(); err != nil {
		return err
	}
	if err := a.loadBranchLabels(); err != nil {
		return err
	}
	a.buildUI()
	a.applyFilter(a.filterValue)
	a.setStatus(a.statusSummary())
	ActivateTheme("azure light")
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
	Grid(a.tree, Row(0), Column(0), Sticky(NEWS))
	Grid(treeScroll, Row(0), Column(1), Sticky(NS))
	treeScroll.Configure(Command(func(e *Event) { e.Yview(a.tree) }))

	Bind(a.tree, "<<TreeviewSelect>>", Command(a.onTreeSelectionChanged))

	diffPane := diffArea.TPanedwindow(Orient(HORIZONTAL))
	Grid(diffPane, Row(0), Column(0), Sticky(NEWS))

	textFrame := diffPane.TFrame()
	fileFrame := diffPane.TFrame()
	diffPane.Add(textFrame.Window)
	diffPane.Add(fileFrame.Window)

	GridRowConfigure(fileFrame.Window, 0, Weight(1))
	GridColumnConfigure(fileFrame.Window, 0, Weight(1))
	GridRowConfigure(textFrame.Window, 0, Weight(1))
	GridColumnConfigure(textFrame.Window, 0, Weight(1))

	detailYScroll := textFrame.TScrollbar(Command(func(e *Event) { e.Yview(a.detail) }))
	detailXScroll := textFrame.TScrollbar(Orient(HORIZONTAL), Command(func(e *Event) { e.Xview(a.detail) }))
	a.detail = textFrame.Text(Wrap(NONE), Font(CourierFont(), 11), Exportselection(false), Tabs("1c"))
	a.detail.Configure(Yscrollcommand(func(e *Event) { e.ScrollSet(detailYScroll) }))
	a.detail.Configure(Xscrollcommand(func(e *Event) { e.ScrollSet(detailXScroll) }))
	a.detail.TagConfigure("diffAdd", Background("#dff5de"))
	a.detail.TagConfigure("diffDel", Background("#f9d6d5"))
	a.detail.TagConfigure("diffHeader", Background("#e4e4e4"))
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

	bindNav("<KeyPress-p>", func() { a.moveSelection(-1) })
	bindNav("<KeyPress-k>", func() { a.moveSelection(-1) })
	bindNav("<KeyPress-n>", func() { a.moveSelection(1) })
	bindNav("<KeyPress-j>", func() { a.moveSelection(1) })
	bindNav("<KeyPress-Home>", a.selectFirst)
	bindNav("<KeyPress-End>", a.selectLast)

	bindNav("<Control-Prior>", func() { a.scrollTreePages(-1) })
	bindNav("<Control-Next>", func() { a.scrollTreePages(1) })
	bindNav("<Command-Prior>", func() { a.scrollTreePages(-1) })
	bindNav("<Command-Next>", func() { a.scrollTreePages(1) })

	bindAny("<F5>", func() { a.reloadCommitsAsync() })

	bindNav("<KeyPress-Delete>", func() { a.scrollDetailPages(-1) })
	bindNav("<KeyPress-BackSpace>", func() { a.scrollDetailPages(-1) })
	bindNav("<KeyPress-b>", func() { a.scrollDetailPages(-1) })
	bindNav("<KeyPress-B>", func() { a.scrollDetailPages(-1) })
	bindNav("<KeyPress-space>", func() { a.scrollDetailPages(1) })
	bindNav("<KeyPress-u>", func() { a.scrollDetailLines(-18) })
	bindNav("<KeyPress-U>", func() { a.scrollDetailLines(-18) })
	bindNav("<KeyPress-d>", func() { a.scrollDetailLines(18) })
	bindNav("<KeyPress-D>", func() { a.scrollDetailLines(18) })

	bindAny("<KeyPress-/>", func() {
		if a.filterEntry == nil || a.filterHasFocus() {
			return
		}
		a.focusFilterEntry()
	})
}

func (a *Controller) filterHasFocus() bool {
	if a.filterEntry == nil {
		return false
	}
	return Focus() == a.filterEntry.String()
}

func (a *Controller) moveSelection(delta int) {
	if a.tree == nil || len(a.visible) == 0 {
		return
	}
	idx := a.currentSelectionIndex() + delta
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
	if sel[0] == moreIndicatorID {
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

	go a.populateDiff(entry, hash)
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
	for idx, entry := range a.visible {
		labels := a.branchLabels[entry.Commit.Hash.String()]
		graph := formatGraphValue(entry, labels)
		msg, author, when := commitListColumns(entry)
		vals := tclList(graph, msg, author, when)
		a.tree.Insert("", "end", Id(strconv.Itoa(idx)), Values(vals))
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
		parts[i] = fmt.Sprintf("{%s}", escapeTclString(v))
	}
	return strings.Join(parts, " ")
}

func escapeTclString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "{", `\{`)
	s = strings.ReplaceAll(s, "}", `\}`)
	return s
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
