package gui

import (
	"testing"

	"github.com/thiagokokada/gitk-go/internal/git"
)

func TestFileSectionIndexForLine(t *testing.T) {
	sections := []git.FileSection{
		{Path: "Commit", Line: 1},
		{Path: "a.go", Line: 5},
		{Path: "b.go", Line: 10},
	}
	tests := []struct {
		line int
		want int
	}{
		{line: -1, want: 0},
		{line: 0, want: 0},
		{line: 1, want: 0},
		{line: 4, want: 0},
		{line: 5, want: 1},
		{line: 9, want: 1},
		{line: 10, want: 2},
		{line: 999, want: 2},
	}
	for _, tc := range tests {
		if got := fileSectionIndexForLine(sections, tc.line); got != tc.want {
			t.Fatalf("line=%d: want %d, got %d", tc.line, tc.want, got)
		}
	}
}
