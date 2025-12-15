package gui

import (
	"strings"
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

func TestDiffLineTag(t *testing.T) {
	tests := []struct {
		line string
		want string
	}{
		{line: "", want: ""},
		{line: "diff --git a/a b/a", want: "diffHeader"},
		{line: "+added", want: "diffAdd"},
		{line: "+++ b/file", want: ""},
		{line: "-removed", want: "diffDel"},
		{line: "--- a/file", want: ""},
		{line: " context", want: ""},
	}
	for _, tc := range tests {
		if got := diffLineTag(tc.line); got != tc.want {
			t.Fatalf("line=%q: want %q, got %q", tc.line, tc.want, got)
		}
	}
}

func TestPrepareDiffDisplayInsertsSpacing(t *testing.T) {
	diff := strings.Join([]string{
		"diff --git a/file1 b/file1",
		"@@ -1,0 +1,2 @@",
		"+foo",
		"diff --git a/file2 b/file2",
		"@@ -3,0 +3,2 @@",
		"+bar",
	}, "\n")
	sections := []git.FileSection{
		{Path: "file1", Line: 1},
		{Path: "file2", Line: 4},
	}
	gotDiff, gotSections := prepareDiffDisplay(diff, sections)
	lines := strings.Split(gotDiff, "\n")
	if len(lines) < 7 {
		t.Fatalf("unexpected diff line count: %d", len(lines))
	}
	if lines[3] != "" {
		t.Fatalf("expected blank spacer line between diffs, got %q", lines[3])
	}
	if gotSections[0].Line != 1 {
		t.Fatalf("expected first section to stay at line 1, got %d", gotSections[0].Line)
	}
	if gotSections[1].Line != 4 {
		t.Fatalf("expected second section to shift to line 4, got %d", gotSections[1].Line)
	}
}

func TestPrepareDiffDisplayNoContent(t *testing.T) {
	gotDiff, gotSections := prepareDiffDisplay("", nil)
	if gotDiff != "" || gotSections != nil {
		t.Fatalf("expected passthrough for empty diff, got %q %#v", gotDiff, gotSections)
	}
}

func TestDiffLineTokens(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{in: "a/file b/file", want: []string{"a/file", "b/file"}},
		{in: "  a/file\tb/file  ", want: []string{"a/file", "b/file"}},
		{in: "\"a/with space\" \"b/with space\"", want: []string{"a/with space", "b/with space"}},
		{in: "\"a/with \\\"quote\\\"\" b/x", want: []string{"a/with \"quote\"", "b/x"}},
		{in: "\"a/with \\\\ slash\" b/x", want: []string{"a/with \\ slash", "b/x"}},
	}
	for _, tc := range tests {
		got := diffLineTokens(tc.in)
		if len(got) != len(tc.want) {
			t.Fatalf("in=%q: want %d tokens, got %d (%v)", tc.in, len(tc.want), len(got), got)
		}
		for i := range tc.want {
			if got[i] != tc.want[i] {
				t.Fatalf("in=%q: token[%d] want %q, got %q", tc.in, i, tc.want[i], got[i])
			}
		}
	}
}

func TestNormalizeDiffPath(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "a/foo", want: "foo"},
		{in: "b/foo", want: "foo"},
		{in: "foo", want: "foo"},
	}
	for _, tc := range tests {
		if got := normalizeDiffPath(tc.in); got != tc.want {
			t.Fatalf("in=%q: want %q, got %q", tc.in, tc.want, got)
		}
	}
}

func TestDiffPathFromLine(t *testing.T) {
	tests := []struct {
		line   string
		want   string
		wantOK bool
	}{
		{line: "other", want: "", wantOK: false},
		{line: "diff --git", want: "", wantOK: false},
		{line: "diff --git ", want: "", wantOK: true},
		{line: "diff --git a/foo b/foo", want: "foo", wantOK: true},
		{line: "diff --git \"a/foo bar\" \"b/foo bar\"", want: "foo bar", wantOK: true},
	}
	for _, tc := range tests {
		got, ok := diffPathFromLine(tc.line)
		if ok != tc.wantOK {
			t.Fatalf("line=%q: want ok=%v, got %v (path=%q)", tc.line, tc.wantOK, ok, got)
		}
		if ok && got != tc.want {
			t.Fatalf("line=%q: want %q, got %q", tc.line, tc.want, got)
		}
	}
}

func TestDiffLineCode(t *testing.T) {
	tests := []struct {
		line      string
		wantCode  string
		wantOff   int
		wantMatch bool
	}{
		{line: "", wantMatch: false},
		{line: "diff --git a/x b/x", wantMatch: false},
		{line: "+foo", wantCode: "foo", wantOff: 1, wantMatch: true},
		{line: "-bar", wantCode: "bar", wantOff: 1, wantMatch: true},
		{line: " baz", wantCode: "baz", wantOff: 1, wantMatch: true},
		{line: "+++ b/x", wantMatch: false},
		{line: "--- a/x", wantMatch: false},
		{line: "\\ No newline at end of file", wantMatch: false},
	}
	for _, tc := range tests {
		code, off, ok := diffLineCode(tc.line)
		if ok != tc.wantMatch {
			t.Fatalf("line=%q: want ok=%v, got %v", tc.line, tc.wantMatch, ok)
		}
		if !ok {
			continue
		}
		if code != tc.wantCode || off != tc.wantOff {
			t.Fatalf("line=%q: want (%q,%d), got (%q,%d)", tc.line, tc.wantCode, tc.wantOff, code, off)
		}
	}
}
