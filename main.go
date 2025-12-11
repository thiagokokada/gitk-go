package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	. "modernc.org/tk9.0"
	_ "modernc.org/tk9.0/themes/azure"
)

const defaultLimit = 200

type commitEntry struct {
	commit     *object.Commit
	summary    string
	searchText string
}

type gitkApp struct {
	repoPath string
	limit    int

	repo    *git.Repository
	headRef string

	commits []*commitEntry
	visible []*commitEntry

	list        *ListboxWidget
	detail      *TextWidget
	status      *TLabelWidget
	filterEntry *TEntryWidget

	filterValue string

	selectedMu   sync.RWMutex
	selectedHash string
}

func main() {
	repoFlag := flag.String("repo", ".", "path to the Git repository to explore")
	limitFlag := flag.Int("limit", defaultLimit, "maximum number of commits to load")
	flag.Parse()

	app := &gitkApp{repoPath: *repoFlag, limit: *limitFlag}
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

	if err := a.loadCommits(); err != nil {
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

func (a *gitkApp) loadCommits() error {
	entries, head, err := a.scanCommits()
	if err != nil {
		return err
	}
	a.commits = entries
	a.visible = entries
	a.headRef = head
	return nil
}

func (a *gitkApp) scanCommits() ([]*commitEntry, string, error) {
	ref, err := a.repo.Head()
	if err != nil {
		return nil, "", fmt.Errorf("resolve HEAD: %w", err)
	}

	opts := &git.LogOptions{From: ref.Hash(), Order: git.LogOrderCommitterTime}
	iter, err := a.repo.Log(opts)
	if err != nil {
		return nil, "", fmt.Errorf("read commits: %w", err)
	}
	defer iter.Close()

	var entries []*commitEntry
	for len(entries) < a.limit {
		commit, err := iter.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", fmt.Errorf("iterate commits: %w", err)
		}
		entries = append(entries, newCommitEntry(commit))
	}

	head := ref.Name().Short()
	if head == "" {
		head = ref.Name().String()
	}

	return entries, head, nil
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

	pane := App.TPanedwindow(Orient(HORIZONTAL))
	Grid(pane, Row(1), Column(0), Sticky(NEWS), Padx("4p"), Pady("4p"))

	left := pane.TFrame()
	right := pane.TFrame()
	pane.Add(left.Window)
	pane.Add(right.Window)

	GridRowConfigure(left.Window, 0, Weight(1))
	GridColumnConfigure(left.Window, 0, Weight(1))
	GridRowConfigure(right.Window, 0, Weight(1))
	GridColumnConfigure(right.Window, 0, Weight(1))

	listScroll := left.TScrollbar(Command(func(e *Event) { e.Yview(a.list) }))
	a.list = left.Listbox(Selectmode(BROWSE), Exportselection(false), Font(CourierFont(), 11))
	a.list.Configure(Yscrollcommand(func(e *Event) { e.ScrollSet(listScroll) }))
	Grid(a.list, Row(0), Column(0), Sticky(NEWS))
	Grid(listScroll, Row(0), Column(1), Sticky(NS))

	Bind(a.list, "<<ListboxSelect>>", Command(a.onSelectionChanged))

	detailYScroll := right.TScrollbar(Command(func(e *Event) { e.Yview(a.detail) }))
	detailXScroll := right.TScrollbar(Orient(HORIZONTAL), Command(func(e *Event) { e.Xview(a.detail) }))
	a.detail = right.Text(Wrap(NONE), Font(CourierFont(), 11), Exportselection(false), Tabs("1c"))
	a.detail.Configure(Yscrollcommand(func(e *Event) { e.ScrollSet(detailYScroll) }))
	a.detail.Configure(Xscrollcommand(func(e *Event) { e.ScrollSet(detailXScroll) }))
	Grid(a.detail, Row(0), Column(0), Sticky(NEWS))
	Grid(detailYScroll, Row(0), Column(1), Sticky(NS))
	Grid(detailXScroll, Row(1), Column(0), Sticky(WE))
	a.detail.Configure(State("disabled"))

	a.status = App.TLabel(Anchor(W), Relief(SUNKEN), Padding("4p"))
	Grid(a.status, Row(2), Column(0), Sticky(WE))

	a.clearDetailText("Select a commit to view its details.")
}

func (a *gitkApp) onSelectionChanged() {
	if a.list == nil {
		return
	}
	selection := a.list.Curselection()
	if len(selection) == 0 {
		return
	}
	a.showCommitDetails(selection[0])
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
	a.writeDetailText(header + "\nLoading diff...")

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
		a.writeDetailText(output)
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
	go func() {
		entries, head, err := a.scanCommits()
		if err != nil {
			msg := fmt.Sprintf("Failed to reload commits: %v", err)
			log.Print(msg)
			a.setStatus(msg)
			return
		}
		PostEvent(func() {
			a.commits = entries
			a.visible = entries
			a.headRef = head
			currentFilter := a.filterValue
			if a.filterEntry != nil {
				currentFilter = a.filterEntry.Textvariable()
			}
			a.applyFilter(currentFilter)
			a.setStatus(a.statusSummary())
		}, false)
	}()
}

func (a *gitkApp) applyFilter(raw string) {
	if a.list == nil {
		return
	}
	a.filterValue = raw
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

	a.list.Delete(0, END)
	for _, entry := range a.visible {
		a.list.Insert(END, entry.summary)
	}

	if len(a.visible) == 0 {
		a.clearDetailText("No commits match the current filter.")
		a.setStatus(a.statusSummary())
		return
	}

	a.list.SelectionClear(0, END)
	a.list.SelectionSet(0)
	a.list.Activate(0)
	a.showCommitDetails(0)
	a.setStatus(a.statusSummary())
}

func (a *gitkApp) clearDetailText(msg string) {
	if a.detail == nil {
		return
	}
	a.writeDetailText(msg)
}

func (a *gitkApp) writeDetailText(content string) {
	if a.detail == nil {
		return
	}
	a.detail.Configure(State(NORMAL))
	a.detail.Delete("1.0", END)
	a.detail.Insert("1.0", content)
	a.detail.Configure(State("disabled"))
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
	base := fmt.Sprintf("Showing %d/%d commits on %s (limit %d) — %s", visible, total, head, a.limit, a.repoPath)
	if filterDesc == "" {
		return base
	}
	return fmt.Sprintf("Filter %q — %s", filterDesc, base)
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
