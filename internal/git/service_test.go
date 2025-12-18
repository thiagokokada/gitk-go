package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gitlib "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	diff "github.com/go-git/go-git/v5/plumbing/format/diff"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestFormatCommitHeader(t *testing.T) {
	ts := time.Date(2023, 7, 1, 12, 0, 0, 0, time.UTC)
	commit := &object.Commit{
		Hash:   plumbing.NewHash("1234567890abcdef1234567890abcdef12345678"),
		Author: object.Signature{Name: "Alice", Email: "alice@example.com", When: ts},
		Committer: object.Signature{
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

func TestNewEntrySearchText(t *testing.T) {
	ts := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	commit := &object.Commit{
		Hash:    plumbing.NewHash("abcdef1234567890abcdef1234567890abcdef12"),
		Author:  object.Signature{Name: "Bob", Email: "bob@example.com", When: ts},
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
	head := plumbing.NewHash("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	parent := plumbing.NewHash("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	merge := plumbing.NewHash("cccccccccccccccccccccccccccccccccccccccc")
	other := plumbing.NewHash("dddddddddddddddddddddddddddddddddddddddd")

	builder := newGraphBuilder()
	lineHead := builder.Line(&object.Commit{Hash: head, ParentHashes: []plumbing.Hash{parent, merge}})
	if lineHead != "*" && lineHead != "* |" {
		t.Fatalf("unexpected graph for merge head: %q", lineHead)
	}
	lineParent := builder.Line(&object.Commit{Hash: parent, ParentHashes: []plumbing.Hash{other}})
	if lineParent != "* |" {
		t.Fatalf("unexpected graph after shifting columns: %q", lineParent)
	}
	lineMerge := builder.Line(&object.Commit{Hash: merge})
	if lineMerge != "| *" {
		t.Fatalf("unexpected graph for secondary branch: %q", lineMerge)
	}
}

func TestFormatSummary(t *testing.T) {
	commit := &object.Commit{
		Hash: plumbing.NewHash("abcdef1234567890abcdef1234567890abcdef12"),
		Author: object.Signature{
			Name:  "Bob",
			Email: "bob@example.com",
			When:  time.Unix(0, 0),
		},
		Committer: object.Signature{
			Name:  "Bob",
			Email: "bob@example.com",
			When:  time.Unix(3600, 0),
		},
		Message: strings.Repeat("x", 200),
	}
	got := formatSummary(commit)
	if len(got) == 0 || !strings.Contains(got, commit.Hash.String()[:7]) {
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

func TestFilePatchPath(t *testing.T) {
	fp := testPatch{
		from: testFile{path: "old.txt"},
		to:   testFile{path: "new.txt"},
	}
	if got := filePatchPath(fp); got != "new.txt" {
		t.Fatalf("expected new path, got %q", got)
	}
	fp = testPatch{
		from: testFile{path: "old.txt"},
	}
	if got := filePatchPath(fp); got != "old.txt" {
		t.Fatalf("expected old path fallback, got %q", got)
	}
}

func TestBranchLabels_HeadPrependedAndRemoteHeadFiltered(t *testing.T) {
	dir, hashes := createTestRepo(t, 1)
	svc, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	headHash := plumbing.NewHash(hashes[0])

	mainRef := plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), headHash)
	if err := svc.repo.Storer.SetReference(mainRef); err != nil {
		t.Fatalf("SetReference(main): %v", err)
	}
	if err := svc.repo.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, mainRef.Name())); err != nil {
		t.Fatalf("SetReference(HEAD): %v", err)
	}
	if err := svc.repo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewRemoteReferenceName("origin", "main"), headHash)); err != nil {
		t.Fatalf("SetReference(origin/main): %v", err)
	}
	if err := svc.repo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewRemoteReferenceName("origin", "HEAD"), headHash)); err != nil {
		t.Fatalf("SetReference(origin/HEAD): %v", err)
	}
	if err := svc.repo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewTagReferenceName("v1"), headHash)); err != nil {
		t.Fatalf("SetReference(tag): %v", err)
	}

	labels, err := svc.BranchLabels()
	if err != nil {
		t.Fatalf("BranchLabels: %v", err)
	}
	key := headHash.String()
	got := labels[key]
	if len(got) == 0 {
		t.Fatalf("expected labels for %s, got none: %+v", key, labels)
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
	if containsString(got, "v1") {
		t.Fatalf("did not expect tag label %q in %+v", "v1", got)
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

func TestRenderPatch_NilPatch(t *testing.T) {
	text, sections, err := renderPatch("", nil)
	if err != nil {
		t.Fatalf("renderPatch() error = %v", err)
	}
	if text != "No changes." {
		t.Fatalf("expected %q, got %q", "No changes.", text)
	}
	if sections != nil {
		t.Fatalf("expected nil sections, got %+v", sections)
	}

	text, sections, err = renderPatch("Header", nil)
	if err != nil {
		t.Fatalf("renderPatch() error = %v", err)
	}
	if text != "Header\n" {
		t.Fatalf("expected %q, got %q", "Header\n", text)
	}
	if sections != nil {
		t.Fatalf("expected nil sections, got %+v", sections)
	}
}

func TestRenderPatch_EmptyFilePatches(t *testing.T) {
	text, sections, err := renderPatch("Header", emptyFilePatches{})
	if err != nil {
		t.Fatalf("renderPatch() error = %v", err)
	}
	if text != "Header\nNo changes.\n" {
		t.Fatalf("expected %q, got %q", "Header\nNo changes.\n", text)
	}
	if sections != nil {
		t.Fatalf("expected nil sections, got %+v", sections)
	}
}

func TestRenderPatch_SectionsMatchOutput(t *testing.T) {
	dir, hashes := createTestRepo(t, 2)
	repo, err := gitlib.PlainOpen(dir)
	if err != nil {
		t.Fatalf("PlainOpen: %v", err)
	}
	newest, err := repo.CommitObject(plumbing.NewHash(hashes[0]))
	if err != nil {
		t.Fatalf("CommitObject(newest): %v", err)
	}
	parent, err := repo.CommitObject(plumbing.NewHash(hashes[1]))
	if err != nil {
		t.Fatalf("CommitObject(parent): %v", err)
	}
	currentTree, err := newest.Tree()
	if err != nil {
		t.Fatalf("Tree(current): %v", err)
	}
	parentTree, err := parent.Tree()
	if err != nil {
		t.Fatalf("Tree(parent): %v", err)
	}
	changes, err := object.DiffTree(parentTree, currentTree)
	if err != nil {
		t.Fatalf("DiffTree: %v", err)
	}
	patch, err := changes.Patch()
	if err != nil {
		t.Fatalf("Patch: %v", err)
	}

	text, sections, err := renderPatch("Header", patch)
	if err != nil {
		t.Fatalf("renderPatch() error = %v", err)
	}
	if len(sections) == 0 {
		t.Fatalf("expected sections, got none")
	}

	lineByPath := map[string]int{}
	for i, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, "diff --git ") {
			continue
		}
		path := parseGitDiffPath(line)
		if path == "" {
			continue
		}
		lineByPath[path] = i + 1
	}
	if len(lineByPath) == 0 {
		t.Fatalf("expected diff headers in output, got: %q", text)
	}
	for _, sec := range sections {
		wantLine, ok := lineByPath[sec.Path]
		if !ok {
			t.Fatalf("missing diff header for section path %q in output: %+v", sec.Path, lineByPath)
		}
		if sec.Line != wantLine {
			t.Fatalf("section line mismatch for %q: got %d want %d", sec.Path, sec.Line, wantLine)
		}
	}
}

type emptyFilePatches struct{}

func (emptyFilePatches) FilePatches() []diff.FilePatch { return nil }

type testFile struct {
	path string
}

func (f testFile) Hash() plumbing.Hash     { return plumbing.ZeroHash }
func (f testFile) Mode() filemode.FileMode { return 0 }
func (f testFile) Path() string            { return f.path }

type testPatch struct {
	from diff.File
	to   diff.File
}

func (p testPatch) Files() (diff.File, diff.File) { return p.from, p.to }
func (p testPatch) IsBinary() bool                { return false }
func (p testPatch) Chunks() []diff.Chunk          { return nil }
func (p testPatch) Message() string               { return "" }
func (p testPatch) IsRename() bool                { return false }

func createTestRepo(t *testing.T, commitCount int) (path string, hashesNewestFirst []string) {
	t.Helper()
	if commitCount <= 0 {
		t.Fatalf("commitCount must be positive")
	}
	dir := t.TempDir()
	repo, err := gitlib.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}
	fileName := "file.txt"
	absFile := filepath.Join(dir, fileName)
	var created []string
	for i := range commitCount {
		content := []byte(fmt.Sprintf("commit %d\n", i))
		if err := os.WriteFile(absFile, content, 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if _, err := wt.Add(fileName); err != nil {
			t.Fatalf("Add: %v", err)
		}
		when := time.Unix(int64(i+1), 0).UTC()
		hash, err := wt.Commit(fmt.Sprintf("commit %d", i), &gitlib.CommitOptions{
			Author:    &object.Signature{Name: "Alice", Email: "alice@example.com", When: when},
			Committer: &object.Signature{Name: "Alice", Email: "alice@example.com", When: when},
		})
		if err != nil {
			t.Fatalf("Commit: %v", err)
		}
		created = append(created, hash.String())
	}
	for i := len(created) - 1; i >= 0; i-- {
		hashesNewestFirst = append(hashesNewestFirst, created[i])
	}
	return dir, hashesNewestFirst
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
	if entries1[0].Commit.Hash.String() != hashes[0] || entries1[1].Commit.Hash.String() != hashes[1] {
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
	if entries2[0].Commit.Hash.String() != hashes[2] || entries2[1].Commit.Hash.String() != hashes[3] {
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
	if entries3[0].Commit.Hash.String() != hashes[4] {
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
	if entries2[0].Commit.Hash.String() != hashes[0] || entries2[1].Commit.Hash.String() != hashes[1] {
		t.Fatalf("unexpected hashes after reset: %s %s", entries2[0].Commit.Hash, entries2[1].Commit.Hash)
	}
}
