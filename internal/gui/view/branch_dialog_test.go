package view

import (
	"slices"
	"testing"
)

func TestBuildBranchSwitchRows_SortsDedupesAndMovesCurrent(t *testing.T) {
	rows := BuildBranchSwitchRows([]string{"z", "main", "dev", "main"}, "main")
	if len(rows) != 3 {
		t.Fatalf("len(rows) = %d, want 3", len(rows))
	}
	if rows[0].Name != "main" || !rows[0].IsCurrent {
		t.Fatalf("first row = %+v, want current main first", rows[0])
	}
	gotNames := []string{rows[0].Name, rows[1].Name, rows[2].Name}
	if !slices.Equal(gotNames, []string{"main", "dev", "z"}) {
		t.Fatalf("names = %#v, want %#v", gotNames, []string{"main", "dev", "z"})
	}
	if rows[0].Display == rows[0].Name {
		t.Fatalf("expected current display to differ from name, got %q", rows[0].Display)
	}
}

func TestFilterBranchSwitchRows_CaseInsensitiveSubstring(t *testing.T) {
	all := []BranchSwitchRow{
		{Name: "main"},
		{Name: "feature/login"},
		{Name: "bugfix/Crash"},
	}
	got := FilterBranchSwitchRows(all, "CRa")
	if len(got) != 1 || got[0].Name != "bugfix/Crash" {
		t.Fatalf("filter result = %#v, want bugfix/Crash", got)
	}
}
