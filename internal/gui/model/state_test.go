package model

import (
	"errors"
	"testing"

	"github.com/thiagokokada/gitk-go/internal/git"
)

func TestNewDiffStateInitializesSyntaxTags(t *testing.T) {
	state := NewDiffState()
	if state.SyntaxTags == nil {
		t.Fatalf("expected syntax tags map")
	}
}

func TestDiffStateSetFileSectionsResetsSelection(t *testing.T) {
	state := NewDiffState()
	state.SelectedFileIndex = 2

	state.SetFileSections([]git.FileSection{{Path: "a.go", Line: 10}})

	if state.SelectedFileIndex != -1 {
		t.Fatalf("selected index = %d, want -1", state.SelectedFileIndex)
	}
	if len(state.FileSections) != 1 || state.FileSections[0].Path != "a.go" {
		t.Fatalf("unexpected file Sections: %+v", state.FileSections)
	}
}

func TestDiffStateBeginUserFileSelection(t *testing.T) {
	state := NewDiffState()
	state.SetFileSections(NewDiffViewModel([]git.FileSection{{Path: "a.go", Line: 7}}).Sections)

	line, ok := state.BeginUserFileSelection(1)
	if !ok {
		t.Fatalf("expected file selection")
	}
	if line != 7 {
		t.Fatalf("line = %d, want 7", line)
	}
	if !state.SkipNextSync {
		t.Fatalf("expected skip next sync")
	}

	if _, ok := state.BeginUserFileSelection(-1); ok {
		t.Fatalf("expected invalid selection to fail")
	}
	state.SuppressFileSelection = true
	if _, ok := state.BeginUserFileSelection(1); ok {
		t.Fatalf("expected suppressed selection to fail")
	}
}

func TestDiffStateSelectFileIndex(t *testing.T) {
	state := NewDiffState()
	state.SetFileSections(NewDiffViewModel([]git.FileSection{{Path: "a.go", Line: 7}}).Sections)

	if !state.SelectFileIndex(1) {
		t.Fatalf("expected selection to change")
	}
	if state.SelectedFileIndex != 1 {
		t.Fatalf("selected index = %d, want 1", state.SelectedFileIndex)
	}
	if !state.SuppressFileSelection {
		t.Fatalf("expected suppress flag")
	}
	if state.SelectFileIndex(1) {
		t.Fatalf("expected unchanged selection")
	}

	state.FinishProgrammaticFileSelection()
	if state.SuppressFileSelection {
		t.Fatalf("expected suppress flag cleared")
	}
}

func TestDiffStateSyncSelectionIndexForLine(t *testing.T) {
	state := NewDiffState()
	state.SetFileSections(NewDiffViewModel([]git.FileSection{
		{Path: "a.go", Line: 7},
		{Path: "b.go", Line: 20},
	}).Sections)

	idx, ok := state.SyncSelectionIndexForLine(21)
	if !ok {
		t.Fatalf("expected index for line")
	}
	if idx != 2 {
		t.Fatalf("index = %d, want 2", idx)
	}
	if _, ok := state.SyncSelectionIndexForLine(0); ok {
		t.Fatalf("expected invalid line")
	}
}

func TestDiffStateSelectedFilePath(t *testing.T) {
	state := NewDiffState()
	state.SetFileSections(NewDiffViewModel([]git.FileSection{{Path: "a.go", Line: 7}}).Sections)
	if _, ok := state.SelectedFilePath(); ok {
		t.Fatalf("expected no path without selection")
	}
	state.SelectedFileIndex = 1
	path, ok := state.SelectedFilePath()
	if !ok || path != "a.go" {
		t.Fatalf("path = %q ok=%v, want a.go true", path, ok)
	}
}

func TestDiffStateConsumeSkipNextSync(t *testing.T) {
	state := NewDiffState()
	if state.ConsumeSkipNextSync() {
		t.Fatalf("expected no skip")
	}
	state.SkipNextSync = true
	if !state.ConsumeSkipNextSync() {
		t.Fatalf("expected skip")
	}
	if state.SkipNextSync {
		t.Fatalf("expected skip flag cleared")
	}
}

func TestNewAppModelInitializesStateContainers(t *testing.T) {
	model := NewApp("/repo/path")
	if model.Repo.Path != "/repo/path" {
		t.Fatalf("repo path = %q, want %q", model.Repo.Path, "/repo/path")
	}
	if model.State.Diff.SyntaxTags == nil {
		t.Fatalf("expected diff syntax tags")
	}
	if model.State.Tree.BranchLabels == nil {
		t.Fatalf("expected branch labels map")
	}
	if model.State.Tree.Rows.CommitIDs == nil {
		t.Fatalf("expected tree row commit ids map")
	}
	if model.State.Tree.Rows.VisibleByID == nil {
		t.Fatalf("expected tree row visible index map")
	}
	if model.State.Tree.Rows.Items == nil {
		t.Fatalf("expected tree row items map")
	}
	if model.State.Tree.Rows.Values == nil {
		t.Fatalf("expected tree row values map")
	}
	if model.State.Tree.Rows.SpecialItems == nil {
		t.Fatalf("expected tree row special items map")
	}
	if model.State.LocalDiff.Items == nil {
		t.Fatalf("expected local diff cache map")
	}
}

func TestAppModelResetRepositoryClearsFilter(t *testing.T) {
	model := NewApp("/old")
	model.Repo.HeadRef = "main"
	model.Data.Commits = []git.Entry{{Commit: git.Commit{Hash: "1111111111111111111111111111111111111111"}}}
	model.Data.Visible = model.Data.Commits
	model.State.Filter.Value = "feature"
	model.State.Tree.HasMore = true
	model.State.LocalDiff.State(false, true)

	model.ResetRepository("/new")

	if model.Repo.Path != "/new" {
		t.Fatalf("repo path = %q, want %q", model.Repo.Path, "/new")
	}
	if model.Repo.HeadRef != "" {
		t.Fatalf("expected head ref reset, got %q", model.Repo.HeadRef)
	}
	if len(model.Data.Commits) != 0 || len(model.Data.Visible) != 0 {
		t.Fatalf("expected commit data reset")
	}
	if model.State.Filter.Value != "" {
		t.Fatalf("expected filter cleared, got %q", model.State.Filter.Value)
	}
	if model.State.Tree.HasMore {
		t.Fatalf("expected tree state reset")
	}
	if model.State.Diff.SyntaxTags == nil {
		t.Fatalf("expected reset diff syntax tags")
	}
}

func TestAppModelResetBranchPreservesFilter(t *testing.T) {
	model := NewApp("/repo")
	model.Repo.HeadRef = "main"
	model.Data.Commits = []git.Entry{{Commit: git.Commit{Hash: "1111111111111111111111111111111111111111"}}}
	model.Data.Visible = model.Data.Commits
	model.State.Filter.Value = "feature"
	model.State.Tree.HasMore = true

	model.ResetBranch()

	if model.Repo.Path != "/repo" {
		t.Fatalf("repo path = %q, want %q", model.Repo.Path, "/repo")
	}
	if model.Repo.HeadRef != "" {
		t.Fatalf("expected head ref reset, got %q", model.Repo.HeadRef)
	}
	if len(model.Data.Commits) != 0 || len(model.Data.Visible) != 0 {
		t.Fatalf("expected commit data reset")
	}
	if model.State.Filter.Value != "feature" {
		t.Fatalf("expected filter preserved, got %q", model.State.Filter.Value)
	}
	if model.State.Tree.HasMore {
		t.Fatalf("expected tree state reset")
	}
}

func TestAppModelApplyFilterUpdatesVisibleRows(t *testing.T) {
	h1 := "1111111111111111111111111111111111111111"
	h2 := "2222222222222222222222222222222222222222"
	model := NewApp("/repo")
	model.Data.Commits = []git.Entry{
		{Commit: git.Commit{Hash: h1}, SearchText: "feature branch"},
		{Commit: git.Commit{Hash: h2}, SearchText: "bug fix"},
	}

	model.ApplyFilter("FEATURE")

	if model.State.Filter.Value != "FEATURE" {
		t.Fatalf("filter value = %q, want %q", model.State.Filter.Value, "FEATURE")
	}
	if len(model.Data.Visible) != 1 || model.Data.Visible[0].Commit.Hash != h1 {
		t.Fatalf("unexpected visible commits: %+v", model.Data.Visible)
	}
	if got, ok := model.State.Tree.Rows.VisibleByID[h1]; !ok || got != 0 {
		t.Fatalf("visible index for h1 = %d, want 0", got)
	}
	if _, ok := model.State.Tree.Rows.VisibleByID[h2]; ok {
		t.Fatalf("filtered commit should not be indexed")
	}
}

func TestAppModelSetReloadedCommitsResetsBatchState(t *testing.T) {
	h1 := "1111111111111111111111111111111111111111"
	entries := []git.Entry{{Commit: git.Commit{Hash: h1}}}
	model := NewApp("/repo")

	model.SetReloadedCommits(entries, "main", true)

	if model.Repo.HeadRef != "main" {
		t.Fatalf("head ref = %q, want %q", model.Repo.HeadRef, "main")
	}
	if len(model.Data.Commits) != 1 || model.Data.Commits[0].Commit.Hash != entries[0].Commit.Hash {
		t.Fatalf("unexpected commits: %+v", model.Data.Commits)
	}
	if len(model.Data.Visible) != 1 || model.Data.Visible[0].Commit.Hash != entries[0].Commit.Hash {
		t.Fatalf("unexpected visible commits: %+v", model.Data.Visible)
	}
	if !model.State.Tree.HasMore {
		t.Fatalf("expected hasMore set")
	}
	if _, ok := model.State.Tree.Rows.CommitIDs[h1]; !ok {
		t.Fatalf("expected commit id indexed")
	}
	if !model.State.Tree.Rows.RefreshValues {
		t.Fatalf("expected tree row refresh")
	}
}

func TestAppModelAppendCommitsExtendsBatchState(t *testing.T) {
	h1 := "1111111111111111111111111111111111111111"
	h2 := "2222222222222222222222222222222222222222"
	model := NewApp("/repo")
	model.SetReloadedCommits([]git.Entry{{Commit: git.Commit{Hash: h1}}}, "main", true)
	model.State.Tree.Rows.RefreshValues = false

	model.AppendCommits([]git.Entry{{Commit: git.Commit{Hash: h2}}}, false)

	if len(model.Data.Commits) != 2 {
		t.Fatalf("commit count = %d, want 2", len(model.Data.Commits))
	}
	if model.State.Tree.HasMore {
		t.Fatalf("expected hasMore cleared")
	}
	if _, ok := model.State.Tree.Rows.CommitIDs[h1]; !ok {
		t.Fatalf("expected existing commit id retained")
	}
	if _, ok := model.State.Tree.Rows.CommitIDs[h2]; !ok {
		t.Fatalf("expected appended commit id indexed")
	}
	if !model.State.Tree.Rows.RefreshValues {
		t.Fatalf("expected tree row refresh")
	}
}

func TestAppModelFallbackSelectionPlan(t *testing.T) {
	model := NewApp("/repo")
	plan := model.FallbackSelectionPlan()
	if plan.Kind != SelectionDisplayMessage || plan.Message != "Repository has no commits yet." {
		t.Fatalf("unexpected empty repo plan: %+v", plan)
	}

	model.Data.Commits = []git.Entry{{Commit: git.Commit{Hash: "1111111111111111111111111111111111111111"}}}
	plan = model.FallbackSelectionPlan()
	if plan.Kind != SelectionDisplayMessage || plan.Message != "No commits match the current filter." {
		t.Fatalf("unexpected filtered repo plan: %+v", plan)
	}

	entry := git.Entry{Commit: git.Commit{Hash: "1111111111111111111111111111111111111111"}}
	model.Data.Visible = []git.Entry{entry}
	plan = model.FallbackSelectionPlan()
	if plan.Kind != SelectionDisplayCommit || plan.Entry.Commit.Hash != entry.Commit.Hash ||
		plan.Index != 0 || !plan.LoadDetail {
		t.Fatalf("unexpected commit fallback plan: %+v", plan)
	}
}

func TestAppModelFilterSelectionPlanPreservesVisibleLocalRow(t *testing.T) {
	model := NewApp("/repo")
	model.State.Selection.SetLocal(true)
	model.State.Tree.Rows.AddSpecialItem(LocalStagedRowID)

	plan := model.FilterSelectionPlan()

	if plan.Kind != SelectionDisplayLocal || !plan.Staged {
		t.Fatalf("unexpected local plan: %+v", plan)
	}
}

func TestAppModelFilterSelectionPlanChoosesCommit(t *testing.T) {
	h1 := "1111111111111111111111111111111111111111"
	h2 := "2222222222222222222222222222222222222222"
	first := git.Entry{Commit: git.Commit{Hash: h1}}
	second := git.Entry{Commit: git.Commit{Hash: h2}}
	model := NewApp("/repo")
	model.Data.Commits = []git.Entry{first, second}
	model.Data.Visible = model.Data.Commits
	model.State.Tree.Rows.SetVisibleIndex(model.Data.Visible)
	model.State.Selection.SetCommit(second, 1)

	plan := model.FilterSelectionPlan()

	if plan.Kind != SelectionDisplayCommit || plan.Entry.Commit.Hash != second.Commit.Hash ||
		plan.Index != 1 || plan.LoadDetail {
		t.Fatalf("unexpected existing commit plan: %+v", plan)
	}

	model.Data.Visible = []git.Entry{first}
	model.State.Tree.Rows.SetVisibleIndex(model.Data.Visible)
	plan = model.FilterSelectionPlan()
	if plan.Kind != SelectionDisplayCommit || plan.Entry.Commit.Hash != first.Commit.Hash ||
		plan.Index != 0 || !plan.LoadDetail {
		t.Fatalf("unexpected fallback commit plan: %+v", plan)
	}
}

func TestAppModelTreeSelectionPlan(t *testing.T) {
	hash := "1111111111111111111111111111111111111111"
	entry := git.Entry{Commit: git.Commit{Hash: hash}}
	model := NewApp("/repo")
	model.Data.Visible = []git.Entry{entry}
	model.State.Tree.Rows.SetVisibleIndex(model.Data.Visible)

	plan := model.TreeSelectionPlan(hash)
	if plan.Kind != TreeSelectionCommit || plan.Entry.Commit.Hash != entry.Commit.Hash || plan.Index != 0 {
		t.Fatalf("unexpected commit tree plan: %+v", plan)
	}

	model.State.Selection.SetCommit(entry, 0)
	plan = model.TreeSelectionPlan(hash)
	if plan.Kind != TreeSelectionNone {
		t.Fatalf("expected unchanged selection to be ignored, got %+v", plan)
	}

	plan = model.TreeSelectionPlan(LocalUnstagedRowID)
	if plan.Kind != TreeSelectionLocal || plan.Staged {
		t.Fatalf("unexpected local tree plan: %+v", plan)
	}

	plan = model.TreeSelectionPlan(LoadingIndicatorID)
	if plan.Kind != TreeSelectionClear || model.State.Selection.CommitHash() != "" {
		t.Fatalf("unexpected clear tree plan: %+v", plan)
	}
}

func TestTreeStateLocalRowVisibility(t *testing.T) {
	state := NewTreeState()
	if !state.SetLocalRowVisible(false, true) {
		t.Fatalf("expected unstaged row change")
	}
	if state.SetLocalRowVisible(false, true) {
		t.Fatalf("expected unchanged unstaged row")
	}
	if !state.LocalRowVisible(false) {
		t.Fatalf("expected unstaged row visible")
	}
	if got := state.LocalRowIDs(); len(got) != 1 || got[0] != LocalUnstagedRowID {
		t.Fatalf("unexpected local row ids: %+v", got)
	}

	if !state.SetLocalRowVisible(true, true) {
		t.Fatalf("expected staged row change")
	}
	if !state.LocalRowVisible(true) {
		t.Fatalf("expected staged row visible")
	}
	if idx := state.LocalRowInsertIndex(true); idx != 1 {
		t.Fatalf("staged insert index = %d, want 1", idx)
	}
}

func TestTreeStatePruneStaleCommitRows(t *testing.T) {
	keep := "1111111111111111111111111111111111111111"
	stale := "2222222222222222222222222222222222222222"
	state := NewTreeState()
	state.SetReloadedCommits([]git.Entry{{Commit: git.Commit{Hash: keep}}}, true)
	state.Rows.AddItem(keep)
	state.Rows.SetItemValue(keep, TreeRow{Commit: "keep"})
	state.Rows.AddItem(stale)
	state.Rows.SetItemValue(stale, TreeRow{Commit: "stale"})

	got := state.Rows.PruneStaleCommitRows()

	if len(got) != 1 || got[0] != stale {
		t.Fatalf("stale row ids = %+v, want %q", got, stale)
	}
	if !state.Rows.HasItem(keep) {
		t.Fatalf("expected kept row to remain tracked")
	}
	if state.Rows.HasItem(stale) {
		t.Fatalf("expected stale row to be removed")
	}
}

func TestTreeStateMarkNoMoreCommits(t *testing.T) {
	state := NewTreeState()
	state.HasMore = true

	state.MarkNoMoreCommits()

	if state.HasMore {
		t.Fatalf("expected hasMore cleared")
	}
}

func TestTreeStateCommitBatchLoadLifecycle(t *testing.T) {
	state := NewTreeState()
	state.HasMore = true

	if !state.BeginCommitBatchLoad(false) {
		t.Fatalf("expected load to begin")
	}
	if !state.LoadingBatch {
		t.Fatalf("expected loading batch")
	}

	if state.BeginCommitBatchLoad(false) {
		t.Fatalf("expected concurrent load to be rejected")
	}

	state.FinishCommitBatchLoad()
	if state.LoadingBatch {
		t.Fatalf("expected loading cleared")
	}

	state.HasMore = false
	if state.BeginCommitBatchLoad(false) {
		t.Fatalf("expected foreground load without more commits to be rejected")
	}
	if !state.BeginCommitBatchLoad(true) {
		t.Fatalf("expected prefetch load to be allowed")
	}
}

func TestLocalDiffCacheState(t *testing.T) {
	var cache LocalDiffCache
	if got := cache.State(false, false); got != nil {
		t.Fatalf("expected nil without create, got %+v", got)
	}
	a := cache.State(false, true)
	if a == nil {
		t.Fatalf("expected state allocation")
	}
	b := cache.State(false, false)
	if b != a {
		t.Fatalf("expected same state instance, got %p and %p", a, b)
	}
	c := cache.State(true, true)
	if c == nil || c == a {
		t.Fatalf("expected distinct staged state, got %p and %p", a, c)
	}
}

func TestLocalDiffStateSnapshotCopiesSections(t *testing.T) {
	state := &LocalDiffState{
		Ready:    true,
		Loading:  false,
		Diff:     "diff",
		Sections: []git.FileSection{{Path: "a", Line: 1}},
		Err:      errors.New("boom"),
	}
	state.Lock()
	snap := state.SnapshotLocked()
	state.Unlock()
	if !snap.Ready || snap.Loading {
		t.Fatalf("unexpected flags in snapshot: %+v", snap)
	}
	if snap.Diff != "diff" {
		t.Fatalf("unexpected diff in snapshot: %q", snap.Diff)
	}
	if snap.Err == nil || snap.Err.Error() != "boom" {
		t.Fatalf("unexpected error in snapshot: %+v", snap.Err)
	}
	if len(snap.Sections) != 1 || snap.Sections[0].Path != "a" {
		t.Fatalf("unexpected sections in snapshot: %+v", snap.Sections)
	}
	state.Sections[0].Path = "mutated"
	if snap.Sections[0].Path != "a" {
		t.Fatalf("expected snapshot to be independent copy, got %+v", snap.Sections)
	}
}

func TestLocalDiffStateStartLoadingAndReset(t *testing.T) {
	state := &LocalDiffState{Ready: true, Generation: 41}
	state.Lock()
	gen, started := state.StartLoadingLocked(false)
	state.Unlock()
	if started {
		t.Fatalf("expected no start when ready and !force")
	}
	if gen != 0 {
		t.Fatalf("expected gen=0 when not started, got %d", gen)
	}

	state.Lock()
	gen, started = state.StartLoadingLocked(true)
	state.Unlock()
	if !started {
		t.Fatalf("expected start when forced")
	}
	if gen != 42 {
		t.Fatalf("expected generation 42, got %d", gen)
	}
	if !state.Loading || state.Ready {
		t.Fatalf("unexpected flags after start: ready=%v loading=%v", state.Ready, state.Loading)
	}

	state.Lock()
	state.ResetLocked()
	state.Unlock()
	if state.Loading || state.Ready {
		t.Fatalf("expected reset to clear ready/loading")
	}
	if state.Generation != 43 {
		t.Fatalf("expected reset to bump generation to 43, got %d", state.Generation)
	}
}
