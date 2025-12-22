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

	got := parseGitDiffSections(diffText, 5)
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
