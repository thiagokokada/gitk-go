package model

import (
	"testing"

	"github.com/thiagokokada/gitk-go/internal/git"
)

func TestDiffFilePathForIndex(t *testing.T) {
	sections := NewDiffViewModel([]git.FileSection{
		{Path: "a.go", Line: 5, Added: 3, Removed: 1},
		{Path: "Commit", Line: 10},
	}).Sections
	tests := []struct {
		idx    int
		want   string
		wantOK bool
	}{
		{idx: -1, want: "", wantOK: false},
		{idx: 0, want: "", wantOK: false},
		{idx: 1, want: "a.go", wantOK: true},
		{idx: 2, want: "Commit", wantOK: true},
		{idx: 3, want: "", wantOK: false},
	}
	for _, tc := range tests {
		got, ok := DiffFilePathForIndex(sections, tc.idx)
		if ok != tc.wantOK || got != tc.want {
			t.Fatalf("idx=%d: want (%q,%v), got (%q,%v)", tc.idx, tc.want, tc.wantOK, got, ok)
		}
	}
}

func TestNewDiffViewModel_LabelsIncludeLineCounts(t *testing.T) {
	model := NewDiffViewModel([]git.FileSection{
		{Path: "main.go", Line: 5, Added: 12, Removed: 4},
		{Path: "README.md", Line: 20, Added: 0, Removed: 2},
	})

	if len(model.Rows) != 3 {
		t.Fatalf("expected 3 rows, got %d: %+v", len(model.Rows), model.Rows)
	}
	if model.Rows[0].Label != "Commit (+12 -6)" {
		t.Fatalf("unexpected commit label: %q", model.Rows[0].Label)
	}
	if model.Rows[1].Label != "main.go (+12 -4)" {
		t.Fatalf("unexpected first file label: %q", model.Rows[1].Label)
	}
	if model.Rows[2].Label != "README.md (+0 -2)" {
		t.Fatalf("unexpected second file label: %q", model.Rows[2].Label)
	}
}
