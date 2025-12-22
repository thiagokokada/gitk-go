package git

import "testing"

func TestWorktreeDiff_Empty(t *testing.T) {
	t.Parallel()

	backend := &fakeBackend{
		repoPath: "repo",
		worktreeDiffTextFunc: func(staged bool) (string, error) {
			return " \n", nil
		},
	}
	svc := NewWithBackend(backend)

	diff, sections, err := svc.WorktreeDiff(false)
	if err != nil {
		t.Fatalf("WorktreeDiff: %v", err)
	}
	if diff != "" || sections != nil {
		t.Fatalf("expected empty result, got diff=%q sections=%+v", diff, sections)
	}
}

func TestWorktreeDiff_PrependsHeaderAndOffsetsSections(t *testing.T) {
	t.Parallel()

	const diffText = "diff --git a/foo.txt b/foo.txt\n"
	backend := &fakeBackend{
		repoPath: "repo",
		worktreeDiffTextFunc: func(staged bool) (string, error) {
			if !staged {
				t.Fatalf("expected staged=true")
			}
			return diffText, nil
		},
	}
	svc := NewWithBackend(backend)

	diff, sections, err := svc.WorktreeDiff(true)
	if err != nil {
		t.Fatalf("WorktreeDiff: %v", err)
	}
	if diff == "" {
		t.Fatalf("expected diff output")
	}
	if len(sections) != 1 || sections[0].Path != "foo.txt" {
		t.Fatalf("unexpected sections: %+v", sections)
	}
	// WorktreeDiff adds a 1-line header before the diff text.
	if sections[0].Line != 2 {
		t.Fatalf("expected section line=2, got %d", sections[0].Line)
	}
}
