package gui

import (
	"errors"
	"testing"

	"github.com/thiagokokada/gitk-go/internal/git"
)

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
