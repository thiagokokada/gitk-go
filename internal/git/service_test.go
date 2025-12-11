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

func TestParseGraphLine(t *testing.T) {
	line := "| * 1234567890abcdef1234567890abcdef12345678"
	row, ok := parseGraphLine(line)
	if !ok {
		t.Fatalf("expected parse success")
	}
	if row.graph != "| *" {
		t.Fatalf("unexpected graph %q", row.graph)
	}
	if row.hash != "1234567890abcdef1234567890abcdef12345678" {
		t.Fatalf("unexpected hash %q", row.hash)
	}
}

func TestIsHexHash(t *testing.T) {
	valid := "abcdef1234567890abcdef1234567890abcdef12"
	invalid := "xyz123"
	if !isHexHash(valid) {
		t.Fatalf("expected %q to be valid hash", valid)
	}
	if isHexHash(invalid) {
		t.Fatalf("expected %q to be invalid hash", invalid)
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
