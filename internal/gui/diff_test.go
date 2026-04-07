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

func TestDiffViewModel(t *testing.T) {
	sections := []git.FileSection{
		{Path: "a.go", Line: 5},
		{Path: "b.go", Line: 10},
	}
	model := newDiffViewModel(sections)
	if len(model.sections) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(model.sections))
	}
	if model.sections[0].Path != diffCommitSectionLabel || model.sections[0].Line != 1 {
		t.Fatalf("unexpected commit section: %#v", model.sections[0])
	}
	if model.sections[1].Path != "a.go" || model.sections[1].Line != 5 {
		t.Fatalf("unexpected first file section: %#v", model.sections[1])
	}
	if model.sections[2].Path != "b.go" || model.sections[2].Line != 10 {
		t.Fatalf("unexpected second file section: %#v", model.sections[2])
	}
	if len(model.labels) != 3 || model.labels[0] != diffCommitSectionLabel || model.labels[2] != "b.go" {
		t.Fatalf("unexpected labels: %#v", model.labels)
	}
}

func TestDiffViewModelNoSections(t *testing.T) {
	model := newDiffViewModel(nil)
	if len(model.sections) != 0 {
		t.Fatalf("expected no sections, got %#v", model.sections)
	}
	if len(model.labels) != 0 {
		t.Fatalf("expected no labels, got %#v", model.labels)
	}
}

func TestDiffSectionLine(t *testing.T) {
	sections := newDiffViewModel([]git.FileSection{{Path: "a.go", Line: 5}}).sections
	if line, ok := diffSectionLine(sections, 1); !ok || line != 5 {
		t.Fatalf("expected line 5 for index 1, got (%d,%v)", line, ok)
	}
	if _, ok := diffSectionLine(sections, -1); ok {
		t.Fatalf("expected invalid index to return ok=false")
	}
}

func TestDiffSectionIndexForLine(t *testing.T) {
	sections := newDiffViewModel([]git.FileSection{
		{Path: "a.go", Line: 5},
		{Path: "b.go", Line: 10},
	}).sections
	tests := []struct {
		line    int
		wantIdx int
		wantOK  bool
	}{
		{line: 1, wantIdx: 0, wantOK: true},
		{line: 6, wantIdx: 1, wantOK: true},
		{line: 10, wantIdx: 2, wantOK: true},
		{line: 0, wantIdx: 0, wantOK: true},
	}
	for _, tc := range tests {
		idx, ok := diffSectionIndexForLine(sections, tc.line)
		if ok != tc.wantOK || idx != tc.wantIdx {
			t.Fatalf("line=%d: want (%d,%v), got (%d,%v)", tc.line, tc.wantIdx, tc.wantOK, idx, ok)
		}
	}
}

func TestDiffScrollFraction(t *testing.T) {
	tests := []struct {
		line       int
		totalLines int
		want       float64
	}{
		{line: 1, totalLines: 1, want: 0},
		{line: 1, totalLines: 10, want: 0},
		{line: 10, totalLines: 10, want: 1},
		{line: 20, totalLines: 10, want: 1},
		{line: -1, totalLines: 10, want: 0},
	}
	for _, tc := range tests {
		got := diffScrollFraction(tc.line, tc.totalLines)
		if got != tc.want {
			t.Fatalf("line=%d total=%d: want %v, got %v", tc.line, tc.totalLines, tc.want, got)
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

func TestContainsUnmergedPathMarker(t *testing.T) {
	tests := []struct {
		name string
		diff string
		want bool
	}{
		{name: "empty", diff: "", want: false},
		{name: "normal_diff", diff: "diff --git a/a b/a\n@@ -1 +1 @@\n-a\n+b\n", want: false},
		{name: "plain_unmerged", diff: "* Unmerged path foo/bar.txt\n", want: true},
		{name: "header_then_unmerged", diff: "Local changes\n* Unmerged path foo/bar.txt\n", want: true},
		{
			name: "context_line_with_marker_text",
			diff: strings.Join([]string{
				"diff --git a/a.txt b/a.txt",
				"@@ -1 +1,2 @@",
				" context",
				" * Unmerged path appears in file content",
				"+next",
			}, "\n"),
			want: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := containsUnmergedPathMarker(tc.diff); got != tc.want {
				t.Fatalf("containsUnmergedPathMarker() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestShouldAddInlineDiffActions(t *testing.T) {
	diff := strings.Join([]string{
		"diff --git a/a.txt b/a.txt",
		"@@ -1 +1 @@",
		"-a",
		"+b",
	}, "\n")
	chunks := parseDiffChunks(diff)
	if !shouldAddInlineDiffActions(diff, chunks) {
		t.Fatal("expected inline actions for small regular diff")
	}

	if shouldAddInlineDiffActions("* Unmerged path foo/bar.txt\n", nil) {
		t.Fatal("expected inline actions to be disabled for unmerged diff marker")
	}

	large := make([]diffFileChunk, maxInlineDiffActions+1)
	for i := range large {
		large[i] = diffFileChunk{hunks: []diffHunkChunk{{header: "@@ -1 +1 @@"}}}
	}
	if shouldAddInlineDiffActions(diff, large) {
		t.Fatal("expected inline actions to be disabled when button count exceeds limit")
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

func TestParseDiffChunksAndBuildPatches(t *testing.T) {
	diff := strings.Join([]string{
		"diff --git a/a.txt b/a.txt",
		"index 111..222 100644",
		"--- a/a.txt",
		"+++ b/a.txt",
		"@@ -1,2 +1,2 @@",
		"-line1",
		"+line1a",
		" line2",
		"@@ -5,1 +5,2 @@",
		"+line5a",
	}, "\n")
	chunks := parseDiffChunks(diff)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	chunk := chunks[0]
	if chunk.lineNo != 1 {
		t.Fatalf("expected file line 1, got %d", chunk.lineNo)
	}
	if len(chunk.hunks) != 2 {
		t.Fatalf("expected 2 hunks, got %d", len(chunk.hunks))
	}
	if chunk.hunks[0].lineNo != 5 {
		t.Fatalf("expected hunk line 5, got %d", chunk.hunks[0].lineNo)
	}
	filePatch, ok := buildFilePatch(chunk)
	if !ok {
		t.Fatalf("expected file patch")
	}
	if filePatch != diff+"\n" {
		t.Fatalf("file patch mismatch:\nwant:\n%s\ngot:\n%s", diff+"\n", filePatch)
	}
	hunkPatch, ok := buildHunkPatch(chunk, chunk.hunks[0])
	if !ok {
		t.Fatalf("expected hunk patch")
	}
	wantHunk := strings.Join([]string{
		"diff --git a/a.txt b/a.txt",
		"index 111..222 100644",
		"--- a/a.txt",
		"+++ b/a.txt",
		"@@ -1,2 +1,2 @@",
		"-line1",
		"+line1a",
		" line2",
		"",
	}, "\n")
	if hunkPatch != wantHunk {
		t.Fatalf("hunk patch mismatch:\nwant:\n%s\ngot:\n%s", wantHunk, hunkPatch)
	}
}

func TestRemovePatchFromDiffText(t *testing.T) {
	diff := strings.Join([]string{
		"Local uncommitted changes, not checked in to index",
		"diff --git a/a.txt b/a.txt",
		"index 111..222 100644",
		"--- a/a.txt",
		"+++ b/a.txt",
		"@@ -1,2 +1,2 @@",
		"-line1",
		"+line1a",
		" line2",
		"@@ -5,1 +5,2 @@",
		"+line5a",
		"",
	}, "\n")
	chunks := parseDiffChunks(diff)
	if len(chunks) != 1 || len(chunks[0].hunks) != 2 {
		t.Fatalf("unexpected diff chunks")
	}
	patch, ok := buildHunkPatch(chunks[0], chunks[0].hunks[0])
	if !ok {
		t.Fatalf("expected hunk patch")
	}
	updated, sections, ok := removePatchFromDiffText(diff, patch)
	if !ok {
		t.Fatalf("expected patch removal")
	}
	if strings.Contains(updated, "@@ -1,2 +1,2 @@") {
		t.Fatalf("expected first hunk to be removed")
	}
	if len(sections) != 1 || sections[0].Path != "a.txt" {
		t.Fatalf("unexpected sections: %#v", sections)
	}
}
