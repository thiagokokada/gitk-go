package git

import (
	"strings"
	"testing"
)

func TestParseRefLabelsFromShowRef(t *testing.T) {
	t.Parallel()

	const (
		commit1 = "1111111111111111111111111111111111111111"
		commit2 = "2222222222222222222222222222222222222222"
		tagObj  = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	)

	in := strings.Join([]string{
		commit1 + " refs/heads/main",
		commit1 + " refs/remotes/origin/main",
		commit1 + " refs/remotes/origin/HEAD",
		commit2 + " refs/tags/v1.0",
		tagObj + " refs/tags/v2.0",
		commit1 + " refs/tags/v2.0^{}",
		"",
	}, "\n")

	got, err := parseRefsFromShowRef(in)
	if err != nil {
		t.Fatalf("parseRefsFromShowRef() error = %v", err)
	}

	if len(got) != 5 {
		t.Fatalf("unexpected ref count: got %d want 5", len(got))
	}

	assertHasRef(t, got, Ref{Hash: commit1, Kind: RefKindBranch, Name: "main"})
	assertHasRef(t, got, Ref{Hash: commit1, Kind: RefKindRemoteBranch, Name: "origin/main"})
	assertHasRef(t, got, Ref{Hash: commit1, Kind: RefKindRemoteBranch, Name: "origin/HEAD"})
	assertHasRef(t, got, Ref{Hash: commit2, Kind: RefKindTag, Name: "v1.0"})
	// v2.0 should use the peeled hash.
	assertHasRef(t, got, Ref{Hash: commit1, Kind: RefKindTag, Name: "v2.0"})
}

func TestParseRefsFromShowRef_InvalidLine(t *testing.T) {
	t.Parallel()

	_, err := parseRefsFromShowRef("refs/heads/main\n")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBranchLabels_IncludesHEADAndBranch(t *testing.T) {
	dir, hashes := createTestRepo(t, 1)
	svc, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	branch := runGit(t, dir, nil, "symbolic-ref", "-q", "--short", "HEAD")
	wantHead := "HEAD"
	if branch != "" {
		wantHead = "HEAD -> " + branch
	}

	labels, err := svc.BranchLabels()
	if err != nil {
		t.Fatalf("BranchLabels: %v", err)
	}

	key := hashes[0]
	vals := labels[key]
	if len(vals) == 0 {
		t.Fatalf("expected labels for %s", key)
	}
	if vals[0] != wantHead {
		t.Fatalf("expected first label %q, got %q (all=%+v)", wantHead, vals[0], vals)
	}
	if branch != "" && !contains(vals, branch) {
		t.Fatalf("expected branch label %q in %+v", branch, vals)
	}
}

func contains(vals []string, want string) bool {
	for _, v := range vals {
		if v == want {
			return true
		}
	}
	return false
}

func assertHasRef(t *testing.T, refs []Ref, want Ref) {
	t.Helper()
	for _, got := range refs {
		if got.Hash == want.Hash && got.Kind == want.Kind && got.Name == want.Name {
			return
		}
	}
	t.Fatalf("missing ref: %+v (got=%+v)", want, refs)
}
