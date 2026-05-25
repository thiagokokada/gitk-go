package gui

import (
	"fmt"
	"log/slog"
	"os"
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
	app, err := NewController(cfg)
	if err != nil {
		return err
	}
	return app.Run()
}

func NewController(cfg RunConfig) (*Controller, error) {
	if cfg.RepoPath == "" {
		cfg.RepoPath = "."
	}
	svc, err := git.Open(cfg.RepoPath)
	if err != nil {
		return nil, err
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
		theme: controllerTheme{
			pref:     pref,
			activate: cfg.ThemeActivator,
		},
		model: newAppModel(svc.RepoPath()),
	}
	app.configureDebouncedActions()
	return app, nil
}

func (a *Controller) configureDebouncedActions() {
	a.runtime.actions.filterDebounce.Configure(filterDebounceDelay, func(value string) {
		if value == "" {
			return
		}
		PostEvent(func() {
			a.applyFilter(value)
		}, false)
	})
	a.runtime.actions.diffDebounce.Configure(diffDebounceDelay, func(req diffRequest) {
		if req.entry == nil {
			return
		}
		go a.populateDiff(req.entry, req.hash)
	})
}

func (a *Controller) Run() error {
	defer a.shutdown()

	a.configureLogging()
	if err := a.initializeTkRuntime(); err != nil {
		return err
	}
	a.buildUI()
	a.startRuntimeServices()
	a.startInitialDataLoads()

	App.WmTitle("gitk-go")
	App.SetResizable(true, true)
	App.Center().Wait()

	return nil
}

func (a *Controller) configureLogging() {
	level := slog.LevelInfo
	if a.cfg.verbose {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
}

func (a *Controller) initializeTkRuntime() error {
	if err := InitializeExtension("eval"); err != nil && err != AlreadyInitialized {
		return fmt.Errorf("init eval extension: %v", err)
	}
	a.applyThemePalette(paletteForPreference(a.theme.pref))
	applyAppIcon()
	return nil
}

func (a *Controller) startRuntimeServices() {
	a.loadPreferences()
	a.startThemeWatch()
	a.initAutoReload(a.cfg.autoReloadRequested)
}

func (a *Controller) startInitialDataLoads() {
	a.showInitialLoadingRow()
	a.setStatus("Loading commits...")
	a.refreshLocalChangesAsync(true)
	a.reloadCommitsAsync()
}

func (a *Controller) loadBranchLabels() error {
	labels, err := a.svc.BranchLabels()
	if err != nil {
		return err
	}
	a.model.state.tree.branchLabels = labels
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
	actions := a.model.state.tree.localChangePlan(repoReady, prefetch, status)
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
	a.model.state.selection.SetCommit(entry, index)
	a.showDiffStatus(header, "Loading diff...")
	a.scheduleDiffLoad(entry, hash)
}

func (a *Controller) selectFallbackCommit() {
	plan := a.model.fallbackSelectionPlan()
	switch plan.kind {
	case selectionDisplayMessage:
		a.clearDetailText(plan.message)
		a.setStatus(a.statusSummary())
		return
	case selectionDisplayCommit:
		a.selectCommitPlan(plan)
		a.scheduleGraphCanvasDraw()
	default:
		return
	}
}

func (a *Controller) showLocalChanges(staged bool) {
	a.cancelPendingDiffLoad()
	a.model.state.selection.SetLocal(staged)
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
		a.showDiffStatus(header, "Loading local changes...")
		return
	}
	if snap.err != nil {
		a.showDiffStatus(header, fmt.Sprintf("Unable to compute diff: %v", snap.err))
		return
	}
	diff := snap.diff
	if strings.TrimSpace(diff) == "" {
		a.showDiffStatus(header, "No changes.")
		return
	}
	a.showRenderedDiff(diff, snap.sections)
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
	return a.model.state.localDiff.state(staged, create)
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
	PostEvent(func() {
		if a.currentSelection() != hash {
			return
		}
		a.showRenderedDiff(diff, sections)
	}, false)
}

func (a *Controller) scheduleDiffLoad(entry *git.Entry, hash string) {
	if entry == nil {
		return
	}
	slog.Debug("scheduleDiffLoad", slog.String("hash", hash))
	a.runtime.actions.diffDebounce.Trigger(diffRequest{entry: entry, hash: hash})
}

func (a *Controller) cancelPendingDiffLoad() {
	slog.Debug("cancelPendingDiffLoad")
	a.runtime.actions.diffDebounce.Stop()
}

func (a *Controller) refreshCommitBatchState(prefetchLocalChanges bool) {
	if err := a.loadBranchLabels(); err != nil {
		slog.Error("failed to refresh branch labels", slog.Any("error", err))
	}
	a.applyFilterContent(a.model.state.filter.value)
	a.refreshLocalChangesAsync(prefetchLocalChanges)
	a.setStatus(a.statusSummary())
}

func (a *Controller) applyReloadedCommitBatch(entries []*git.Entry, head string, hasMore bool) {
	a.model.setReloadedCommits(entries, head, hasMore)
	slog.Debug("reloadCommitsAsync loaded",
		slog.Int("count", len(entries)),
		slog.String("head", head),
		slog.Bool("has_more", hasMore),
	)
	a.refreshCommitBatchState(true)
}

func (a *Controller) applyAppendedCommitBatch(entries []*git.Entry, hasMore bool, background bool) {
	if len(entries) == 0 {
		a.model.state.tree.markNoMoreCommits()
		if !background {
			a.setStatus("No more commits available.")
		}
		return
	}
	a.model.appendCommits(entries, hasMore)
	slog.Debug("loadMoreCommitsAsync loaded",
		slog.Int("added", len(entries)),
		slog.Int("total", len(a.model.data.commits)),
		slog.Bool("has_more", hasMore),
		slog.Bool("background", background),
	)
	a.refreshCommitBatchState(false)
	if background && a.model.state.tree.hasMore {
		go a.loadMoreCommitsAsync(true)
	}
}

func (a *Controller) reloadCommitsAsync() {
	if !a.model.state.tree.beginCommitBatchLoad(true) {
		return
	}
	slog.Debug("reloadCommitsAsync start",
		slog.Uint64("batch", uint64(a.cfg.batch)),
		slog.String("filter", a.model.state.filter.value),
	)
	go func() {
		entries, head, hasMore, err := a.svc.ScanCommits(0, a.cfg.batch)
		PostEvent(func() {
			a.model.state.tree.finishCommitBatchLoad()
			if err != nil {
				slog.Error("failed to reload commits", slog.Any("error", err))
				a.setStatus(fmt.Sprintf("Failed to reload commits: %v", err))
				return
			}
			a.applyReloadedCommitBatch(entries, head, hasMore)
		}, false)
	}()
}

func (a *Controller) loadMoreCommitsAsync(prefetch bool) {
	if !a.model.state.tree.beginCommitBatchLoad(prefetch) {
		return
	}
	skip := len(a.model.data.commits)
	slog.Debug("loadMoreCommitsAsync start",
		slog.Int("skip", skip),
		slog.Bool("prefetch", prefetch),
		slog.String("filter", a.model.state.filter.value),
	)
	go func(skipCount uint, background bool) {
		entries, _, hasMore, err := a.svc.ScanCommits(skipCount, a.cfg.batch)
		PostEvent(func() {
			a.model.state.tree.finishCommitBatchLoad()
			if err != nil {
				slog.Error("failed to load more commits", slog.Any("error", err))
				if !background {
					a.setStatus(fmt.Sprintf("Failed to load more commits: %v", err))
				}
				return
			}
			a.applyAppendedCommitBatch(entries, hasMore, background)
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

func (a *Controller) scrollDiffToLine(line int) {
	if line <= 0 {
		return
	}
	totalLines := a.textLineCount()
	a.ui.diffDetail.Yviewmoveto(diffScrollFraction(line, totalLines))
}

func (a *Controller) textLineCount() int {
	lines, ok := textIndexLineNumber(a.ui.diffDetail.Index(END))
	if !ok {
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
	line, ok := textIndexLineNumber(a.ui.diffDetail.Index("@0,0"))
	if !ok {
		return 0
	}
	return line
}

func (a *Controller) currentSelection() string {
	return a.model.state.selection.CommitHash()
}

func (a *Controller) setStatus(msg string) {
	text := msg
	PostEvent(func() {
		a.ui.status.Configure(Txt(text))
	}, false)
}

func (a *Controller) statusSummary() string {
	total := len(a.model.data.commits)
	visible := len(a.model.data.visible)
	head := a.model.repo.headRef
	if head == "" {
		head = "HEAD"
	}
	filterDesc := strings.TrimSpace(a.model.state.filter.value)
	path := a.model.repo.path
	if path == "" && a.svc != nil {
		path = a.svc.RepoPath()
	}
	base := fmt.Sprintf("Showing %d/%d loaded commits on %s — %s", visible, total, head, path)
	if a.model.state.tree.hasMore {
		base += " (more available)"
	}
	if filterDesc == "" {
		return base
	}
	return fmt.Sprintf("Filter %q — %s", filterDesc, base)
}

func (a *Controller) onDiffScrolled() {
	if a.model.state.diff.consumeSkipNextSync() {
		return
	}
	a.syncFileSelectionToDiff()
}
