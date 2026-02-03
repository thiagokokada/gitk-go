package gui

import (
	"testing"

	"github.com/thiagokokada/gitk-go/internal/git"
)

func TestDiffFilePathForIndex(t *testing.T) {
	sections := newDiffViewModel([]git.FileSection{
		{Path: "a.go", Line: 5},
		{Path: "Commit", Line: 10},
	}).sections
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
		got, ok := diffFilePathForIndex(sections, tc.idx)
		if ok != tc.wantOK || got != tc.want {
			t.Fatalf("idx=%d: want (%q,%v), got (%q,%v)", tc.idx, tc.want, tc.wantOK, got, ok)
		}
	}
}
