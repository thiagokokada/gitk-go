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
		Hash: plumbing.NewHash("abcdef1234567890abcdef1234567890abcdef12"),
		Author: object.Signature{
			Name:  "Alice",
			Email: "alice@example.com",
			When:  time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC),
		},
		Committer: object.Signature{
			Name:  "Alice",
			Email: "alice@example.com",
			When:  time.Date(2025, 1, 2, 9, 30, 0, 0, time.UTC),
		},
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
	if when != "2025-01-02 09:30" {
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
		tree: treeState{
			hasMore: true,
		},
		filter: filterState{
			value: "feature",
		},
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
	expected := "\"hello\" \"a{b}\" \"path\\\\to\""
	if result != expected {
		t.Fatalf("unexpected tcl list: %q", result)
	}
}

func TestBuildTreeRows(t *testing.T) {
	now := time.Date(2025, 2, 1, 12, 0, 0, 0, time.UTC)
	entry1 := &git.Entry{
		Commit: &object.Commit{
			Hash:   plumbing.NewHash("1111111111111111111111111111111111111111"),
			Author: object.Signature{Name: "Alice", Email: "alice@example.com", When: now},
			Committer: object.Signature{
				Name:  "Alice",
				Email: "alice@example.com",
				When:  now,
			},
			Message: "first message",
		},
		Graph: "* |",
	}
	entry2 := &git.Entry{
		Commit: &object.Commit{
			Hash:   plumbing.NewHash("2222222222222222222222222222222222222222"),
			Author: object.Signature{Name: "Bob", Email: "bob@example.com", When: now.Add(-time.Hour)},
			Committer: object.Signature{
				Name:  "Bob",
				Email: "bob@example.com",
				When:  now.Add(-2 * time.Hour),
			},
			Message: "second message line\nmore",
		},
		Graph: "|/",
	}
	labels := map[string][]string{
		entry1.Commit.Hash.String(): {"HEAD -> main"},
	}
	rows := buildTreeRows([]*git.Entry{entry1, entry2}, labels)
	if len(rows) != 2 {
		t.Fatalf("expected two rows, got %d", len(rows))
	}
	if rows[0].ID != "0" || rows[1].ID != "1" {
		t.Fatalf("unexpected row ids: %#v", rows)
	}
	if rows[0].Graph != "* | [HEAD -> main]" {
		t.Fatalf("unexpected graph: %q", rows[0].Graph)
	}
	if !strings.Contains(rows[0].Commit, "first message") {
		t.Fatalf("missing commit message in row: %q", rows[0].Commit)
	}
	if !strings.Contains(rows[1].Author, "Bob") || !strings.Contains(rows[1].Author, "bob@example.com") {
		t.Fatalf("unexpected author column: %q", rows[1].Author)
	}
	if !strings.Contains(rows[1].Date, "2025-02-01 10:00") {
		t.Fatalf("unexpected date column: %q", rows[1].Date)
	}
}

func TestThemePreferenceFromString(t *testing.T) {
	if got := ThemePreferenceFromString("Dark"); got != ThemeDark {
		t.Fatalf("expected ThemeDark, got %v", got)
	}
	if got := ThemePreferenceFromString("light"); got != ThemeLight {
		t.Fatalf("expected ThemeLight, got %v", got)
	}
	if got := ThemePreferenceFromString("other"); got != ThemeAuto {
		t.Fatalf("expected ThemeAuto fallback, got %v", got)
	}
}

func TestPaletteForPreference(t *testing.T) {
	orig := detectDarkMode
	t.Cleanup(func() { detectDarkMode = orig })

	detectDarkMode = func() (bool, error) { return true, nil }
	if pal := paletteForPreference(ThemeAuto); pal.ThemeName != darkPalette.ThemeName {
		t.Fatalf("expected dark palette for auto detection, got %+v", pal)
	}

	detectDarkMode = func() (bool, error) { return false, nil }
	if pal := paletteForPreference(ThemeAuto); pal.ThemeName != lightPalette.ThemeName {
		t.Fatalf("expected light palette fallback on detection failure, got %+v", pal)
	}

	if pal := paletteForPreference(ThemeLight); pal.ThemeName != lightPalette.ThemeName {
		t.Fatalf("explicit light preference should use light palette, got %+v", pal)
	}
	if pal := paletteForPreference(ThemeDark); pal.ThemeName != darkPalette.ThemeName {
		t.Fatalf("explicit dark preference should use dark palette, got %+v", pal)
	}
}
