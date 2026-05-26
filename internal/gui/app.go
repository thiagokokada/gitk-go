package gui

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/thiagokokada/gitk-go/internal/git"
	"github.com/thiagokokada/gitk-go/internal/gui/model"
	"github.com/thiagokokada/gitk-go/internal/gui/view"

	. "modernc.org/tk9.0"
	_ "modernc.org/tk9.0/themes/azure" // load theme
)

const (
	autoLoadThreshold   = 0.98
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
		model: model.NewApp(svc.RepoPath()),
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
	a.runtime.actions.diffDebounce.Configure(diffDebounceDelay, func(req model.DiffRequest) {
		if req.Entry == nil {
			return
		}
		go a.populateDiff(req.Entry, req.Hash)
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
	a.model.State.Tree.BranchLabels = labels
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
	actions := a.model.State.Tree.LocalChangePlan(repoReady, prefetch, status)
	a.setLocalRowVisibility(false, actions.ShowUnstaged)
	a.setLocalRowVisibility(true, actions.ShowStaged)
	if actions.ResetUnstaged {
		a.model.State.LocalDiff.Reset(false)
	}
	if actions.ResetStaged {
		a.model.State.LocalDiff.Reset(true)
	}
	if actions.LoadUnstaged {
		a.ensureLocalDiffLoading(false, true)
	}
	if actions.LoadStaged {
		a.ensureLocalDiffLoading(true, true)
	}
}

func (a *Controller) showCommitDetails(entry *git.Entry, index int) {
	header := git.FormatCommitHeader(entry.Commit)
	hash := entry.Commit.Hash
	a.model.State.Selection.SetCommit(entry, index)
	a.showDiffStatus(header, "Loading diff...")
	a.scheduleDiffLoad(entry, hash)
}

func (a *Controller) selectFallbackCommit() {
	plan := a.model.FallbackSelectionPlan()
	switch plan.Kind {
	case model.SelectionDisplayMessage:
		a.clearDetailText(plan.Message)
		a.setStatus(a.statusSummary())
		return
	case model.SelectionDisplayCommit:
		a.selectCommitPlan(plan)
		a.scheduleGraphCanvasDraw()
	default:
		return
	}
}

func (a *Controller) showLocalChanges(staged bool) {
	a.cancelPendingDiffLoad()
	a.model.State.Selection.SetLocal(staged)
	a.renderLocalChanges(staged, true)
}

func (a *Controller) renderLocalChanges(staged bool, requestReload bool) {
	header := localUnstagedLabel
	if staged {
		header = localStagedLabel
	}
	snap := a.model.State.LocalDiff.Snapshot(staged)
	if requestReload && snap.Ready {
		a.presentLocalDiff(header, snap)
		a.ensureLocalDiffLoading(staged, true)
		return
	}
	if requestReload {
		a.ensureLocalDiffLoading(staged, true)
		snap = a.model.State.LocalDiff.Snapshot(staged)
	} else if !snap.Ready && !snap.Loading {
		a.ensureLocalDiffLoading(staged, false)
		snap = a.model.State.LocalDiff.Snapshot(staged)
	}
	a.presentLocalDiff(header, snap)
}

func (a *Controller) presentLocalDiff(header string, snap model.LocalDiffSnapshot) {
	if !snap.Ready {
		a.showDiffStatus(header, "Loading local changes...")
		return
	}
	if snap.Err != nil {
		a.showDiffStatus(header, fmt.Sprintf("Unable to compute diff: %v", snap.Err))
		return
	}
	diff := snap.Diff
	if strings.TrimSpace(diff) == "" {
		a.showDiffStatus(header, "No changes.")
		return
	}
	a.showRenderedDiff(diff, snap.Sections)
}

func (a *Controller) ensureLocalDiffLoading(staged bool, force bool) {
	gen, started := a.model.State.LocalDiff.BeginLoading(staged, force)
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
	if !a.model.State.LocalDiff.CompleteLoading(staged, gen, diff, sections, err) {
		return
	}
	PostEvent(func() {
		a.onLocalDiffLoaded(staged)
	}, false)
}

func (a *Controller) onLocalDiffLoaded(staged bool) {
	snap := a.model.State.LocalDiff.Snapshot(staged)
	if snap.Err == nil {
		if strings.TrimSpace(snap.Diff) == "" {
			a.setLocalRowVisibility(staged, false)
		} else {
			a.setLocalRowVisibility(staged, true)
		}
	}
	targetID := model.LocalRowID(staged)
	if a.ui.SelectedTreeRow() != targetID {
		return
	}
	topLine := a.ui.DiffTopLine()
	a.renderLocalChanges(staged, false)
	if topLine > 0 {
		a.ui.ScrollDiffToLine(topLine)
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
	a.runtime.actions.diffDebounce.Trigger(model.DiffRequest{Entry: entry, Hash: hash})
}

func (a *Controller) cancelPendingDiffLoad() {
	slog.Debug("cancelPendingDiffLoad")
	a.runtime.actions.diffDebounce.Stop()
}

func (a *Controller) refreshCommitBatchState(prefetchLocalChanges bool) {
	if err := a.loadBranchLabels(); err != nil {
		slog.Error("failed to refresh branch labels", slog.Any("error", err))
	}
	a.applyFilterContent(a.model.State.Filter.Value)
	a.refreshLocalChangesAsync(prefetchLocalChanges)
	a.setStatus(a.statusSummary())
}

func (a *Controller) applyReloadedCommitBatch(entries []*git.Entry, head string, hasMore bool) {
	a.model.SetReloadedCommits(entries, head, hasMore)
	slog.Debug("reloadCommitsAsync loaded",
		slog.Int("count", len(entries)),
		slog.String("head", head),
		slog.Bool("has_more", hasMore),
	)
	a.refreshCommitBatchState(true)
}

func (a *Controller) applyAppendedCommitBatch(entries []*git.Entry, hasMore bool, background bool) {
	if len(entries) == 0 {
		a.model.State.Tree.MarkNoMoreCommits()
		if !background {
			a.setStatus("No more commits available.")
		}
		return
	}
	a.model.AppendCommits(entries, hasMore)
	slog.Debug("loadMoreCommitsAsync loaded",
		slog.Int("added", len(entries)),
		slog.Int("total", len(a.model.Data.Commits)),
		slog.Bool("has_more", hasMore),
		slog.Bool("background", background),
	)
	a.refreshCommitBatchState(false)
	if background && a.model.State.Tree.HasMore {
		go a.loadMoreCommitsAsync(true)
	}
}

func (a *Controller) reloadCommitsAsync() {
	if !a.model.State.Tree.BeginCommitBatchLoad(true) {
		return
	}
	slog.Debug("reloadCommitsAsync start",
		slog.Uint64("batch", uint64(a.cfg.batch)),
		slog.String("filter", a.model.State.Filter.Value),
	)
	go func() {
		entries, head, hasMore, err := a.svc.ScanCommits(0, a.cfg.batch)
		PostEvent(func() {
			a.model.State.Tree.FinishCommitBatchLoad()
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
	if !a.model.State.Tree.BeginCommitBatchLoad(prefetch) {
		return
	}
	skip := len(a.model.Data.Commits)
	slog.Debug("loadMoreCommitsAsync start",
		slog.Int("skip", skip),
		slog.Bool("prefetch", prefetch),
		slog.String("filter", a.model.State.Filter.Value),
	)
	go func(skipCount uint, background bool) {
		entries, _, hasMore, err := a.svc.ScanCommits(skipCount, a.cfg.batch)
		PostEvent(func() {
			a.model.State.Tree.FinishCommitBatchLoad()
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
	a.ui.WriteDetailText(content, highlightDiff, diffLineTag)
	a.maybeStartSyntaxHighlight(content, highlightDiff)
}

func (a *Controller) copyDetailSelection(stripMarkers bool) {
	text := a.ui.SelectedDiffText()
	if text == "" {
		return
	}
	if stripMarkers {
		text = view.StripDiffLineMarkers(text)
	}
	if text == "" {
		return
	}
	a.ui.CopyToClipboard(text)
	if stripMarkers {
		a.setStatus("Copied selection without +/- markers.")
	} else {
		a.setStatus("Copied selection.")
	}
}

func (a *Controller) currentSelection() string {
	return a.model.State.Selection.CommitHash()
}

func (a *Controller) setStatus(msg string) {
	a.ui.SetStatus(msg)
}

func (a *Controller) statusSummary() string {
	total := len(a.model.Data.Commits)
	visible := len(a.model.Data.Visible)
	head := a.model.Repo.HeadRef
	if head == "" {
		head = "HEAD"
	}
	filterDesc := strings.TrimSpace(a.model.State.Filter.Value)
	path := a.model.Repo.Path
	if path == "" && a.svc != nil {
		path = a.svc.RepoPath()
	}
	base := fmt.Sprintf("Showing %d/%d loaded commits on %s — %s", visible, total, head, path)
	if a.model.State.Tree.HasMore {
		base += " (more available)"
	}
	if filterDesc == "" {
		return base
	}
	return fmt.Sprintf("Filter %q — %s", filterDesc, base)
}

func (a *Controller) onDiffScrolled() {
	if a.model.State.Diff.ConsumeSkipNextSync() {
		return
	}
	a.syncFileSelectionToDiff()
}
