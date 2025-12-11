package git

import (
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	diff "github.com/go-git/go-git/v5/plumbing/format/diff"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestFormatCommitHeader(t *testing.T) {
	ts := time.Date(2023, 7, 1, 12, 0, 0, 0, time.UTC)
	commit := &object.Commit{
		Hash:    plumbing.NewHash("1234567890abcdef1234567890abcdef12345678"),
		Author:  object.Signature{Name: "Alice", Email: "alice@example.com", When: ts},
		Message: "Subject line\n\nBody line",
	}
	got := FormatCommitHeader(commit)
	if !strings.Contains(got, "commit 1234567890abcdef1234567890abcdef12345678") {
		t.Fatalf("header missing hash: %s", got)
	}
	if !strings.Contains(got, "Author: Alice <alice@example.com>") {
		t.Fatalf("header missing author: %s", got)
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
		Hash:    plumbing.NewHash("abcdef1234567890abcdef1234567890abcdef12"),
		Author:  object.Signature{Name: "Bob", Email: "bob@example.com", When: time.Unix(0, 0)},
		Message: strings.Repeat("x", 200),
	}
	got := formatSummary(commit)
	if len(got) == 0 || !strings.Contains(got, commit.Hash.String()[:7]) {
		t.Fatalf("summary missing hash: %s", got)
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
