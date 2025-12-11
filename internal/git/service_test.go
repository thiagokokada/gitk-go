package git

import (
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
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
	a := plumbing.NewHash("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	b := plumbing.NewHash("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	c := plumbing.NewHash("cccccccccccccccccccccccccccccccccccccccc")

	builder := newGraphBuilder()
	lineA := builder.Line(&object.Commit{Hash: a, ParentHashes: []plumbing.Hash{b}})
	if lineA != "*" {
		t.Fatalf("unexpected graph for first commit: %q", lineA)
	}
	lineB := builder.Line(&object.Commit{Hash: b, ParentHashes: []plumbing.Hash{c}})
	if lineB != "*" {
		t.Fatalf("unexpected graph after advancing: %q", lineB)
	}
	lineC := builder.Line(&object.Commit{Hash: c})
	if lineC != "*" {
		t.Fatalf("unexpected graph for root commit: %q", lineC)
	}
}
