package git

import (
	"strings"
	"testing"
	"time"
)

func TestParseGitDiffSections(t *testing.T) {
	t.Parallel()

	diffText := strings.Join([]string{
		"header line",
		"diff --git a/foo.txt b/foo.txt",
		`diff --git "a/space name.txt" "b/space name.txt"`,
		`diff --git "a/quo\"te.txt" "b/quo\"te.txt"`,
		"diff --git a/onlyone",
		"not a diff line",
	}, "\n")

	got := ParseDiffSections(diffText, 5)
	if len(got) != 3 {
		t.Fatalf("expected 3 sections, got %d: %+v", len(got), got)
	}
	if got[0].Path != "foo.txt" || got[0].Line != 7 {
		t.Fatalf("unexpected first section: %+v", got[0])
	}
	if got[1].Path != "space name.txt" || got[1].Line != 8 {
		t.Fatalf("unexpected second section: %+v", got[1])
	}
	if got[2].Path != `quo"te.txt` || got[2].Line != 9 {
		t.Fatalf("unexpected third section: %+v", got[2])
	}
}

func TestDiffPathFromLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		line   string
		want   string
		wantOK bool
	}{
		{line: "diff --git a/foo.txt b/foo.txt", want: "foo.txt", wantOK: true},
		{line: `diff --git "a/space name.txt" "b/space name.txt"`, want: "space name.txt", wantOK: true},
		{line: "diff --git a/onlyone", want: "", wantOK: true},
		{line: "not a diff line", want: "", wantOK: false},
	}

	for _, tc := range tests {
		got, ok := DiffPathFromLine(tc.line)
		if got != tc.want || ok != tc.wantOK {
			t.Fatalf("line=%q: want (%q,%v), got (%q,%v)", tc.line, tc.want, tc.wantOK, got, ok)
		}
	}
}

func TestDiffLineCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		line       string
		wantCode   string
		wantOffset int
		wantOK     bool
	}{
		{line: "+added", wantCode: "added", wantOffset: 1, wantOK: true},
		{line: "-removed", wantCode: "removed", wantOffset: 1, wantOK: true},
		{line: " context", wantCode: "context", wantOffset: 1, wantOK: true},
		{line: "+++ b/foo.txt", wantCode: "", wantOffset: 0, wantOK: false},
		{line: "\\ No newline at end of file", wantCode: "", wantOffset: 0, wantOK: false},
		{line: "@@ -1 +1 @@", wantCode: "", wantOffset: 0, wantOK: false},
	}

	for _, tc := range tests {
		code, offset, ok := DiffLineCode(tc.line)
		if code != tc.wantCode || offset != tc.wantOffset || ok != tc.wantOK {
			t.Fatalf(
				"line=%q: want (%q,%d,%v), got (%q,%d,%v)",
				tc.line,
				tc.wantCode,
				tc.wantOffset,
				tc.wantOK,
				code,
				offset,
				ok,
			)
		}
	}
}

func TestDiff_NoFileLevelChanges(t *testing.T) {
	t.Parallel()

	backend := &fakeBackend{
		repoPath: "repo",
		commitDiffTextFunc: func(commitHash string, parentHash string) (string, error) {
			return "  \n", nil
		},
	}
	svc := NewWithBackend(backend)

	ts := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	commit := &Commit{
		Hash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Author: Signature{
			Name:  "Alice",
			Email: "alice@example.com",
			When:  ts,
		},
		Committer: Signature{
			Name:  "Alice",
			Email: "alice@example.com",
			When:  ts,
		},
		Message: "msg",
	}

	diff, sections, err := svc.Diff(commit)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if !strings.Contains(diff, "No file level changes.") {
		t.Fatalf("expected fallback message, got:\n%s", diff)
	}
	if sections != nil {
		t.Fatalf("expected no sections, got %+v", sections)
	}
}

func TestDiff_PassesParentHashToBackend(t *testing.T) {
	t.Parallel()

	backend := &fakeBackend{
		repoPath: "repo",
		commitDiffTextFunc: func(commitHash string, parentHash string) (string, error) {
			return "diff --git a/foo.txt b/foo.txt\n", nil
		},
	}
	svc := NewWithBackend(backend)

	ts := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	parent := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	commit := &Commit{
		Hash:         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ParentHashes: []string{parent},
		Author:       Signature{Name: "Alice", Email: "alice@example.com", When: ts},
		Committer:    Signature{Name: "Alice", Email: "alice@example.com", When: ts},
		Message:      "msg",
	}

	diff, sections, err := svc.Diff(commit)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if backend.lastCommitHash != commit.Hash || backend.lastParentHash != parent {
		t.Fatalf("backend called with commit=%q parent=%q", backend.lastCommitHash, backend.lastParentHash)
	}
	if !strings.Contains(diff, "diff --git a/foo.txt b/foo.txt") {
		t.Fatalf("expected diff output, got:\n%s", diff)
	}
	if len(sections) != 1 || sections[0].Path != "foo.txt" {
		t.Fatalf("unexpected sections: %+v", sections)
	}
}
