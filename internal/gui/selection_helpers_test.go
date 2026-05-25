package gui

import (
	"testing"

	"github.com/thiagokokada/gitk-go/internal/git"
	"github.com/thiagokokada/gitk-go/internal/gui/model"
	"github.com/thiagokokada/gitk-go/internal/gui/selection"
)

func TestSelectionMatchesTreeID(t *testing.T) {
	var sel selection.State
	if selectionMatchesTreeID(&sel, "abc") {
		t.Fatalf("expected empty selection to return false")
	}

	sel.SetLocal(false)
	if !selectionMatchesTreeID(&sel, model.LocalUnstagedRowID) {
		t.Fatalf("expected local unstaged selection to match")
	}
	if selectionMatchesTreeID(&sel, model.LocalStagedRowID) {
		t.Fatalf("expected local unstaged selection to not match staged row")
	}

	sel.SetLocal(true)
	if !selectionMatchesTreeID(&sel, model.LocalStagedRowID) {
		t.Fatalf("expected local staged selection to match")
	}
	if selectionMatchesTreeID(&sel, model.LocalUnstagedRowID) {
		t.Fatalf("expected local staged selection to not match unstaged row")
	}

	entry := &git.Entry{Commit: &git.Commit{Hash: "abc"}}
	sel.SetCommit(entry, 0)
	if !selectionMatchesTreeID(&sel, "abc") {
		t.Fatalf("expected commit selection to match hash")
	}
	if selectionMatchesTreeID(&sel, "def") {
		t.Fatalf("expected commit selection to not match other hash")
	}
	if selectionMatchesTreeID(&sel, model.LocalUnstagedRowID) {
		t.Fatalf("expected commit selection to not match local row")
	}
}
