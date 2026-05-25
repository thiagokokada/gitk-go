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
