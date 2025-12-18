package git

import (
	"strings"
	"testing"
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
