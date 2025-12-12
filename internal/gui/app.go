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
	_ "modernc.org/tk9.0/themes/azure"
)

const (
	autoLoadThreshold   = 0.98
	moreIndicatorID     = "__more__"
	loadingIndicatorID  = "__loading__"
	localUnstagedRowID  = "__local_unstaged__"
	localStagedRowID    = "__local_staged__"
	diffDebounceDelay   = 120 * time.Millisecond
	filterDebounceDelay = 240 * time.Millisecond
)

const (
	localUnstagedLabel = "Local uncommitted changes, not checked in to index"
	localStagedLabel   = "Local changes checked into index but not committed"
)

type Controller struct {
	svc                 *git.Service
	repoPath            string
	batch               int
	themePref           ThemePreference
	palette             colorPalette
	autoReloadRequested bool
	verbose             bool

	headRef string

	commits []*git.Entry
	visible []*git.Entry

	tree      treeState
	diff      diffState
	filter    filterState
	shortcuts shortcutState
	status    *TLabelWidget

	selection  selectionState
	localDiffs localDiffCache
	watch      autoReloadState
}

type treeState struct {
	widget            *TTreeviewWidget
	menu              *MenuWidget
	branchLabels      map[string][]string
	contextTargetID   string
	hasMore           bool
	loadingBatch      bool
	showLocalUnstaged bool
	showLocalStaged   bool
}

type diffState struct {
	detail       *TextWidget
	fileList     *ListboxWidget
	fileSections []git.FileSection
	syntaxTags   map[string]string

	mu          sync.Mutex
	loadTimer   *time.Timer
	pendingDiff *git.Entry
	pendingHash string
}

type shortcutState struct {
	window *ToplevelWidget
}

type filterState struct {
	entry *TEntryWidget
	value string

	mu      sync.Mutex
	timer   *time.Timer
	pending string
}

type selectionState struct {
	mu   sync.RWMutex
	hash string
}

func (s *selectionState) Set(hash string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hash = hash
}

func (s *selectionState) Get() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hash
}

type localDiffCache struct {
	mu    sync.Mutex
	items map[bool]*localDiffState
}

func (c *localDiffCache) state(staged bool, create bool) *localDiffState {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.items == nil {
		if !create {
			return nil
		}
		c.items = make(map[bool]*localDiffState)
	}
	if st, ok := c.items[staged]; ok {
		return st
	}
	if !create {
		return nil
	}
	st := &localDiffState{}
	c.items[staged] = st
	return st
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

func Run(repoPath string, batch int, pref ThemePreference, autoReload bool, verbose bool) error {
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
		svc:                 svc,
		repoPath:            svc.RepoPath(),
		batch:               batch,
		themePref:           pref,
		autoReloadRequested: autoReload,
		verbose:             verbose,
	}
	app.diff.syntaxTags = make(map[string]string)
	return app.run()
}

func (a *Controller) run() error {
	defer a.shutdown()
	a.palette = paletteForPreference(a.themePref)
	if a.palette.ThemeName != "" {
		ActivateTheme(a.palette.ThemeName)
	}
	applyAppIcon()
	a.buildUI()
	a.initAutoReload(a.autoReloadRequested)
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
	a.tree.branchLabels = labels
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
	prevUnstaged := a.tree.showLocalUnstaged
	prevStaged := a.tree.showLocalStaged
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
	diff, sections := prepareDiffDisplay(diff, snap.sections)
	a.writeDetailText(diff, len(sections) > 0)
	a.setFileSections(sections)
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
	defer state.Unlock()
	if state.loading {
		return
	}
	if state.ready && !force {
		return
	}
	state.loading = true
	state.ready = false
	state.diff = ""
	state.sections = nil
	state.err = nil
	state.generation++
	gen := state.generation
	go a.computeLocalDiff(staged, gen)
}

func (a *Controller) computeLocalDiff(staged bool, gen int) {
	if a.svc == nil {
		return
	}
	diff, sections, err := a.svc.WorktreeDiff(staged)
	state := a.localDiffState(staged, true)
	state.Lock()
	defer state.Unlock()
	if gen != state.generation {
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
	defer state.Unlock()
	state.loading = false
	state.ready = false
	state.diff = ""
	state.sections = nil
	state.err = nil
	state.generation++
}

func (a *Controller) localDiffState(staged bool, create bool) *localDiffState {
	return a.localDiffs.state(staged, create)
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
	if a.tree.widget == nil {
		return
	}
	sel := a.tree.widget.Selection("")
	if len(sel) == 0 || sel[0] != targetID {
		return
	}
	a.renderLocalChanges(staged, false)
}

func (a *Controller) populateDiff(entry *git.Entry, hash string) {
	diff, sections, err := a.svc.Diff(entry.Commit)
	if err != nil {
		diff = fmt.Sprintf("Unable to compute diff: %v", err)
	}
	diff, sections = prepareDiffDisplay(diff, sections)
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
	a.diff.mu.Lock()
	defer a.diff.mu.Unlock()
	a.diff.pendingDiff = entry
	a.diff.pendingHash = hash
	if a.diff.loadTimer != nil {
		a.diff.loadTimer.Stop()
	}
	a.diff.loadTimer = time.AfterFunc(diffDebounceDelay, func() {
		a.diff.mu.Lock()
		defer a.diff.mu.Unlock()
		pending := a.diff.pendingDiff
		pendingHash := a.diff.pendingHash
		a.diff.pendingDiff = nil
		a.diff.pendingHash = ""
		a.diff.loadTimer = nil
		if pending == nil {
			return
		}
		go a.populateDiff(pending, pendingHash)
	})
}

func (a *Controller) cancelPendingDiffLoad() {
	a.diff.mu.Lock()
	defer a.diff.mu.Unlock()
	if a.diff.loadTimer != nil {
		a.diff.loadTimer.Stop()
		a.diff.loadTimer = nil
	}
	a.diff.pendingDiff = nil
	a.diff.pendingHash = ""
}

func (a *Controller) reloadCommitsAsync() {
	if a.tree.loadingBatch {
		return
	}
	a.tree.loadingBatch = true
	currentFilter := a.filter.value
	go func(filter string) {
		entries, head, hasMore, err := a.svc.ScanCommits(0, a.batch)
		PostEvent(func() {
			a.tree.loadingBatch = false
			if err != nil {
				msg := fmt.Sprintf("Failed to reload commits: %v", err)
				log.Print(msg)
				a.setStatus(msg)
				return
			}
			a.commits = entries
			a.visible = entries
			a.headRef = head
			a.tree.hasMore = hasMore
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
	if a.tree.loadingBatch || (!prefetch && !a.tree.hasMore) {
		return
	}
	a.tree.loadingBatch = true
	currentFilter := a.filter.value
	skip := len(a.commits)
	go func(filter string, skipCount int, background bool) {
		entries, _, hasMore, err := a.svc.ScanCommits(skipCount, a.batch)
		PostEvent(func() {
			a.tree.loadingBatch = false
			if err != nil {
				msg := fmt.Sprintf("Failed to load more commits: %v", err)
				log.Print(msg)
				if !background {
					a.setStatus(msg)
				}
				return
			}
			if len(entries) == 0 {
				a.tree.hasMore = false
				if !background {
					a.setStatus("No more commits available.")
				}
				return
			}
			a.commits = append(a.commits, entries...)
			a.tree.hasMore = hasMore
			if err := a.loadBranchLabels(); err != nil {
				log.Printf("failed to refresh branch labels: %v", err)
			}
			a.applyFilter(filter)
			a.refreshLocalChangesAsync(false)
			a.setStatus(a.statusSummary())
			if background && a.tree.hasMore {
				go a.loadMoreCommitsAsync(true)
			}
		}, false)
	}(currentFilter, skip, prefetch)
}

func (a *Controller) clearDetailText(msg string) {
	if a.diff.detail == nil {
		return
	}
	a.writeDetailText(msg, false)
	a.setFileSections(nil)
}

func (a *Controller) writeDetailText(content string, highlightDiff bool) {
	if a.diff.detail == nil {
		return
	}
	a.diff.detail.Configure(State(NORMAL))
	a.diff.detail.Delete("1.0", END)
	a.diff.detail.Insert("1.0", content)
	if highlightDiff {
		a.highlightDiffLines(content)
		a.applySyntaxHighlight(content)
	} else {
		a.diff.detail.TagRemove("diffAdd", "1.0", END)
		a.diff.detail.TagRemove("diffDel", "1.0", END)
		a.diff.detail.TagRemove("diffHeader", "1.0", END)
		a.clearSyntaxHighlight()
	}
	a.diff.detail.Configure(State("disabled"))
}

func (a *Controller) highlightDiffLines(content string) {
	a.diff.detail.TagRemove("diffAdd", "1.0", END)
	a.diff.detail.TagRemove("diffDel", "1.0", END)
	a.diff.detail.TagRemove("diffHeader", "1.0", END)
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
		lineNo := i + 1
		start := fmt.Sprintf("%d.0", lineNo)
		end := fmt.Sprintf("%d.0", lineNo+1)
		if lineNo == len(lines) {
			end = fmt.Sprintf("%d.end", lineNo)
		}
		a.diff.detail.TagAdd(tag, start, end)
	}
}

func prepareDiffDisplay(content string, sections []git.FileSection) (string, []git.FileSection) {
	if content == "" {
		return content, sections
	}
	lines := strings.Split(content, "\n")
	var b strings.Builder
	newSections := make([]git.FileSection, len(sections))
	copy(newSections, sections)
	extraLines := 0
	nextSection := 0
	for i, line := range lines {
		lineNo := i + 1
		for nextSection < len(newSections) && newSections[nextSection].Line == lineNo {
			newSections[nextSection].Line = lineNo + extraLines
			nextSection++
		}
		if strings.HasPrefix(line, "diff --git ") && b.Len() > 0 {
			b.WriteString("\n")
			extraLines++
		}
		b.WriteString(line)
		if i < len(lines)-1 {
			b.WriteString("\n")
		}
	}
	for nextSection < len(newSections) {
		newSections[nextSection].Line += extraLines
		nextSection++
	}
	return b.String(), newSections
}

func (a *Controller) setFileSections(sections []git.FileSection) {
	a.diff.fileSections = sections
	if a.diff.fileList == nil {
		return
	}
	a.diff.fileList.Configure(State("normal"))
	a.diff.fileList.Delete(0, END)
	if len(sections) == 0 {
		a.diff.fileList.Insert(END, "(no files)")
		a.diff.fileList.Configure(State("disabled"))
		return
	}
	for _, sec := range sections {
		a.diff.fileList.Insert(END, sec.Path)
	}
	a.diff.fileList.SelectionClear(0, END)
	a.diff.fileList.Activate(0)
	a.diff.fileList.Configure(State("normal"))
}

func (a *Controller) onFileSelectionChanged() {
	if len(a.diff.fileSections) == 0 || a.diff.fileList == nil {
		return
	}
	selection := a.diff.fileList.Curselection()
	if len(selection) == 0 {
		return
	}
	idx := selection[0]
	if idx < 0 || idx >= len(a.diff.fileSections) {
		return
	}
	a.scrollDiffToLine(a.diff.fileSections[idx].Line)
}

func (a *Controller) scrollDiffToLine(line int) {
	if a.diff.detail == nil || line <= 0 {
		return
	}
	defer func() {
		recover()
	}()
	totalLines := a.textLineCount()
	if totalLines <= 1 {
		a.diff.detail.Yviewmoveto(0)
		return
	}
	fraction := float64(line-1) / float64(totalLines-1)
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	a.diff.detail.Yviewmoveto(fraction)
}

func (a *Controller) textLineCount() int {
	if a.diff.detail == nil {
		return 0
	}
	index := a.diff.detail.Index(END)
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
	a.selection.Set(hash)
}

func (a *Controller) currentSelection() string {
	return a.selection.Get()
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

func (a *Controller) debugf(format string, args ...any) {
	if !a.verbose {
		return
	}
	log.Printf(format, args...)
}

func (a *Controller) statusSummary() string {
	total := len(a.commits)
	visible := len(a.visible)
	head := a.headRef
	if head == "" {
		head = "HEAD"
	}
	filterDesc := strings.TrimSpace(a.filter.value)
	path := a.repoPath
	if path == "" && a.svc != nil {
		path = a.svc.RepoPath()
	}
	base := fmt.Sprintf("Showing %d/%d loaded commits on %s — %s", visible, total, head, path)
	if a.tree.hasMore {
		base += " (more available)"
	}
	if filterDesc == "" {
		return base
	}
	return fmt.Sprintf("Filter %q — %s", filterDesc, base)
}
