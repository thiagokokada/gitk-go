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
	if got := model.state.tree.rows.visibleByID[h1]; got != 0 {
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
