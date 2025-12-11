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
	loadingIndicatorID = "__loading__"
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
	a.buildUI()
	a.showInitialLoadingRow()
	a.setStatus("Loading commits...")
	a.refreshLocalChangesAsync(true)
	a.reloadCommitsAsync()
	App.WmTitle("gitk-go")
	App.SetResizable(true, true)
	App.Center().Wait()
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
	case loadingIndicatorID:
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
