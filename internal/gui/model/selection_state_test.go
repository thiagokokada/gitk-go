package model

import (
	"strings"
	"testing"

	"github.com/thiagokokada/gitk-go/internal/git"
)

func TestSelectionStateCommitIndex(t *testing.T) {
	visible := []git.Entry{
		{Commit: git.Commit{Hash: strings.Repeat("a", 40)}},
		{Commit: git.Commit{Hash: strings.Repeat("b", 40)}},
	}

	t.Run("empty", func(t *testing.T) {
		var sel SelectionState
		if got := sel.CommitIndex(visible); got != -1 {
			t.Fatalf("expected -1, got %d", got)
		}
	})

	t.Run("direct-hit", func(t *testing.T) {
		var sel SelectionState
		sel.SetCommit(visible[1], 1)
		if got := sel.CommitIndex(visible); got != 1 {
			t.Fatalf("expected 1, got %d", got)
		}
	})

	t.Run("hash-miss", func(t *testing.T) {
		var sel SelectionState
		sel.SetCommit(git.Entry{Commit: git.Commit{Hash: strings.Repeat("c", 40)}}, 0)
		if got := sel.CommitIndex(visible); got != -1 {
			t.Fatalf("expected -1, got %d", got)
		}
	})

	t.Run("fallback-scan", func(t *testing.T) {
		var sel SelectionState
		sel.SetCommit(visible[0], 10)
		if got := sel.CommitIndex(visible); got != 0 {
			t.Fatalf("expected 0, got %d", got)
		}
	})
}

func TestSelectionStateCommitHash(t *testing.T) {
	var sel SelectionState
	if got := sel.CommitHash(); got != "" {
		t.Fatalf("expected empty hash, got %q", got)
	}
	sel.SetLocal(false)
	if got := sel.CommitHash(); got != "" {
		t.Fatalf("expected empty hash for local selection, got %q", got)
	}
	entry := git.Entry{Commit: git.Commit{Hash: "abc"}}
	sel.SetCommit(entry, 0)
	if got := sel.CommitHash(); got != "abc" {
		t.Fatalf("expected hash %q, got %q", "abc", got)
	}
}

func TestSelectionStateLocalSelection(t *testing.T) {
	var sel SelectionState
	if _, ok := sel.LocalSelection(); ok {
		t.Fatalf("expected empty selection to return ok=false")
	}
	sel.SetLocal(false)
	staged, ok := sel.LocalSelection()
	if !ok {
		t.Fatalf("expected local selection to return ok=true")
	}
	if staged {
		t.Fatalf("expected unstaged local selection")
	}
	sel.SetLocal(true)
	staged, ok = sel.LocalSelection()
	if !ok {
		t.Fatalf("expected local selection to return ok=true")
	}
	if !staged {
		t.Fatalf("expected staged local selection")
	}
	entry := git.Entry{Commit: git.Commit{Hash: "abc"}}
	sel.SetCommit(entry, 0)
	if _, ok := sel.LocalSelection(); ok {
		t.Fatalf("expected commit selection to return ok=false")
	}
}

func TestSelectionStateMatchesTreeID(t *testing.T) {
	var sel SelectionState
	if sel.MatchesTreeID("abc") {
		t.Fatalf("expected empty selection to return false")
	}

	sel.SetLocal(false)
	if !sel.MatchesTreeID(LocalUnstagedRowID) {
		t.Fatalf("expected local unstaged selection to match")
	}
	if sel.MatchesTreeID(LocalStagedRowID) {
		t.Fatalf("expected local unstaged selection to not match staged row")
	}

	sel.SetLocal(true)
	if !sel.MatchesTreeID(LocalStagedRowID) {
		t.Fatalf("expected local staged selection to match")
	}
	if sel.MatchesTreeID(LocalUnstagedRowID) {
		t.Fatalf("expected local staged selection to not match unstaged row")
	}

	entry := git.Entry{Commit: git.Commit{Hash: "abc"}}
	sel.SetCommit(entry, 0)
	if !sel.MatchesTreeID("abc") {
		t.Fatalf("expected commit selection to match hash")
	}
	if sel.MatchesTreeID("def") {
		t.Fatalf("expected commit selection to not match other hash")
	}
	if sel.MatchesTreeID(LocalUnstagedRowID) {
		t.Fatalf("expected commit selection to not match local row")
	}
}
