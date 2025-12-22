package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFormatCommitHeader(t *testing.T) {
	ts := time.Date(2023, 7, 1, 12, 0, 0, 0, time.UTC)
	commit := &Commit{
		Hash:   "1234567890abcdef1234567890abcdef12345678",
		Author: Signature{Name: "Alice", Email: "alice@example.com", When: ts},
		Committer: Signature{
			Name:  "Bob",
			Email: "bob@example.com",
			When:  ts.Add(2 * time.Hour),
		},
		Message: "Subject line\n\nBody line",
	}
	got := FormatCommitHeader(commit)
	if !strings.Contains(got, "commit 1234567890abcdef1234567890abcdef12345678") {
		t.Fatalf("header missing hash: %s", got)
	}
	if !strings.Contains(got, "Author: Alice <alice@example.com>  2023-07-01 12:00:00 +0000") {
		t.Fatalf("header missing author: %s", got)
	}
	if !strings.Contains(got, "Committer: Bob <bob@example.com>  2023-07-01 14:00:00 +0000") {
		t.Fatalf("header missing committer info: %s", got)
	}
	if !strings.Contains(got, "Subject line") || !strings.Contains(got, "Body line") {
		t.Fatalf("header missing message lines: %s", got)
	}
}

func TestOpenResolvesWorkdirToRepoRoot(t *testing.T) {
	dir, _ := createTestRepo(t, 1)
	subdir := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	svc, err := Open(subdir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks(dir): %v", err)
	}
	got, err := filepath.EvalSymlinks(svc.RepoPath())
	if err != nil {
		t.Fatalf("EvalSymlinks(svc.RepoPath()): %v", err)
	}
	if got != want {
		t.Fatalf("expected repo root %q, got %q", want, got)
	}
}

func TestNewEntrySearchText(t *testing.T) {
	ts := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	commit := &Commit{
		Hash:    "abcdef1234567890abcdef1234567890abcdef12",
		Author:  Signature{Name: "Bob", Email: "bob@example.com", When: ts},
		Message: "Hello World",
	}
	entry := newEntry(commit)
	if entry.Summary == "" {
		t.Fatalf("summary should not be empty")
	}
	lower := entry.SearchText
	if !strings.Contains(lower, "bob@example.com") || !strings.Contains(lower, "hello world") {
		t.Fatalf("search text not normalized: %s", lower)
	}
}

func TestGraphBuilderLine(t *testing.T) {
	head := strings.Repeat("a", 40)
	parent := strings.Repeat("b", 40)
	merge := strings.Repeat("c", 40)
	other := strings.Repeat("d", 40)

	builder := newGraphBuilder(DefaultGraphMaxColumns)
	lineHead := builder.Line(&Commit{Hash: head, ParentHashes: []string{parent, merge}})
	if lineHead != "*" && lineHead != "* |" {
		t.Fatalf("unexpected graph for merge head: %q", lineHead)
	}
	lineParent := builder.Line(&Commit{Hash: parent, ParentHashes: []string{other}})
	if lineParent != "* |" {
		t.Fatalf("unexpected graph after shifting columns: %q", lineParent)
	}
	lineMerge := builder.Line(&Commit{Hash: merge})
	if lineMerge != "| *" {
		t.Fatalf("unexpected graph for secondary branch: %q", lineMerge)
	}
}

func TestGraphBuilderCapsColumns(t *testing.T) {
	builder := newGraphBuilder(DefaultGraphMaxColumns)
	builder.maxColumns = 50

	parentBase := strings.Repeat("9", 40)
	for i := range 250 {
		hash := fmt.Sprintf("%040x", i+1)
		parent := fmt.Sprintf("%040x", i+1000)
		if parent == parentBase {
			parent = fmt.Sprintf("%040x", i+2000)
		}
		line := builder.Line(&Commit{Hash: hash, ParentHashes: []string{parent}})
		if len(builder.columns) > builder.maxColumns {
			t.Fatalf("columns grew beyond cap: len=%d cap=%d", len(builder.columns), builder.maxColumns)
		}
		if fields := len(strings.Fields(line)); fields > builder.maxColumns {
			t.Fatalf("line fields grew beyond cap: fields=%d cap=%d line=%q", fields, builder.maxColumns, line)
		}
	}
}

func TestFormatSummary(t *testing.T) {
	commit := &Commit{
		Hash: "abcdef1234567890abcdef1234567890abcdef12",
		Author: Signature{
			Name:  "Bob",
			Email: "bob@example.com",
			When:  time.Unix(0, 0),
		},
		Committer: Signature{
			Name:  "Bob",
			Email: "bob@example.com",
			When:  time.Unix(3600, 0),
		},
		Message: strings.Repeat("x", 200),
	}
	got := formatSummary(commit)
	if len(got) == 0 || !strings.Contains(got, commit.Hash[:7]) {
		t.Fatalf("summary missing hash: %s", got)
	}
	committerStamp := commit.Committer.When.Format("2006-01-02 15:04")
	authorStamp := commit.Author.When.Format("2006-01-02 15:04")
	if !strings.Contains(got, committerStamp) {
		t.Fatalf("expected committer timestamp %s in summary: %s", committerStamp, got)
	}
	if strings.Contains(got, authorStamp) && authorStamp != committerStamp {
		t.Fatalf("summary should not use author timestamp when committer differs: %s", got)
	}
	if strings.Contains(got, strings.Repeat("x", 150)) {
		t.Fatalf("summary did not truncate long message: %s", got)
	}
}

func TestBranchLabels_HeadPrependedAndRemoteHeadFilteredAndTagIncluded(t *testing.T) {
	dir, hashes := createTestRepo(t, 1)
	svc, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	headHash := hashes[0]

	runGit(t, dir, nil, "branch", "-M", "main")
	runGit(t, dir, nil, "update-ref", "refs/remotes/origin/main", headHash)
	runGit(t, dir, nil, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	runGit(t, dir, nil, "tag", "v1", headHash)

	labels, err := svc.BranchLabels()
	if err != nil {
		t.Fatalf("BranchLabels: %v", err)
	}
	got := labels[headHash]
	if len(got) == 0 {
		t.Fatalf("expected labels for %s, got none: %+v", headHash, labels)
	}
	if got[0] != "HEAD -> main" {
		t.Fatalf("expected HEAD label to be first, got %q", got[0])
	}
	if !containsString(got, "main") {
		t.Fatalf("expected local branch label %q in %+v", "main", got)
	}
	if !containsString(got, "origin/main") {
		t.Fatalf("expected remote branch label %q in %+v", "origin/main", got)
	}
	if containsString(got, "origin/HEAD") {
		t.Fatalf("did not expect remote HEAD label %q in %+v", "origin/HEAD", got)
	}
	if !containsString(got, "tag: v1") {
		t.Fatalf("expected tag label %q in %+v", "tag: v1", got)
	}
}

func TestScanCommitsPaginationDoesNotSkipExtraCommit(t *testing.T) {
	dir, hashes := createTestRepo(t, 5)
	svc, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	entries1, _, more, err := svc.ScanCommits(0, 2)
	if err != nil {
		t.Fatalf("ScanCommits(0): %v", err)
	}
	if len(entries1) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries1))
	}
	if !more {
		t.Fatalf("expected more commits after first batch")
	}
	if entries1[0].Commit.Hash != hashes[0] || entries1[1].Commit.Hash != hashes[1] {
		t.Fatalf("unexpected first batch hashes: %s %s", entries1[0].Commit.Hash, entries1[1].Commit.Hash)
	}
	if entries1[0].Graph == "" || entries1[1].Graph == "" {
		t.Fatalf("expected graph strings to be populated")
	}

	entries2, _, more, err := svc.ScanCommits(2, 2)
	if err != nil {
		t.Fatalf("ScanCommits(2): %v", err)
	}
	if len(entries2) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries2))
	}
	if !more {
		t.Fatalf("expected more commits after second batch")
	}
	if entries2[0].Commit.Hash != hashes[2] || entries2[1].Commit.Hash != hashes[3] {
		t.Fatalf("unexpected second batch hashes: %s %s", entries2[0].Commit.Hash, entries2[1].Commit.Hash)
	}

	entries3, _, more, err := svc.ScanCommits(4, 2)
	if err != nil {
		t.Fatalf("ScanCommits(4): %v", err)
	}
	if len(entries3) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries3))
	}
	if more {
		t.Fatalf("expected no more commits after last batch")
	}
	if entries3[0].Commit.Hash != hashes[4] {
		t.Fatalf("unexpected last batch hash: %s", entries3[0].Commit.Hash)
	}
}

func TestScanCommitsSkipMismatchResetsSession(t *testing.T) {
	dir, hashes := createTestRepo(t, 3)
	svc, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	entries1, _, _, err := svc.ScanCommits(0, 2)
	if err != nil {
		t.Fatalf("ScanCommits(0): %v", err)
	}
	if len(entries1) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries1))
	}
	entries2, _, _, err := svc.ScanCommits(0, 2)
	if err != nil {
		t.Fatalf("ScanCommits(0) second time: %v", err)
	}
	if len(entries2) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries2))
	}
	if entries2[0].Commit.Hash != hashes[0] || entries2[1].Commit.Hash != hashes[1] {
		t.Fatalf("unexpected hashes after reset: %s %s", entries2[0].Commit.Hash, entries2[1].Commit.Hash)
	}
}

func TestSetGraphMaxColumnsAppliesToSession(t *testing.T) {
	dir, _ := createTestRepo(t, 3)
	svc, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	svc.SetGraphMaxColumns(50)
	if _, _, _, err := svc.ScanCommits(0, 1); err != nil {
		t.Fatalf("ScanCommits: %v", err)
	}
	if svc.scan == nil || svc.scan.graphBuilder == nil {
		t.Fatalf("expected scan graph builder to be initialized")
	}
	if got := svc.scan.graphBuilder.maxColumns; got != 50 {
		t.Fatalf("expected maxColumns=50, got %d", got)
	}

	svc.SetGraphMaxColumns(0)
	if got := svc.scan.graphBuilder.maxColumns; got != DefaultGraphMaxColumns {
		t.Fatalf("expected default maxColumns=%d, got %d", DefaultGraphMaxColumns, got)
	}
}

func containsString(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

func createTestRepo(t *testing.T, commitCount int) (path string, hashesNewestFirst []string) {
	t.Helper()
	if commitCount <= 0 {
		t.Fatalf("commitCount must be positive")
	}
	dir := t.TempDir()
	runGit(t, dir, nil, "init", "-q")
	runGit(t, dir, nil, "config", "user.name", "Alice")
	runGit(t, dir, nil, "config", "user.email", "alice@example.com")

	fileName := "file.txt"
	absFile := filepath.Join(dir, fileName)
	var created []string
	for i := range commitCount {
		content := []byte(fmt.Sprintf("commit %d\n", i))
		if err := os.WriteFile(absFile, content, 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		runGit(t, dir, nil, "add", fileName)
		when := time.Unix(int64(i+1), 0).UTC().Format(time.RFC3339)
		env := []string{
			"GIT_AUTHOR_DATE=" + when,
			"GIT_COMMITTER_DATE=" + when,
		}
		runGit(t, dir, env, "commit", "-m", fmt.Sprintf("commit %d", i), "--quiet", "--no-gpg-sign")
		hash := runGit(t, dir, nil, "rev-parse", "HEAD")
		created = append(created, hash)
	}

	// Standardize on main for tests that expect it.
	runGit(t, dir, nil, "branch", "-M", "main")

	for i := len(created) - 1; i >= 0; i-- {
		hashesNewestFirst = append(hashesNewestFirst, created[i])
	}
	return dir, hashesNewestFirst
}

func runGit(t *testing.T, dir string, extraEnv []string, args ...string) string {
	t.Helper()

	cmdArgs := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", cmdArgs...)
	cmd.Env = append(os.Environ(), extraEnv...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, msg)
		}
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(stdout.String())
}
