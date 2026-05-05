package gui

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/thiagokokada/gitk-go/internal/git"

	. "modernc.org/tk9.0"
	_ "modernc.org/tk9.0/themes/azure" // load theme
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

// RunConfig describes the parameters that control the GUI runtime.
type RunConfig struct {
	RepoPath        string
	Batch           uint
	GraphMaxColumns uint
	GraphCanvas     bool
	ThemePreference ThemePreference
	ThemeActivator  func(string) error
	AutoReload      bool
	SyntaxHighlight bool
	Verbose         bool
}

func Run(cfg RunConfig) error {
	if cfg.RepoPath == "" {
		cfg.RepoPath = "."
	}
	if err := InitializeExtension("eval"); err != nil && err != AlreadyInitialized {
		return fmt.Errorf("init eval extension: %v", err)
	}
	svc, err := git.Open(cfg.RepoPath)
	if err != nil {
		return err
	}
	svc.SetGraphMaxColumns(int(cfg.GraphMaxColumns))
	pref := cfg.ThemePreference
	if pref < ThemeAuto || pref > ThemeDark {
		pref = ThemeAuto
	}
	app := &Controller{
		svc: svc,
		cfg: controllerConfig{
			batch:               cfg.Batch,
			graphCanvas:         cfg.GraphCanvas,
			autoReloadRequested: cfg.AutoReload,
			syntaxHighlight:     cfg.SyntaxHighlight,
			verbose:             cfg.Verbose,
		},
		repo: controllerRepo{
			path: svc.RepoPath(),
		},
		theme: controllerTheme{
			pref:     pref,
			activate: cfg.ThemeActivator,
		},
	}
	app.initDebouncedActions()
	app.state.diff.syntaxTags = make(map[string]string)
	return app.run()
}

func (a *Controller) initDebouncedActions() {
	a.state.filter.debounce.Configure(filterDebounceDelay, func(value string) {
		if value == "" {
			return
		}
		PostEvent(func() {
			a.applyFilter(value)
		}, false)
	})
	a.state.diff.debounce.Configure(diffDebounceDelay, func(req diffRequest) {
		if req.entry == nil {
			return
		}
		go a.populateDiff(req.entry, req.hash)
	})
}

func (a *Controller) run() error {
	defer a.shutdown()
	a.applyThemePalette(paletteForPreference(a.theme.pref))
	level := slog.LevelInfo
	if a.cfg.verbose {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
	applyAppIcon()
	a.buildUI()
	a.loadPreferences()
	a.startThemeWatch()
	a.initAutoReload(a.cfg.autoReloadRequested)
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
	a.state.tree.branchLabels = labels
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
			slog.Error("local changes", slog.Any("error", err))
			return
		}
		PostEvent(func() {
			a.applyLocalChangeStatus(status, repoReady, prefetch)
		}, false)
	}()
}

func (a *Controller) applyLocalChangeStatus(status git.LocalChanges, repoReady bool, prefetch bool) {
	actions := a.state.tree.localChangePlan(repoReady, prefetch, status)
	a.setLocalRowVisibility(false, actions.showUnstaged)
	a.setLocalRowVisibility(true, actions.showStaged)
	if actions.resetUnstaged {
		a.resetLocalDiffState(false)
	}
	if actions.resetStaged {
		a.resetLocalDiffState(true)
	}
	if actions.loadUnstaged {
		a.ensureLocalDiffLoading(false, true)
	}
	if actions.loadStaged {
		a.ensureLocalDiffLoading(true, true)
	}
}

func (a *Controller) showCommitDetails(entry *git.Entry, index int) {
	header := git.FormatCommitHeader(entry.Commit)
	hash := entry.Commit.Hash
	a.state.selection.SetCommit(entry, index)
	a.setFileSections(nil)
	a.writeDetailText(header+"\nLoading diff...", false)
	a.scheduleDiffLoad(entry, hash)
}

func (a *Controller) selectFallbackCommit() {
	if len(a.data.visible) == 0 {
		a.state.selection.Clear()
		if len(a.data.commits) == 0 {
			a.clearDetailText("Repository has no commits yet.")
		} else {
			a.clearDetailText("No commits match the current filter.")
		}
		a.setStatus(a.statusSummary())
		return
	}
	entry, ok := a.commitEntryAt(0)
	if !ok {
		return
	}
	id := commitRowID(entry)
	if id != "" {
		a.ui.treeView.Selection("set", id)
		a.ui.treeView.Focus(id)
		a.ui.treeView.See(id)
	}
	a.showCommitDetails(entry, 0)
	a.scheduleGraphCanvasDraw()
}

func (a *Controller) showLocalChanges(staged bool) {
	a.cancelPendingDiffLoad()
	a.state.selection.SetLocal(staged)
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
	return state.snapshotLocked()
}

func (a *Controller) ensureLocalDiffLoading(staged bool, force bool) {
	state := a.localDiffState(staged, true)
	state.Lock()
	gen, started := state.startLoadingLocked(force)
	state.Unlock()
	if !started {
		return
	}
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
	state.resetLocked()
}

func (a *Controller) localDiffState(staged bool, create bool) *localDiffState {
	return a.state.localDiff.state(staged, create)
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
	sel := a.ui.treeView.Selection("")
	if len(sel) == 0 || sel[0] != targetID {
		return
	}
	topLine := a.diffTopLine()
	a.renderLocalChanges(staged, false)
	if topLine > 0 {
		a.scrollDiffToLine(topLine)
	}
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
	slog.Debug("scheduleDiffLoad", slog.String("hash", hash))
	a.state.diff.debounce.Trigger(diffRequest{entry: entry, hash: hash})
}

func (a *Controller) cancelPendingDiffLoad() {
	slog.Debug("cancelPendingDiffLoad")
	a.state.diff.debounce.Stop()
}

func (a *Controller) reloadCommitsAsync() {
	if a.state.tree.loadingBatch {
		return
	}
	a.state.tree.loadingBatch = true
	slog.Debug("reloadCommitsAsync start",
		slog.Uint64("batch", uint64(a.cfg.batch)),
		slog.String("filter", a.state.filter.value),
	)
	go func() {
		entries, head, hasMore, err := a.svc.ScanCommits(0, a.cfg.batch)
		PostEvent(func() {
			a.state.tree.loadingBatch = false
			if err != nil {
				slog.Error("failed to reload commits", slog.Any("error", err))
				a.setStatus(fmt.Sprintf("Failed to reload commits: %v", err))
				return
			}
			a.data.commits = entries
			a.data.visible = entries
			a.repo.headRef = head
			a.state.tree.hasMore = hasMore
			a.state.tree.rows.setCommitIDs(entries)
			a.state.tree.rows.refreshValues = true
			slog.Debug("reloadCommitsAsync loaded",
				slog.Int("count", len(entries)),
				slog.String("head", head),
				slog.Bool("has_more", hasMore),
			)
			if err := a.loadBranchLabels(); err != nil {
				slog.Error("failed to refresh branch labels", slog.Any("error", err))
			}
			a.applyFilterContent(a.state.filter.value)
			a.refreshLocalChangesAsync(true)
			a.setStatus(a.statusSummary())
		}, false)
	}()
}

func (a *Controller) loadMoreCommitsAsync(prefetch bool) {
	if a.state.tree.loadingBatch || (!prefetch && !a.state.tree.hasMore) {
		return
	}
	a.state.tree.loadingBatch = true
	skip := len(a.data.commits)
	slog.Debug("loadMoreCommitsAsync start",
		slog.Int("skip", skip),
		slog.Bool("prefetch", prefetch),
		slog.String("filter", a.state.filter.value),
	)
	go func(skipCount uint, background bool) {
		entries, _, hasMore, err := a.svc.ScanCommits(skipCount, a.cfg.batch)
		PostEvent(func() {
			a.state.tree.loadingBatch = false
			if err != nil {
				slog.Error("failed to load more commits", slog.Any("error", err))
				if !background {
					a.setStatus(fmt.Sprintf("Failed to load more commits: %v", err))
				}
				return
			}
			if len(entries) == 0 {
				a.state.tree.hasMore = false
				if !background {
					a.setStatus("No more commits available.")
				}
				return
			}
			a.data.commits = append(a.data.commits, entries...)
			a.state.tree.hasMore = hasMore
			a.state.tree.rows.addCommitIDs(entries)
			a.state.tree.rows.refreshValues = true
			slog.Debug("loadMoreCommitsAsync loaded",
				slog.Int("added", len(entries)),
				slog.Int("total", len(a.data.commits)),
				slog.Bool("has_more", hasMore),
				slog.Bool("background", background),
			)
			if err := a.loadBranchLabels(); err != nil {
				slog.Error("failed to refresh branch labels", slog.Any("error", err))
			}
			a.applyFilterContent(a.state.filter.value)
			a.refreshLocalChangesAsync(false)
			a.setStatus(a.statusSummary())
			if background && a.state.tree.hasMore {
				go a.loadMoreCommitsAsync(true)
			}
		}, false)
	}(uint(skip), prefetch)
}

func (a *Controller) clearDetailText(msg string) {
	a.writeDetailText(msg, false)
	a.setFileSections(nil)
}

func (a *Controller) writeDetailText(content string, highlightDiff bool) {
	a.ui.diffDetail.Configure(State(NORMAL))
	a.ui.diffDetail.Delete("1.0", END)
	a.ui.diffDetail.Insert("1.0", content)
	if highlightDiff {
		a.highlightDiffLines(content)
	} else {
		a.ui.diffDetail.TagRemove("diffAdd", "1.0", END)
		a.ui.diffDetail.TagRemove("diffDel", "1.0", END)
		a.ui.diffDetail.TagRemove("diffHeader", "1.0", END)
	}
	a.maybeStartSyntaxHighlight(content, highlightDiff)
	a.ui.diffDetail.Configure(State("disabled"))
}

func (a *Controller) highlightDiffLines(content string) {
	a.ui.diffDetail.TagRemove("diffAdd", "1.0", END)
	a.ui.diffDetail.TagRemove("diffDel", "1.0", END)
	a.ui.diffDetail.TagRemove("diffHeader", "1.0", END)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if len(line) == 0 {
			continue
		}
		tag := diffLineTag(line)
		if tag == "" {
			continue
		}
		lineNo := i + 1
		start := fmt.Sprintf("%d.0", lineNo)
		end := fmt.Sprintf("%d.0", lineNo+1)
		if lineNo == len(lines) {
			end = fmt.Sprintf("%d.end", lineNo)
		}
		a.ui.diffDetail.TagAdd(tag, start, end)
	}
}

func (a *Controller) copyDetailSelection(stripMarkers bool) {
	ranges := a.ui.diffDetail.TagRanges("sel")
	if len(ranges) < 2 {
		return
	}
	text := a.ui.diffDetail.Get(ranges[0], ranges[1])[0]
	if text == "" {
		return
	}
	if stripMarkers {
		lines := strings.Split(text, "\n")
		filtered := make([]string, 0, len(lines))
		for _, line := range lines {
			if len(line) > 0 && (line[0] == '+' || line[0] == '-') {
				line = line[1:]
			}
			filtered = append(filtered, line)
		}
		text = strings.Join(filtered, "\n")
	}
	if text == "" {
		return
	}
	ClipboardClear()
	ClipboardAppend(text)
	if stripMarkers {
		a.setStatus("Copied selection without +/- markers.")
	} else {
		a.setStatus("Copied selection.")
	}
}

func (a *Controller) setFileSections(sections []git.FileSection) {
	model := newDiffViewModel(sections)
	a.state.diff.fileSections = model.sections
	a.state.diff.selectedFileIndex = -1
	a.ui.diffFileList.Configure(State("normal"))
	a.ui.diffFileList.Delete("1.0", END)
	for i, row := range model.rows {
		if i > 0 {
			a.ui.diffFileList.Insert(END, "\n")
		}
		lineNo := i + 1
		a.ui.diffFileList.Insert(END, row.label)
		a.applyDiffFileRowTags(lineNo, row)
	}
	a.ui.diffFileList.Configure(State("disabled"))
	a.syncFileSelectionToDiff()
}

func (a *Controller) applyDiffFileRowTags(lineNo int, row diffViewRow) {
	a.ui.diffFileList.TagAdd(
		"diffFileAddCount",
		fmt.Sprintf("%d.%d", lineNo, row.addStart),
		fmt.Sprintf("%d.%d", lineNo, row.addEnd),
	)
	a.ui.diffFileList.TagAdd(
		"diffFileDelCount",
		fmt.Sprintf("%d.%d", lineNo, row.delStart),
		fmt.Sprintf("%d.%d", lineNo, row.delEnd),
	)
}

func (a *Controller) onFileSelectionChanged(e *Event) {
	if a.state.diff.suppressFileSelection {
		return
	}
	if len(a.state.diff.fileSections) == 0 {
		return
	}
	idx, ok := a.diffFileListIndexAtY(e)
	if !ok {
		return
	}
	a.setFileListSelection(idx)
	line, ok := diffSectionLine(a.state.diff.fileSections, idx)
	if !ok {
		return
	}
	a.state.diff.skipNextSync = true
	a.scrollDiffToLine(line)
}

func (a *Controller) scrollDiffToLine(line int) {
	if line <= 0 {
		return
	}
	totalLines := a.textLineCount()
	a.ui.diffDetail.Yviewmoveto(diffScrollFraction(line, totalLines))
}

func (a *Controller) textLineCount() int {
	index := a.ui.diffDetail.Index(END)
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

func (a *Controller) diffTopLine() int {
	if a.ui.diffDetail == nil {
		return 0
	}
	index := a.ui.diffDetail.Index("@0,0")
	parts := strings.SplitN(index, ".", 2)
	if len(parts) == 0 {
		return 0
	}
	line, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0
	}
	return line
}

func (a *Controller) currentSelection() string {
	return a.state.selection.CommitHash()
}

func (a *Controller) setStatus(msg string) {
	text := msg
	PostEvent(func() {
		a.ui.status.Configure(Txt(text))
	}, false)
}

func (a *Controller) statusSummary() string {
	total := len(a.data.commits)
	visible := len(a.data.visible)
	head := a.repo.headRef
	if head == "" {
		head = "HEAD"
	}
	filterDesc := strings.TrimSpace(a.state.filter.value)
	path := a.repo.path
	if path == "" && a.svc != nil {
		path = a.svc.RepoPath()
	}
	base := fmt.Sprintf("Showing %d/%d loaded commits on %s — %s", visible, total, head, path)
	if a.state.tree.hasMore {
		base += " (more available)"
	}
	if filterDesc == "" {
		return base
	}
	return fmt.Sprintf("Filter %q — %s", filterDesc, base)
}

func (a *Controller) syncFileSelectionToDiff() {
	if len(a.state.diff.fileSections) == 0 {
		return
	}
	if a.state.diff.skipNextSync {
		return
	}
	line := a.diffTopLine()
	if line <= 0 {
		return
	}
	idx, ok := diffSectionIndexForLine(a.state.diff.fileSections, line)
	if !ok {
		return
	}
	a.setFileListSelection(idx)
}

func (a *Controller) setFileListSelection(idx int) {
	if idx < 0 || idx >= len(a.state.diff.fileSections) {
		return
	}
	if a.state.diff.selectedFileIndex == idx {
		return
	}
	a.state.diff.suppressFileSelection = true
	a.ui.diffFileList.TagRemove("diffFileSelected", "1.0", END)
	line := idx + 1
	a.ui.diffFileList.TagAdd(
		"diffFileSelected",
		fmt.Sprintf("%d.0", line),
		fmt.Sprintf("%d.end", line),
	)
	a.ui.diffFileList.See(fmt.Sprintf("%d.0", line))
	a.state.diff.selectedFileIndex = idx
	PostEvent(func() {
		a.state.diff.suppressFileSelection = false
	}, false)
}

func (a *Controller) diffFileListIndexAtY(e *Event) (int, bool) {
	if e == nil || a.ui.diffFileList == nil {
		return 0, false
	}
	index := a.ui.diffFileList.Index(fmt.Sprintf("@0,%d", e.Y))
	parts := strings.SplitN(index, ".", 2)
	if len(parts) == 0 {
		return 0, false
	}
	line, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, false
	}
	idx := line - 1
	if idx < 0 || idx >= len(a.state.diff.fileSections) {
		return 0, false
	}
	return idx, true
}

func (a *Controller) onDiffScrolled() {
	if a.state.diff.skipNextSync {
		a.state.diff.skipNextSync = false
		return
	}
	a.syncFileSelectionToDiff()
}
