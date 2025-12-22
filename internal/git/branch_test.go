package git

import (
	"slices"
	"testing"

	gitbackend "github.com/thiagokokada/gitk-go/internal/git/backend"
)

func TestLocalBranchNames_SortsDedupesAndReturnsHead(t *testing.T) {
	svc := NewWithBackend(&fakeBackend{
		repoPath: "repo",
		listRefsFunc: func() ([]gitbackend.Ref, error) {
			return []gitbackend.Ref{
				{Kind: gitbackend.RefKindBranch, Name: "z"},
				{Kind: gitbackend.RefKindBranch, Name: "main"},
				{Kind: gitbackend.RefKindBranch, Name: "main"},
				{Kind: gitbackend.RefKindRemoteBranch, Name: "origin/main"},
				{Kind: gitbackend.RefKindTag, Name: "v1"},
			}, nil
		},
		headStateFunc: func() (hash string, headName string, ok bool, err error) {
			return "abc", "main", true, nil
		},
	})

	branches, head, err := svc.LocalBranchNames()
	if err != nil {
		t.Fatalf("LocalBranchNames() error = %v", err)
	}
	if head != "main" {
		t.Fatalf("head = %q, want %q", head, "main")
	}
	if !slices.Equal(branches, []string{"main", "z"}) {
		t.Fatalf("branches = %#v, want %#v", branches, []string{"main", "z"})
	}
}

func TestLocalBranchNames_HeadFallback(t *testing.T) {
	svc := NewWithBackend(&fakeBackend{
		repoPath: "repo",
		listRefsFunc: func() ([]gitbackend.Ref, error) {
			return []gitbackend.Ref{
				{Kind: gitbackend.RefKindBranch, Name: "main"},
			}, nil
		},
		headStateFunc: func() (hash string, headName string, ok bool, err error) {
			return "", "", false, nil
		},
	})

	_, head, err := svc.LocalBranchNames()
	if err != nil {
		t.Fatalf("LocalBranchNames() error = %v", err)
	}
	if head != "HEAD" {
		t.Fatalf("head = %q, want %q", head, "HEAD")
	}
}

func TestSwitchBranch_CallsBackendAndClearsScan(t *testing.T) {
	f := &fakeBackend{
		repoPath: "repo",
		switchBranchFunc: func(branch string) error {
			return nil
		},
	}
	svc := NewWithBackend(f)
	svc.scan = &scanSession{}

	if err := svc.SwitchBranch("feature"); err != nil {
		t.Fatalf("SwitchBranch() error = %v", err)
	}
	if f.lastSwitchBranch != "feature" {
		t.Fatalf("backend branch = %q, want %q", f.lastSwitchBranch, "feature")
	}
	if svc.scan != nil {
		t.Fatalf("expected scan session to be cleared")
	}
}

func TestSwitchBranch_ChangesHead(t *testing.T) {
	dir, _ := createTestRepo(t, 2)

	runGit(t, dir, nil, "checkout", "-b", "feature")
	runGit(t, dir, nil, "commit", "--allow-empty", "-m", "on feature", "--quiet", "--no-gpg-sign")
	runGit(t, dir, nil, "checkout", "main")

	svc, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := svc.SwitchBranch("feature"); err != nil {
		t.Fatalf("SwitchBranch: %v", err)
	}
	_, head, _, err := svc.ScanCommits(0, 1)
	if err != nil {
		t.Fatalf("ScanCommits: %v", err)
	}
	if head != "feature" {
		t.Fatalf("head = %q, want %q", head, "feature")
	}
}
