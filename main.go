package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	. "modernc.org/tk9.0"
	_ "modernc.org/tk9.0/themes/azure"
)

const defaultLimit = 1000

type commitEntry struct {
	commit     *object.Commit
	summary    string
	searchText string
}

type gitkApp struct {
	repoPath string
	batch    int

	repo    *git.Repository
	headRef string

	commits []*commitEntry
	visible []*commitEntry

	tree        *TTreeviewWidget
	detail      *TextWidget
	status      *TLabelWidget
	filterEntry *TEntryWidget
	loadMoreBtn *TButtonWidget

	filterValue  string
	hasMore      bool
	loadingBatch bool

	selectedMu   sync.RWMutex
	selectedHash string
}

func main() {
	batchFlag := flag.Int("limit", defaultLimit, "number of commits to load per batch (default 200)")
	flag.Parse()

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

	opts := &git.LogOptions{From: ref.Hash(), Order: git.LogOrderCommitterTime}
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

	return entries, refName(ref), hasMore, nil
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
		Columns("commit author date"),
		Selectmode("browse"),
		Height(18),
		Yscrollcommand(func(e *Event) { e.ScrollSet(treeScroll) }),
	)
	a.tree.Column("commit", Anchor(W), Width(500))
	a.tree.Column("author", Anchor(W), Width(280))
	a.tree.Column("date", Anchor(W), Width(180))
	a.tree.Heading("commit", Txt("Commit"))
	a.tree.Heading("author", Txt("Author"))
	a.tree.Heading("date", Txt("Date"))
	Grid(a.tree, Row(0), Column(0), Sticky(NEWS))
	Grid(treeScroll, Row(0), Column(1), Sticky(NS))
	treeScroll.Configure(Command(func(e *Event) { e.Yview(a.tree) }))

	a.loadMoreBtn = listArea.TButton(Txt("Load more commits"), Command(func() {
		a.loadMoreCommitsAsync(false)
	}))
	Grid(a.loadMoreBtn, Row(1), Column(0), Columnspan(2), Sticky(WE), Pady("4p"))

	Bind(a.tree, "<<TreeviewSelect>>", Command(a.onTreeSelectionChanged))

	detailYScroll := diffArea.TScrollbar(Command(func(e *Event) { e.Yview(a.detail) }))
	detailXScroll := diffArea.TScrollbar(Orient(HORIZONTAL), Command(func(e *Event) { e.Xview(a.detail) }))
	a.detail = diffArea.Text(Wrap(NONE), Font(CourierFont(), 11), Exportselection(false), Tabs("1c"))
	a.detail.Configure(Yscrollcommand(func(e *Event) { e.ScrollSet(detailYScroll) }))
	a.detail.Configure(Xscrollcommand(func(e *Event) { e.ScrollSet(detailXScroll) }))
	a.detail.TagConfigure("diffAdd", Background("#dff5de"))
	a.detail.TagConfigure("diffDel", Background("#f9d6d5"))
	Grid(a.detail, Row(0), Column(0), Sticky(NEWS))
	Grid(detailYScroll, Row(0), Column(1), Sticky(NS))
	Grid(detailXScroll, Row(1), Column(0), Sticky(WE))
	a.detail.Configure(State("disabled"))

	a.status = App.TLabel(Anchor(W), Relief(SUNKEN), Padding("4p"))
	Grid(a.status, Row(2), Column(0), Sticky(WE))

	a.clearDetailText("Select a commit to view its details.")
	a.updateLoadMoreState()
}

func (a *gitkApp) onTreeSelectionChanged() {
	if a.tree == nil {
		return
	}
	sel := a.tree.Selection("")
	if len(sel) == 0 {
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
	a.writeDetailText(header+"\nLoading diff...", false)

	go a.populateDiff(entry, header, hash)
}

func (a *gitkApp) populateDiff(entry *commitEntry, header, hash string) {
	diff, err := a.diffForCommit(entry.commit)
	if err != nil {
		diff = fmt.Sprintf("Unable to compute diff: %v", err)
	}
	output := header + "\n" + diff
	PostEvent(func() {
		if a.currentSelection() != hash {
			return
		}
		a.writeDetailText(output, true)
	}, false)
}

func (a *gitkApp) diffForCommit(c *object.Commit) (string, error) {
	currentTree, err := c.Tree()
	if err != nil {
		return "", err
	}
	var parentTree *object.Tree
	if c.NumParents() > 0 {
		parent, err := c.Parent(0)
		if err != nil {
			return "", err
		}
		parentTree, err = parent.Tree()
		if err != nil {
			return "", err
		}
	}
	changes, err := object.DiffTree(parentTree, currentTree)
	if err != nil {
		return "", err
	}
	if len(changes) == 0 {
		return "No file level changes.", nil
	}
	patch, err := changes.Patch()
	if err != nil {
		return "", err
	}
	return patch.String(), nil
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
			a.applyFilter(filter)
			a.setStatus(a.statusSummary())
			a.updateLoadMoreState()
		}, false)
	}(currentFilter)
}

func (a *gitkApp) loadMoreCommitsAsync(prefetch bool) {
	if a.loadingBatch || (!prefetch && !a.hasMore) {
		return
	}
	a.loadingBatch = true
	if !prefetch {
		a.updateLoadMoreState()
	}
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
			a.applyFilter(filter)
			a.setStatus(a.statusSummary())
			if background && a.hasMore {
				go a.loadMoreCommitsAsync(true)
			} else if !background {
				a.updateLoadMoreState()
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
		msg, author, when := commitListColumns(entry)
		vals := tclList(msg, author, when)
		a.tree.Insert("", "end", Id(strconv.Itoa(idx)), Values(vals))
	}

	if len(a.visible) == 0 {
		if len(a.commits) == 0 {
			a.clearDetailText("Repository has no commits yet.")
		} else {
			a.clearDetailText("No commits match the current filter.")
		}
		a.setStatus(a.statusSummary())
		a.updateLoadMoreState()
		return
	}

	firstID := strconv.Itoa(0)
	a.tree.Selection("set", firstID)
	a.tree.Focus(firstID)
	a.showCommitDetails(0)
	a.setStatus(a.statusSummary())
	a.updateLoadMoreState()
}

func (a *gitkApp) clearDetailText(msg string) {
	if a.detail == nil {
		return
	}
	a.writeDetailText(msg, false)
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
	}
	a.detail.Configure(State("disabled"))
}

func (a *gitkApp) highlightDiffLines(content string) {
	a.detail.TagRemove("diffAdd", "1.0", END)
	a.detail.TagRemove("diffDel", "1.0", END)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if len(line) == 0 {
			continue
		}
		tag := ""
		switch {
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

func (a *gitkApp) updateLoadMoreState() {
	if a.loadMoreBtn == nil {
		return
	}
	state := "normal"
	if a.loadingBatch || !a.hasMore {
		state = "disabled"
	}
	a.loadMoreBtn.Configure(State(state))
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
