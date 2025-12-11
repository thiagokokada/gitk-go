package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	diff "github.com/go-git/go-git/v5/plumbing/format/diff"
	"github.com/go-git/go-git/v5/plumbing/object"

	. "modernc.org/tk9.0"
	evalext "modernc.org/tk9.0/extensions/eval"
	_ "modernc.org/tk9.0/themes/azure"
)

const (
	defaultLimit      = 1000
	autoLoadThreshold = 0.98
	moreIndicatorID   = "__more__"
)

type commitEntry struct {
	commit     *object.Commit
	summary    string
	searchText string
	graph      string
}

type gitkApp struct {
	repoPath string
	batch    int

	repo    *git.Repository
	headRef string

	commits []*commitEntry
	visible []*commitEntry

	tree         *TTreeviewWidget
	fileList     *ListboxWidget
	detail       *TextWidget
	status       *TLabelWidget
	filterEntry  *TEntryWidget
	fileSections []fileSection
	branchLabels map[string][]string

	filterValue  string
	hasMore      bool
	loadingBatch bool

	selectedMu   sync.RWMutex
	selectedHash string
}

type fileSection struct {
	path string
	line int
}

func main() {
	batchFlag := flag.Int("limit", defaultLimit, "number of commits to load per batch (default 200)")
	flag.Parse()

	if err := InitializeExtension("eval"); err != nil && err != AlreadyInitialized {
		log.Fatalf("init eval extension: %v", err)
	}

	repoPath := "."
	if args := flag.Args(); len(args) > 0 {
		last := args[len(args)-1]
		if strings.HasPrefix(last, "-") {
			// treat as flag without repo argument
		} else {
			repoPath = last
		}
	}

	app := &gitkApp{repoPath: repoPath, batch: *batchFlag}
	if err := app.run(); err != nil {
		log.Fatalf("gitk-go: %v", err)
	}
}

func (a *gitkApp) run() error {
	abs, err := filepath.Abs(a.repoPath)
	if err == nil {
		a.repoPath = abs
	}

	repo, err := git.PlainOpenWithOptions(a.repoPath, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return fmt.Errorf("open repository: %w", err)
	}
	a.repo = repo

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

func (a *gitkApp) loadInitialCommits() error {
	entries, head, hasMore, err := a.scanCommits(0, a.batch)
	if err != nil {
		return err
	}
	a.commits = entries
	a.visible = entries
	a.headRef = head
	a.hasMore = hasMore
	if a.batch <= 0 {
		a.batch = defaultLimit
	}
	return nil
}

func (a *gitkApp) loadBranchLabels() error {
	labels := map[string][]string{}
	if a.repo == nil {
		a.branchLabels = labels
		return nil
	}
	refs, err := a.repo.References()
	if err != nil {
		return err
	}
	defer refs.Close()

	headRef, err := a.repo.Head()
	var headHash plumbing.Hash
	var headBranch string
	if err == nil && headRef != nil {
		headHash = headRef.Hash()
		if headRef.Name().IsBranch() {
			headBranch = headRef.Name().Short()
		}
	}

	err = refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Type() != plumbing.HashReference {
			return nil
		}
		if !ref.Name().IsBranch() {
			return nil
		}
		hash := ref.Hash().String()
		labels[hash] = append(labels[hash], ref.Name().Short())
		return nil
	})
	if err != nil {
		return err
	}

	if headHash != plumbing.ZeroHash {
		key := headHash.String()
		label := "HEAD"
		if headBranch != "" {
			label = fmt.Sprintf("HEAD -> %s", headBranch)
		}
		labels[key] = append([]string{label}, labels[key]...)
	}

	a.branchLabels = labels
	return nil
}

func (a *gitkApp) scanCommits(skip, batch int) ([]*commitEntry, string, bool, error) {
	if batch <= 0 {
		batch = defaultLimit
	}

	ref, err := a.repo.Head()
	if err != nil {
		if err == plumbing.ErrReferenceNotFound {
			return nil, "", false, nil
		}
		return nil, "", false, fmt.Errorf("resolve HEAD: %w", err)
	}

	opts := &git.LogOptions{From: ref.Hash(), Order: git.LogOrderDFS}
	iter, err := a.repo.Log(opts)
	if err != nil {
		return nil, "", false, fmt.Errorf("read commits: %w", err)
	}
	defer iter.Close()

	for skipped := 0; skipped < skip; skipped++ {
		if _, err := iter.Next(); err != nil {
			if err == io.EOF {
				return nil, refName(ref), false, nil
			}
			return nil, "", false, fmt.Errorf("iterate commits: %w", err)
		}
	}

	var entries []*commitEntry
	for len(entries) < batch {
		commit, err := iter.Next()
		if err == io.EOF {
			return entries, refName(ref), false, nil
		}
		if err != nil {
			return nil, "", false, fmt.Errorf("iterate commits: %w", err)
		}
		entries = append(entries, newCommitEntry(commit))
	}

	hasMore := false
	if _, err := iter.Next(); err == nil {
		hasMore = true
	} else if err != io.EOF {
		return nil, "", false, fmt.Errorf("iterate commits: %w", err)
	}

	if err := a.populateGraphStrings(entries, skip); err != nil {
		log.Printf("unable to compute graph column: %v", err)
	}
	return entries, refName(ref), hasMore, nil
}

func (a *gitkApp) populateGraphStrings(entries []*commitEntry, skip int) error {
	if len(entries) == 0 {
		return nil
	}
	total := skip + len(entries)
	if total <= 0 {
		return nil
	}
	args := []string{
		"-C", a.repoPath,
		"log",
		"--graph",
		"--no-color",
		"--topo-order",
		fmt.Sprintf("--max-count=%d", total),
		"--format=%H",
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("git", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("git log --graph: %s", msg)
	}
	rows, err := parseGraphRows(stdout.Bytes())
	if err != nil {
		return err
	}
	if len(rows) < total {
		return fmt.Errorf("git graph returned %d rows, need %d", len(rows), total)
	}
	rows = rows[skip:]
	if len(rows) > len(entries) {
		rows = rows[:len(entries)]
	}
	graphByHash := make(map[string]string, len(rows))
	for _, row := range rows {
		graphByHash[row.hash] = row.graph
	}
	for _, entry := range entries {
		entry.graph = graphByHash[entry.commit.Hash.String()]
	}
	return nil
}

type graphRow struct {
	hash  string
	graph string
}

func parseGraphRows(data []byte) ([]graphRow, error) {
	var rows []graphRow
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if row, ok := parseGraphLine(line); ok {
			rows = append(rows, row)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return rows, nil
}

func parseGraphLine(line string) (graphRow, bool) {
	trimmed := strings.TrimRight(line, " ")
	if len(trimmed) < 40 {
		return graphRow{}, false
	}
	hash := trimmed[len(trimmed)-40:]
	if !isHexHash(hash) {
		return graphRow{}, false
	}
	graph := strings.TrimRight(trimmed[:len(trimmed)-40], " ")
	return graphRow{hash: hash, graph: graph}, true
}

func isHexHash(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

func (a *gitkApp) buildUI() {
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
}

func (a *gitkApp) onTreeSelectionChanged() {
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

func (a *gitkApp) showCommitDetails(index int) {
	if index < 0 || index >= len(a.visible) {
		a.clearDetailText("Commit index out of range.")
		return
	}
	entry := a.visible[index]
	header := formatCommitHeader(entry.commit)
	hash := entry.commit.Hash.String()
	a.setSelectedHash(hash)
	a.setFileSections(nil)
	a.writeDetailText(header+"\nLoading diff...", false)

	go a.populateDiff(entry, hash)
}

func (a *gitkApp) populateDiff(entry *commitEntry, hash string) {
	diff, sections, err := a.diffForCommit(entry.commit)
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

func (a *gitkApp) diffForCommit(c *object.Commit) (string, []fileSection, error) {
	currentTree, err := c.Tree()
	if err != nil {
		return "", nil, err
	}
	var parentTree *object.Tree
	if c.NumParents() > 0 {
		parent, err := c.Parent(0)
		if err != nil {
			return "", nil, err
		}
		parentTree, err = parent.Tree()
		if err != nil {
			return "", nil, err
		}
	}
	changes, err := object.DiffTree(parentTree, currentTree)
	if err != nil {
		return "", nil, err
	}
	if len(changes) == 0 {
		header := formatCommitHeader(c)
		return header + "\nNo file level changes.", nil, nil
	}
	patch, err := changes.Patch()
	if err != nil {
		return "", nil, err
	}
	header := formatCommitHeader(c)
	var sections []fileSection
	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n")
	lineNo := strings.Count(header+"\n", "\n")
	for _, fp := range patch.FilePatches() {
		path := filePatchPath(fp)
		fileHeader := fmt.Sprintf("diff --git a/%s b/%s\n", path, path)
		headerLine := lineNo + 1
		b.WriteString(fileHeader)
		lineNo += strings.Count(fileHeader, "\n")

		if fp.IsBinary() {
			binaryInfo := "(binary files differ)\n"
			b.WriteString(binaryInfo)
			lineNo++
		} else {
			for _, chunk := range fp.Chunks() {
				if chunk == nil {
					continue
				}
				lines := strings.Split(chunk.Content(), "\n")
				for i, line := range lines {
					if i == len(lines)-1 && line == "" {
						continue
					}
					var prefix string
					switch chunk.Type() {
					case diff.Add:
						prefix = "+"
					case diff.Delete:
						prefix = "-"
					default:
						prefix = " "
					}
					b.WriteString(prefix + line + "\n")
					lineNo++
				}
			}
		}
		sections = append(sections, fileSection{path: path, line: headerLine})
	}
	return b.String(), sections, nil
}

func (a *gitkApp) reloadCommitsAsync() {
	if a.loadingBatch {
		return
	}
	a.loadingBatch = true
	currentFilter := a.filterValue
	go func(filter string) {
		entries, head, hasMore, err := a.scanCommits(0, a.batch)
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

func (a *gitkApp) loadMoreCommitsAsync(prefetch bool) {
	if a.loadingBatch || (!prefetch && !a.hasMore) {
		return
	}
	a.loadingBatch = true
	currentFilter := a.filterValue
	skip := len(a.commits)
	go func(filter string, skipCount int, background bool) {
		entries, _, hasMore, err := a.scanCommits(skipCount, a.batch)
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

func (a *gitkApp) applyFilter(raw string) {
	a.filterValue = raw
	if a.tree == nil {
		return
	}
	query := strings.ToLower(strings.TrimSpace(raw))
	if query == "" {
		a.visible = a.commits
	} else {
		var filtered []*commitEntry
		for _, entry := range a.commits {
			if strings.Contains(entry.searchText, query) {
				filtered = append(filtered, entry)
			}
		}
		a.visible = filtered
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
		labels := a.branchLabels[entry.commit.Hash.String()]
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

func (a *gitkApp) scheduleAutoLoadCheck() {
	if a.tree == nil || a.filterValue == "" || !a.hasMore {
		return
	}
	PostEvent(func() {
		a.maybeLoadMoreOnScroll()
	}, false)
}

func (a *gitkApp) maybeLoadMoreOnScroll() {
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

func (a *gitkApp) treeYviewRange() (float64, float64, error) {
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

func (a *gitkApp) clearDetailText(msg string) {
	if a.detail == nil {
		return
	}
	a.writeDetailText(msg, false)
	a.setFileSections(nil)
}

func (a *gitkApp) writeDetailText(content string, highlightDiff bool) {
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

func (a *gitkApp) highlightDiffLines(content string) {
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

func (a *gitkApp) setFileSections(sections []fileSection) {
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
		a.fileList.Insert(END, sec.path)
	}
	a.fileList.SelectionClear(0, END)
	a.fileList.Activate(0)
	a.fileList.Configure(State("normal"))
}

func (a *gitkApp) onFileSelectionChanged() {
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
	a.scrollDiffToLine(a.fileSections[idx].line)
}

func (a *gitkApp) scrollDiffToLine(line int) {
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

func (a *gitkApp) textLineCount() int {
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

func (a *gitkApp) setSelectedHash(hash string) {
	a.selectedMu.Lock()
	defer a.selectedMu.Unlock()
	a.selectedHash = hash
}

func (a *gitkApp) currentSelection() string {
	a.selectedMu.RLock()
	defer a.selectedMu.RUnlock()
	return a.selectedHash
}

func (a *gitkApp) setStatus(msg string) {
	if a.status == nil {
		log.Print(msg)
		return
	}
	text := msg
	PostEvent(func() {
		a.status.Configure(Txt(text))
	}, false)
}

func (a *gitkApp) statusSummary() string {
	total := len(a.commits)
	visible := len(a.visible)
	head := a.headRef
	if head == "" {
		head = "HEAD"
	}
	filterDesc := strings.TrimSpace(a.filterValue)
	base := fmt.Sprintf("Showing %d/%d loaded commits on %s — %s", visible, total, head, a.repoPath)
	if a.hasMore {
		base += " (more available)"
	}
	if filterDesc == "" {
		return base
	}
	return fmt.Sprintf("Filter %q — %s", filterDesc, base)
}

func refName(ref *plumbing.Reference) string {
	name := ref.Name().Short()
	if name == "" {
		name = ref.Name().String()
	}
	return name
}

func newCommitEntry(c *object.Commit) *commitEntry {
	summary := formatSummary(c)
	var b strings.Builder
	b.WriteString(strings.ToLower(c.Hash.String()))
	b.WriteByte(' ')
	b.WriteString(strings.ToLower(c.Author.Name))
	b.WriteByte(' ')
	b.WriteString(strings.ToLower(c.Author.Email))
	b.WriteByte(' ')
	b.WriteString(strings.ToLower(c.Message))
	return &commitEntry{commit: c, summary: summary, searchText: b.String()}
}

func formatSummary(c *object.Commit) string {
	firstLine := strings.SplitN(strings.TrimSpace(c.Message), "\n", 2)[0]
	if len(firstLine) > 80 {
		firstLine = firstLine[:77] + "..."
	}
	timestamp := c.Author.When.Format("2006-01-02 15:04")
	return fmt.Sprintf("%s  %s  %s", c.Hash.String()[:7], timestamp, firstLine)
}

func formatCommitHeader(c *object.Commit) string {
	var b strings.Builder
	fmt.Fprintf(&b, "commit %s\n", c.Hash)
	fmt.Fprintf(&b, "Author: %s <%s>\n", c.Author.Name, c.Author.Email)
	fmt.Fprintf(&b, "Date:   %s\n\n", c.Author.When.Format(time.RFC1123))

	message := strings.TrimRight(c.Message, "\n")
	if message == "" {
		b.WriteString("    (no commit message)\n")
		return b.String()
	}
	for _, line := range strings.Split(message, "\n") {
		if line == "" {
			b.WriteString("\n")
			continue
		}
		fmt.Fprintf(&b, "    %s\n", line)
	}
	return b.String()
}

func commitListColumns(entry *commitEntry) (msg, author, when string) {
	firstLine := strings.SplitN(strings.TrimSpace(entry.commit.Message), "\n", 2)[0]
	if len(firstLine) > 80 {
		firstLine = firstLine[:77] + "..."
	}
	msg = fmt.Sprintf("%s  %s", entry.commit.Hash.String()[:7], firstLine)
	author = fmt.Sprintf("%s <%s>", entry.commit.Author.Name, entry.commit.Author.Email)
	when = entry.commit.Author.When.Format("2006-01-02 15:04")
	return
}

func formatGraphValue(entry *commitEntry, labels []string) string {
	graph := strings.TrimRight(entry.graph, " ")
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

func filePatchPath(fp diff.FilePatch) string {
	from, to := fp.Files()
	if to != nil && to.Path() != "" {
		return to.Path()
	}
	if from != nil && from.Path() != "" {
		return from.Path()
	}
	return "(unknown)"
}
