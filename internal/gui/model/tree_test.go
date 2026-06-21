package model

import (
	"strings"
	"testing"
	"time"

	"github.com/thiagokokada/gitk-go/internal/git"
)

func TestBuildVisibleIndex(t *testing.T) {
	entries := []git.Entry{
		{Commit: git.Commit{Hash: "a"}},
		{Commit: git.Commit{}},
		{Commit: git.Commit{Hash: "b"}},
		{Commit: git.Commit{}},
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

func TestBuildVisibleIndexEmpty(t *testing.T) {
	index := BuildVisibleIndex(nil)
	if index == nil {
		t.Fatalf("expected empty index map")
	}
	if len(index) != 0 {
		t.Fatalf("expected empty index, got %d", len(index))
	}
}

func TestBuildVisibleIndexIntoReuse(t *testing.T) {
	entries := []git.Entry{
		{Commit: git.Commit{Hash: "a"}},
		{Commit: git.Commit{Hash: "b"}},
	}
	index := BuildVisibleIndexInto(entries, nil)
	if len(index) != 2 {
		t.Fatalf("expected 2 entries in index, got %d", len(index))
	}
	next := []git.Entry{{Commit: git.Commit{Hash: "c"}}}
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

	index = BuildVisibleIndexInto(nil, index)
	if index == nil {
		t.Fatalf("expected reused empty index map")
	}
	if len(index) != 0 {
		t.Fatalf("expected reused index to be empty, got %d", len(index))
	}
}

func TestBuildCommitIDSet(t *testing.T) {
	entries := []git.Entry{
		{Commit: git.Commit{Hash: "a"}},
		{Commit: git.Commit{Hash: "b"}},
		{Commit: git.Commit{}},
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

func TestBuildCommitIDSetEmpty(t *testing.T) {
	ids := BuildCommitIDSet(nil)
	if ids == nil {
		t.Fatalf("expected empty ids map")
	}
	if len(ids) != 0 {
		t.Fatalf("expected empty ids, got %d", len(ids))
	}
}

func TestFormatGraphValue(t *testing.T) {
	entry := git.Entry{Graph: "* |"}
	graph := FormatGraphValue(entry, []string{"HEAD -> main", "feature"}, false)
	expected := "* | [HEAD -> main, feature]"
	if graph != expected {
		t.Fatalf("unexpected graph string: %q", graph)
	}

	entry = git.Entry{}
	graph = FormatGraphValue(entry, nil, false)
	if graph != "*" {
		t.Fatalf("expected fallback graph '*', got %q", graph)
	}
}

func TestBuildTreeRows(t *testing.T) {
	now := time.Date(2025, 2, 1, 12, 0, 0, 0, time.UTC)
	entry1 := git.Entry{
		Commit: git.Commit{
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
	entry2 := git.Entry{
		Commit: git.Commit{
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
	rows := BuildTreeRows([]git.Entry{entry1, entry2}, labels, false)
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
	state := NewTreeRowState()
	state.SetCommitIDs([]git.Entry{{Commit: git.Commit{Hash: "a"}}})

	state.SetCommitIDs(nil)
	if state.CommitIDs == nil {
		t.Fatalf("expected commitIDs to be initialized")
	}
	if len(state.CommitIDs) != 0 {
		t.Fatalf("expected empty commitIDs map, got %d", len(state.CommitIDs))
	}
}

func TestTreeRowStateSetVisibleIndexEmpty(t *testing.T) {
	state := NewTreeRowState()
	state.SetVisibleIndex([]git.Entry{{Commit: git.Commit{Hash: "a"}}})

	state.SetVisibleIndex(nil)
	if state.VisibleByID == nil {
		t.Fatalf("expected visibleByID to be initialized")
	}
	if len(state.VisibleByID) != 0 {
		t.Fatalf("expected empty visibleByID map, got %d", len(state.VisibleByID))
	}
}

func TestTreeRowStateItemValueChanged(t *testing.T) {
	state := NewTreeRowState()
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
	state := NewTreeRowState()
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

func TestTreeRowStateTrackedItemIDsIncludesSpecialItems(t *testing.T) {
	state := NewTreeRowState()
	state.AddItem("commit")
	state.AddSpecialItem(LoadingIndicatorID)

	ids := state.TrackedItemIDs()
	got := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		got[id] = struct{}{}
	}
	for _, id := range []string{"commit", LoadingIndicatorID} {
		if _, ok := got[id]; !ok {
			t.Fatalf("expected tracked ids to include %q, got %v", id, ids)
		}
	}
}

func TestTreeRowStateResetTrackingKeepsMapsInitialized(t *testing.T) {
	state := NewTreeRowState()
	state.AddItem("commit")
	state.SetItemValue("commit", TreeRow{Commit: "message"})
	state.AddSpecialItem(LoadingIndicatorID)

	state.ResetTracking()

	if state.Items == nil {
		t.Fatalf("expected items map to stay initialized")
	}
	if state.Values == nil {
		t.Fatalf("expected values map to stay initialized")
	}
	if state.SpecialItems == nil {
		t.Fatalf("expected special items map to stay initialized")
	}
	if len(state.Items) != 0 || len(state.Values) != 0 || len(state.SpecialItems) != 0 {
		t.Fatalf("expected tracking maps to be empty")
	}
}
