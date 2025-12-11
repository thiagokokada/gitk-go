package gui

import (
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/thiagokokada/gitk-go/internal/git"
)

func TestCommitListColumns(t *testing.T) {
	commit := &object.Commit{
		Hash:    plumbing.NewHash("abcdef1234567890abcdef1234567890abcdef12"),
		Author:  object.Signature{Name: "Alice", Email: "alice@example.com", When: time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)},
		Message: "Subject line\nSecond line",
	}
	entry := &git.Entry{Commit: commit}
	msg, author, when := commitListColumns(entry)
	if !strings.Contains(msg, "abcdef1") || !strings.Contains(msg, "Subject line") {
		t.Fatalf("unexpected commit column: %q", msg)
	}
	if author != "Alice <alice@example.com>" {
		t.Fatalf("unexpected author column: %q", author)
	}
	if when != "2025-01-01 10:00" {
		t.Fatalf("unexpected date column: %q", when)
	}
}

func TestFormatGraphValue(t *testing.T) {
	entry := &git.Entry{Graph: "* |"}
	graph := formatGraphValue(entry, []string{"HEAD -> main", "feature"})
	expected := "* | [HEAD -> main, feature]"
	if graph != expected {
		t.Fatalf("unexpected graph string: %q", graph)
	}

	entry = &git.Entry{}
	graph = formatGraphValue(entry, nil)
	if graph != "*" {
		t.Fatalf("expected fallback graph '*', got %q", graph)
	}
}

func TestFilterEntries(t *testing.T) {
	entries := []*git.Entry{
		{SearchText: "hello world"},
		{SearchText: "feature branch"},
	}
	filtered := filterEntries(entries, "HELLO")
	if len(filtered) != 1 || filtered[0] != entries[0] {
		t.Fatalf("expected first entry match, got %#v", filtered)
	}
	filtered = filterEntries(entries, " ")
	if len(filtered) != len(entries) {
		t.Fatalf("expected no filtering on blank query")
	}
}

func TestStatusSummary(t *testing.T) {
	ctrl := &Controller{
		repoPath: "/repo/path",
		headRef:  "main",
		commits: []*git.Entry{
			{}, {},
		},
		visible: []*git.Entry{
			{},
		},
		hasMore:     true,
		filterValue: "feature",
	}
	summary := ctrl.statusSummary()
	if !strings.Contains(summary, "Showing 1/2") {
		t.Fatalf("unexpected summary counts: %s", summary)
	}
	if !strings.Contains(summary, "filter") && !strings.Contains(summary, "Filter") {
		t.Fatalf("expected filter mention in summary: %s", summary)
	}
	if !strings.Contains(summary, "/repo/path") {
		t.Fatalf("expected repo path in summary: %s", summary)
	}
}

func TestTclListAndEscape(t *testing.T) {
	result := tclList("hello", "a{b}", `path\to`)
	expected := "{hello} {a\\{b\\}} {path\\\\to}"
	if result != expected {
		t.Fatalf("unexpected tcl list: %q", result)
	}
}
