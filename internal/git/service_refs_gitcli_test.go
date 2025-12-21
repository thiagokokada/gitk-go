//go:build gitcli

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

	got, err := parseRefLabelsFromShowRef(in)
	if err != nil {
		t.Fatalf("parseRefLabelsFromShowRef() error = %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("unexpected key count: got %d want 2", len(got))
	}

	labels1 := got[commit1]
	if labels1 == nil {
		t.Fatalf("missing commit key %s", commit1)
	}
	if !contains(labels1, "main") {
		t.Fatalf("expected branch label main in %+v", labels1)
	}
	if !contains(labels1, "origin/main") {
		t.Fatalf("expected remote label origin/main in %+v", labels1)
	}
	if contains(labels1, "origin/HEAD") {
		t.Fatalf("did not expect origin/HEAD in %+v", labels1)
	}
	if !contains(labels1, "tag: v2.0") {
		t.Fatalf("expected peeled tag label tag: v2.0 in %+v", labels1)
	}

	labels2 := got[commit2]
	if labels2 == nil {
		t.Fatalf("missing commit key %s", commit2)
	}
	if !contains(labels2, "tag: v1.0") {
		t.Fatalf("expected tag label tag: v1.0 in %+v", labels2)
	}
}

func TestParseRefLabelsFromShowRef_InvalidLine(t *testing.T) {
	t.Parallel()

	_, err := parseRefLabelsFromShowRef("refs/heads/main\n")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBranchLabels_GitCLI_IncludesHEADAndBranch(t *testing.T) {
	dir, hashes := createTestRepo(t, 1)
	svc, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	headRef, err := svc.repo.Head()
	if err != nil {
		t.Fatalf("Head: %v", err)
	}
	branch := ""
	if headRef.Name().IsBranch() {
		branch = headRef.Name().Short()
	}
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
