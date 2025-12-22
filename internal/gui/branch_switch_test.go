package gui

import (
	"slices"
	"testing"
)

func TestBuildBranchChoices_SortsDedupesAndMovesCurrent(t *testing.T) {
	choices := buildBranchChoices([]string{"z", "main", "dev", "main"}, "main")
	if len(choices) != 3 {
		t.Fatalf("len(choices) = %d, want 3", len(choices))
	}
	if choices[0].name != "main" || !choices[0].isCurrent {
		t.Fatalf("first choice = %+v, want current main first", choices[0])
	}
	gotNames := []string{choices[0].name, choices[1].name, choices[2].name}
	if !slices.Equal(gotNames, []string{"main", "dev", "z"}) {
		t.Fatalf("names = %#v, want %#v", gotNames, []string{"main", "dev", "z"})
	}
	if choices[0].display == choices[0].name {
		t.Fatalf("expected current display to differ from name, got %q", choices[0].display)
	}
}

func TestFilterBranchChoices_CaseInsensitiveSubstring(t *testing.T) {
	all := []branchChoice{
		{name: "main"},
		{name: "feature/login"},
		{name: "bugfix/Crash"},
	}
	got := filterBranchChoices(all, "CRa")
	if len(got) != 1 || got[0].name != "bugfix/Crash" {
		t.Fatalf("filter result = %#v, want bugfix/Crash", got)
	}
}
