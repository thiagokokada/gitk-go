package gui

import (
	"testing"

	"github.com/thiagokokada/gitk-go/internal/git"
)

func TestBuildVisibleIndex(t *testing.T) {
	entries := []*git.Entry{
		{Commit: &git.Commit{Hash: "a"}},
		nil,
		{Commit: &git.Commit{Hash: "b"}},
		{Commit: nil},
	}
	index := buildVisibleIndex(entries)
	if len(index) != 2 {
		t.Fatalf("expected 2 entries in index, got %d", len(index))
	}
	if got := index["a"]; got != 0 {
		t.Fatalf("expected index for a to be 0, got %d", got)
	}
	if got := index["b"]; got != 2 {
		t.Fatalf("expected index for b to be 2, got %d", got)
	}
}

func TestBuildCommitIDSet(t *testing.T) {
	entries := []*git.Entry{
		{Commit: &git.Commit{Hash: "a"}},
		{Commit: &git.Commit{Hash: "b"}},
		nil,
	}
	ids := buildCommitIDSet(entries)
	if len(ids) != 2 {
		t.Fatalf("expected 2 ids, got %d", len(ids))
	}
	if _, ok := ids["a"]; !ok {
		t.Fatalf("expected id a to be present")
	}
	if _, ok := ids["b"]; !ok {
		t.Fatalf("expected id b to be present")
	}
}

func TestTreeRowStateSetCommitIDsEmpty(t *testing.T) {
	var state treeRowState
	state.setCommitIDs(nil)
	if state.commitIDs == nil {
		t.Fatalf("expected commitIDs to be initialized")
	}
	if len(state.commitIDs) != 0 {
		t.Fatalf("expected empty commitIDs map, got %d", len(state.commitIDs))
	}
}
