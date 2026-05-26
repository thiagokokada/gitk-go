package model

import (
	"strings"
	"testing"
	"time"

	"github.com/thiagokokada/gitk-go/internal/git"
)

func TestBuildVisibleIndex(t *testing.T) {
	entries := []*git.Entry{
		{Commit: &git.Commit{Hash: "a"}},
		nil,
		{Commit: &git.Commit{Hash: "b"}},
		{Commit: nil},
	}
	index := BuildVisibleIndex(entries)
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

func TestBuildVisibleIndexIntoReuse(t *testing.T) {
	entries := []*git.Entry{
		{Commit: &git.Commit{Hash: "a"}},
		{Commit: &git.Commit{Hash: "b"}},
	}
	index := BuildVisibleIndexInto(entries, nil)
	if len(index) != 2 {
		t.Fatalf("expected 2 entries in index, got %d", len(index))
	}
	next := []*git.Entry{{Commit: &git.Commit{Hash: "c"}}}
	index = BuildVisibleIndexInto(next, index)
	if len(index) != 1 {
		t.Fatalf("expected 1 entry in index, got %d", len(index))
	}
	if _, ok := index["a"]; ok {
		t.Fatalf("expected index to drop old entry a")
	}
	if got := index["c"]; got != 0 {
		t.Fatalf("expected index for c to be 0, got %d", got)
	}
}

func TestBuildCommitIDSet(t *testing.T) {
	entries := []*git.Entry{
		{Commit: &git.Commit{Hash: "a"}},
		{Commit: &git.Commit{Hash: "b"}},
		nil,
	}
	ids := BuildCommitIDSet(entries)
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

func TestFormatGraphValue(t *testing.T) {
	entry := &git.Entry{Graph: "* |"}
	graph := FormatGraphValue(entry, []string{"HEAD -> main", "feature"}, false)
	expected := "* | [HEAD -> main, feature]"
	if graph != expected {
		t.Fatalf("unexpected graph string: %q", graph)
	}

	entry = &git.Entry{}
	graph = FormatGraphValue(entry, nil, false)
	if graph != "*" {
		t.Fatalf("expected fallback graph '*', got %q", graph)
	}
}

func TestBuildTreeRows(t *testing.T) {
	now := time.Date(2025, 2, 1, 12, 0, 0, 0, time.UTC)
	entry1 := &git.Entry{
		Commit: &git.Commit{
			Hash:   "1111111111111111111111111111111111111111",
			Author: git.Signature{Name: "Alice", Email: "alice@example.com", When: now},
			Committer: git.Signature{
				Name:  "Alice",
				Email: "alice@example.com",
				When:  now,
			},
			Message: "first message",
		},
		Graph: "* |",
	}
	entry2 := &git.Entry{
		Commit: &git.Commit{
			Hash:   "2222222222222222222222222222222222222222",
			Author: git.Signature{Name: "Bob", Email: "bob@example.com", When: now.Add(-time.Hour)},
			Committer: git.Signature{
				Name:  "Bob",
				Email: "bob@example.com",
				When:  now.Add(-2 * time.Hour),
			},
			Message: "second message line\nmore",
		},
		Graph: "|/",
	}
	labels := map[string][]string{
		entry1.Commit.Hash: {"HEAD -> main"},
	}
	rows := BuildTreeRows([]*git.Entry{entry1, entry2}, labels, false)
	if len(rows) != 2 {
		t.Fatalf("expected two rows, got %d", len(rows))
	}
	if rows[0].ID != entry1.Commit.Hash || rows[1].ID != entry2.Commit.Hash {
		t.Fatalf("unexpected row ids: %#v", rows)
	}
	if rows[0].Graph != "* | [HEAD -> main]" {
		t.Fatalf("unexpected graph: %q", rows[0].Graph)
	}
	if !strings.Contains(rows[0].Commit, "first message") {
		t.Fatalf("missing commit message in row: %q", rows[0].Commit)
	}
	if !strings.Contains(rows[1].Author, "Bob") || !strings.Contains(rows[1].Author, "bob@example.com") {
		t.Fatalf("unexpected author column: %q", rows[1].Author)
	}
	if !strings.Contains(rows[1].Date, "2025-02-01 10:00") {
		t.Fatalf("unexpected date column: %q", rows[1].Date)
	}
}

func TestTreeRowStateSetCommitIDsEmpty(t *testing.T) {
	var state TreeRowState
	state.SetCommitIDs(nil)
	if state.CommitIDs == nil {
		t.Fatalf("expected commitIDs to be initialized")
	}
	if len(state.CommitIDs) != 0 {
		t.Fatalf("expected empty commitIDs map, got %d", len(state.CommitIDs))
	}
}

func TestTreeRowStateItemValueChanged(t *testing.T) {
	var state TreeRowState
	row := TreeRow{Graph: "*", Commit: "c1", Author: "a1", Date: "d1"}
	if !state.ItemValueChanged("id", row) {
		t.Fatalf("expected itemValueChanged to be true for missing entry")
	}
	state.SetItemValue("id", row)
	if state.ItemValueChanged("id", row) {
		t.Fatalf("expected itemValueChanged to be false for unchanged entry")
	}
	changed := TreeRow{Graph: "*", Commit: "c1", Author: "a2", Date: "d1"}
	if !state.ItemValueChanged("id", changed) {
		t.Fatalf("expected itemValueChanged to be true for changed entry")
	}
}

func TestTreeRowStateSpecialItems(t *testing.T) {
	var state TreeRowState
	if state.HasSpecialItem("row") {
		t.Fatalf("expected no special items initially")
	}
	state.AddSpecialItem("row")
	if !state.HasSpecialItem("row") {
		t.Fatalf("expected special item to be present")
	}
	state.RemoveSpecialItem("row")
	if state.HasSpecialItem("row") {
		t.Fatalf("expected special item to be removed")
	}
}
