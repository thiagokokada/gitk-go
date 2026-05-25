package gui

import (
	"errors"
	"testing"

	"github.com/thiagokokada/gitk-go/internal/git"
)

func TestNewDiffStateInitializesSyntaxTags(t *testing.T) {
	state := newDiffState()
	if state.syntaxTags == nil {
		t.Fatalf("expected syntax tags map")
	}
}

func TestDiffStateSetFileSectionsResetsSelection(t *testing.T) {
	state := newDiffState()
	state.selectedFileIndex = 2

	state.setFileSections([]git.FileSection{{Path: "a.go", Line: 10}})

	if state.selectedFileIndex != -1 {
		t.Fatalf("selected index = %d, want -1", state.selectedFileIndex)
	}
	if len(state.fileSections) != 1 || state.fileSections[0].Path != "a.go" {
		t.Fatalf("unexpected file sections: %+v", state.fileSections)
	}
}

func TestDiffStateBeginUserFileSelection(t *testing.T) {
	state := newDiffState()
	state.setFileSections(newDiffViewModel([]git.FileSection{{Path: "a.go", Line: 7}}).sections)

	line, ok := state.beginUserFileSelection(1)
	if !ok {
		t.Fatalf("expected file selection")
	}
	if line != 7 {
		t.Fatalf("line = %d, want 7", line)
	}
	if !state.skipNextSync {
		t.Fatalf("expected skip next sync")
	}

	if _, ok := state.beginUserFileSelection(-1); ok {
		t.Fatalf("expected invalid selection to fail")
	}
	state.suppressFileSelection = true
	if _, ok := state.beginUserFileSelection(1); ok {
		t.Fatalf("expected suppressed selection to fail")
	}
}

func TestDiffStateSelectFileIndex(t *testing.T) {
	state := newDiffState()
	state.setFileSections(newDiffViewModel([]git.FileSection{{Path: "a.go", Line: 7}}).sections)

	if !state.selectFileIndex(1) {
		t.Fatalf("expected selection to change")
	}
	if state.selectedFileIndex != 1 {
		t.Fatalf("selected index = %d, want 1", state.selectedFileIndex)
	}
	if !state.suppressFileSelection {
		t.Fatalf("expected suppress flag")
	}
	if state.selectFileIndex(1) {
		t.Fatalf("expected unchanged selection")
	}

	state.finishProgrammaticFileSelection()
	if state.suppressFileSelection {
		t.Fatalf("expected suppress flag cleared")
	}
}

func TestDiffStateSyncSelectionIndexForLine(t *testing.T) {
	state := newDiffState()
	state.setFileSections(newDiffViewModel([]git.FileSection{
		{Path: "a.go", Line: 7},
		{Path: "b.go", Line: 20},
	}).sections)

	idx, ok := state.syncSelectionIndexForLine(21)
	if !ok {
		t.Fatalf("expected index for line")
	}
	if idx != 2 {
		t.Fatalf("index = %d, want 2", idx)
	}
	if _, ok := state.syncSelectionIndexForLine(0); ok {
		t.Fatalf("expected invalid line")
	}
}

func TestDiffStateSelectedFilePath(t *testing.T) {
	state := newDiffState()
	state.setFileSections(newDiffViewModel([]git.FileSection{{Path: "a.go", Line: 7}}).sections)
	if _, ok := state.selectedFilePath(); ok {
		t.Fatalf("expected no path without selection")
	}
	state.selectedFileIndex = 1
	path, ok := state.selectedFilePath()
	if !ok || path != "a.go" {
		t.Fatalf("path = %q ok=%v, want a.go true", path, ok)
	}
}

func TestDiffStateConsumeSkipNextSync(t *testing.T) {
	state := newDiffState()
	if state.consumeSkipNextSync() {
		t.Fatalf("expected no skip")
	}
	state.skipNextSync = true
	if !state.consumeSkipNextSync() {
		t.Fatalf("expected skip")
	}
	if state.skipNextSync {
		t.Fatalf("expected skip flag cleared")
	}
}

func TestNewAppModelInitializesStateContainers(t *testing.T) {
	model := newAppModel("/repo/path")
	if model.repo.path != "/repo/path" {
		t.Fatalf("repo path = %q, want %q", model.repo.path, "/repo/path")
	}
	if model.state.diff.syntaxTags == nil {
		t.Fatalf("expected diff syntax tags")
	}
	if model.state.tree.branchLabels == nil {
		t.Fatalf("expected branch labels map")
	}
	if model.state.localDiff.items == nil {
		t.Fatalf("expected local diff cache map")
	}
}

func TestAppModelResetRepositoryClearsFilter(t *testing.T) {
	model := newAppModel("/old")
	model.repo.headRef = "main"
	model.data.commits = []*git.Entry{{}}
	model.data.visible = model.data.commits
	model.state.filter.value = "feature"
	model.state.tree.hasMore = true
	model.state.localDiff.state(false, true)

	model.resetRepository("/new")

	if model.repo.path != "/new" {
		t.Fatalf("repo path = %q, want %q", model.repo.path, "/new")
	}
	if model.repo.headRef != "" {
		t.Fatalf("expected head ref reset, got %q", model.repo.headRef)
	}
	if len(model.data.commits) != 0 || len(model.data.visible) != 0 {
		t.Fatalf("expected commit data reset")
	}
	if model.state.filter.value != "" {
		t.Fatalf("expected filter cleared, got %q", model.state.filter.value)
	}
	if model.state.tree.hasMore {
		t.Fatalf("expected tree state reset")
	}
	if model.state.diff.syntaxTags == nil {
		t.Fatalf("expected reset diff syntax tags")
	}
}

func TestAppModelResetBranchPreservesFilter(t *testing.T) {
	model := newAppModel("/repo")
	model.repo.headRef = "main"
	model.data.commits = []*git.Entry{{}}
	model.data.visible = model.data.commits
	model.state.filter.value = "feature"
	model.state.tree.hasMore = true

	model.resetBranch()

	if model.repo.path != "/repo" {
		t.Fatalf("repo path = %q, want %q", model.repo.path, "/repo")
	}
	if model.repo.headRef != "" {
		t.Fatalf("expected head ref reset, got %q", model.repo.headRef)
	}
	if len(model.data.commits) != 0 || len(model.data.visible) != 0 {
		t.Fatalf("expected commit data reset")
	}
	if model.state.filter.value != "feature" {
		t.Fatalf("expected filter preserved, got %q", model.state.filter.value)
	}
	if model.state.tree.hasMore {
		t.Fatalf("expected tree state reset")
	}
}

func TestAppModelApplyFilterUpdatesVisibleRows(t *testing.T) {
	h1 := "1111111111111111111111111111111111111111"
	h2 := "2222222222222222222222222222222222222222"
	model := newAppModel("/repo")
	model.data.commits = []*git.Entry{
		{Commit: &git.Commit{Hash: h1}, SearchText: "feature branch"},
		{Commit: &git.Commit{Hash: h2}, SearchText: "bug fix"},
	}

	model.applyFilter("FEATURE")

	if model.state.filter.value != "FEATURE" {
		t.Fatalf("filter value = %q, want %q", model.state.filter.value, "FEATURE")
	}
	if len(model.data.visible) != 1 || model.data.visible[0].Commit.Hash != h1 {
		t.Fatalf("unexpected visible commits: %+v", model.data.visible)
	}
	if got, ok := model.state.tree.rows.visibleByID[h1]; !ok || got != 0 {
		t.Fatalf("visible index for h1 = %d, want 0", got)
	}
	if _, ok := model.state.tree.rows.visibleByID[h2]; ok {
		t.Fatalf("filtered commit should not be indexed")
	}
}

func TestAppModelSetReloadedCommitsResetsBatchState(t *testing.T) {
	h1 := "1111111111111111111111111111111111111111"
	entries := []*git.Entry{{Commit: &git.Commit{Hash: h1}}}
	model := newAppModel("/repo")

	model.setReloadedCommits(entries, "main", true)

	if model.repo.headRef != "main" {
		t.Fatalf("head ref = %q, want %q", model.repo.headRef, "main")
	}
	if len(model.data.commits) != 1 || model.data.commits[0] != entries[0] {
		t.Fatalf("unexpected commits: %+v", model.data.commits)
	}
	if len(model.data.visible) != 1 || model.data.visible[0] != entries[0] {
		t.Fatalf("unexpected visible commits: %+v", model.data.visible)
	}
	if !model.state.tree.hasMore {
		t.Fatalf("expected hasMore set")
	}
	if _, ok := model.state.tree.rows.commitIDs[h1]; !ok {
		t.Fatalf("expected commit id indexed")
	}
	if !model.state.tree.rows.refreshValues {
		t.Fatalf("expected tree row refresh")
	}
}

func TestAppModelAppendCommitsExtendsBatchState(t *testing.T) {
	h1 := "1111111111111111111111111111111111111111"
	h2 := "2222222222222222222222222222222222222222"
	model := newAppModel("/repo")
	model.setReloadedCommits([]*git.Entry{{Commit: &git.Commit{Hash: h1}}}, "main", true)
	model.state.tree.rows.refreshValues = false

	model.appendCommits([]*git.Entry{{Commit: &git.Commit{Hash: h2}}}, false)

	if len(model.data.commits) != 2 {
		t.Fatalf("commit count = %d, want 2", len(model.data.commits))
	}
	if model.state.tree.hasMore {
		t.Fatalf("expected hasMore cleared")
	}
	if _, ok := model.state.tree.rows.commitIDs[h1]; !ok {
		t.Fatalf("expected existing commit id retained")
	}
	if _, ok := model.state.tree.rows.commitIDs[h2]; !ok {
		t.Fatalf("expected appended commit id indexed")
	}
	if !model.state.tree.rows.refreshValues {
		t.Fatalf("expected tree row refresh")
	}
}

func TestAppModelFallbackSelectionPlan(t *testing.T) {
	model := newAppModel("/repo")
	plan := model.fallbackSelectionPlan()
	if plan.kind != selectionDisplayMessage || plan.message != "Repository has no commits yet." {
		t.Fatalf("unexpected empty repo plan: %+v", plan)
	}

	model.data.commits = []*git.Entry{{SearchText: "hidden"}}
	plan = model.fallbackSelectionPlan()
	if plan.kind != selectionDisplayMessage || plan.message != "No commits match the current filter." {
		t.Fatalf("unexpected filtered repo plan: %+v", plan)
	}

	entry := &git.Entry{Commit: &git.Commit{Hash: "1111111111111111111111111111111111111111"}}
	model.data.visible = []*git.Entry{entry}
	plan = model.fallbackSelectionPlan()
	if plan.kind != selectionDisplayCommit || plan.entry != entry || plan.index != 0 || !plan.loadDetail {
		t.Fatalf("unexpected commit fallback plan: %+v", plan)
	}
}

func TestAppModelFilterSelectionPlanPreservesVisibleLocalRow(t *testing.T) {
	model := newAppModel("/repo")
	model.state.selection.SetLocal(true)
	model.state.tree.rows.addSpecialItem(localStagedRowID)

	plan := model.filterSelectionPlan()

	if plan.kind != selectionDisplayLocal || !plan.staged {
		t.Fatalf("unexpected local plan: %+v", plan)
	}
}

func TestAppModelFilterSelectionPlanChoosesCommit(t *testing.T) {
	h1 := "1111111111111111111111111111111111111111"
	h2 := "2222222222222222222222222222222222222222"
	first := &git.Entry{Commit: &git.Commit{Hash: h1}}
	second := &git.Entry{Commit: &git.Commit{Hash: h2}}
	model := newAppModel("/repo")
	model.data.commits = []*git.Entry{first, second}
	model.data.visible = model.data.commits
	model.state.tree.rows.setVisibleIndex(model.data.visible)
	model.state.selection.SetCommit(second, 1)

	plan := model.filterSelectionPlan()

	if plan.kind != selectionDisplayCommit || plan.entry != second || plan.index != 1 || plan.loadDetail {
		t.Fatalf("unexpected existing commit plan: %+v", plan)
	}

	model.data.visible = []*git.Entry{first}
	model.state.tree.rows.setVisibleIndex(model.data.visible)
	plan = model.filterSelectionPlan()
	if plan.kind != selectionDisplayCommit || plan.entry != first || plan.index != 0 || !plan.loadDetail {
		t.Fatalf("unexpected fallback commit plan: %+v", plan)
	}
}

func TestAppModelTreeSelectionPlan(t *testing.T) {
	hash := "1111111111111111111111111111111111111111"
	entry := &git.Entry{Commit: &git.Commit{Hash: hash}}
	model := newAppModel("/repo")
	model.data.visible = []*git.Entry{entry}
	model.state.tree.rows.setVisibleIndex(model.data.visible)

	plan := model.treeSelectionPlan(hash)
	if plan.kind != treeSelectionCommit || plan.entry != entry || plan.index != 0 {
		t.Fatalf("unexpected commit tree plan: %+v", plan)
	}

	model.state.selection.SetCommit(entry, 0)
	plan = model.treeSelectionPlan(hash)
	if plan.kind != treeSelectionNone {
		t.Fatalf("expected unchanged selection to be ignored, got %+v", plan)
	}

	plan = model.treeSelectionPlan(localUnstagedRowID)
	if plan.kind != treeSelectionLocal || plan.staged {
		t.Fatalf("unexpected local tree plan: %+v", plan)
	}

	plan = model.treeSelectionPlan(loadingIndicatorID)
	if plan.kind != treeSelectionClear || model.state.selection.CommitHash() != "" {
		t.Fatalf("unexpected clear tree plan: %+v", plan)
	}
}

func TestTreeStateLocalRowVisibility(t *testing.T) {
	state := newTreeState()
	if !state.setLocalRowVisible(false, true) {
		t.Fatalf("expected unstaged row change")
	}
	if state.setLocalRowVisible(false, true) {
		t.Fatalf("expected unchanged unstaged row")
	}
	if !state.localRowVisible(false) {
		t.Fatalf("expected unstaged row visible")
	}
	if got := state.localRowIDs(); len(got) != 1 || got[0] != localUnstagedRowID {
		t.Fatalf("unexpected local row ids: %+v", got)
	}

	if !state.setLocalRowVisible(true, true) {
		t.Fatalf("expected staged row change")
	}
	if !state.localRowVisible(true) {
		t.Fatalf("expected staged row visible")
	}
	if idx := state.localRowInsertIndex(true); idx != 1 {
		t.Fatalf("staged insert index = %d, want 1", idx)
	}
}

func TestTreeStatePruneStaleCommitRows(t *testing.T) {
	keep := "1111111111111111111111111111111111111111"
	stale := "2222222222222222222222222222222222222222"
	state := newTreeState()
	state.setReloadedCommits([]*git.Entry{{Commit: &git.Commit{Hash: keep}}}, true)
	state.rows.addItem(keep)
	state.rows.setItemValue(keep, treeRow{Commit: "keep"})
	state.rows.addItem(stale)
	state.rows.setItemValue(stale, treeRow{Commit: "stale"})

	got := state.rows.pruneStaleCommitRows()

	if len(got) != 1 || got[0] != stale {
		t.Fatalf("stale row ids = %+v, want %q", got, stale)
	}
	if !state.rows.hasItem(keep) {
		t.Fatalf("expected kept row to remain tracked")
	}
	if state.rows.hasItem(stale) {
		t.Fatalf("expected stale row to be removed")
	}
}

func TestTreeStateMarkNoMoreCommits(t *testing.T) {
	state := newTreeState()
	state.hasMore = true

	state.markNoMoreCommits()

	if state.hasMore {
		t.Fatalf("expected hasMore cleared")
	}
}

func TestTreeStateCommitBatchLoadLifecycle(t *testing.T) {
	state := newTreeState()
	state.hasMore = true

	if !state.beginCommitBatchLoad(false) {
		t.Fatalf("expected load to begin")
	}
	if !state.loadingBatch {
		t.Fatalf("expected loading batch")
	}

	if state.beginCommitBatchLoad(false) {
		t.Fatalf("expected concurrent load to be rejected")
	}

	state.finishCommitBatchLoad()
	if state.loadingBatch {
		t.Fatalf("expected loading cleared")
	}

	state.hasMore = false
	if state.beginCommitBatchLoad(false) {
		t.Fatalf("expected foreground load without more commits to be rejected")
	}
	if !state.beginCommitBatchLoad(true) {
		t.Fatalf("expected prefetch load to be allowed")
	}
}

func TestLocalDiffCacheState(t *testing.T) {
	var cache localDiffCache
	if got := cache.state(false, false); got != nil {
		t.Fatalf("expected nil without create, got %+v", got)
	}
	a := cache.state(false, true)
	if a == nil {
		t.Fatalf("expected state allocation")
	}
	b := cache.state(false, false)
	if b != a {
		t.Fatalf("expected same state instance, got %p and %p", a, b)
	}
	c := cache.state(true, true)
	if c == nil || c == a {
		t.Fatalf("expected distinct staged state, got %p and %p", a, c)
	}
}

func TestLocalDiffStateSnapshotCopiesSections(t *testing.T) {
	state := &localDiffState{
		ready:    true,
		loading:  false,
		diff:     "diff",
		sections: []git.FileSection{{Path: "a", Line: 1}},
		err:      errors.New("boom"),
	}
	state.Lock()
	snap := state.snapshotLocked()
	state.Unlock()
	if !snap.ready || snap.loading {
		t.Fatalf("unexpected flags in snapshot: %+v", snap)
	}
	if snap.diff != "diff" {
		t.Fatalf("unexpected diff in snapshot: %q", snap.diff)
	}
	if snap.err == nil || snap.err.Error() != "boom" {
		t.Fatalf("unexpected error in snapshot: %+v", snap.err)
	}
	if len(snap.sections) != 1 || snap.sections[0].Path != "a" {
		t.Fatalf("unexpected sections in snapshot: %+v", snap.sections)
	}
	state.sections[0].Path = "mutated"
	if snap.sections[0].Path != "a" {
		t.Fatalf("expected snapshot to be independent copy, got %+v", snap.sections)
	}
}

func TestLocalDiffStateStartLoadingAndReset(t *testing.T) {
	state := &localDiffState{ready: true, generation: 41}
	state.Lock()
	gen, started := state.startLoadingLocked(false)
	state.Unlock()
	if started {
		t.Fatalf("expected no start when ready and !force")
	}
	if gen != 0 {
		t.Fatalf("expected gen=0 when not started, got %d", gen)
	}

	state.Lock()
	gen, started = state.startLoadingLocked(true)
	state.Unlock()
	if !started {
		t.Fatalf("expected start when forced")
	}
	if gen != 42 {
		t.Fatalf("expected generation 42, got %d", gen)
	}
	if !state.loading || state.ready {
		t.Fatalf("unexpected flags after start: ready=%v loading=%v", state.ready, state.loading)
	}

	state.Lock()
	state.resetLocked()
	state.Unlock()
	if state.loading || state.ready {
		t.Fatalf("expected reset to clear ready/loading")
	}
	if state.generation != 43 {
		t.Fatalf("expected reset to bump generation to 43, got %d", state.generation)
	}
}
