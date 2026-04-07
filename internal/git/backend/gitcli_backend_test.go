package backend

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseStatusPorcelainV2(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want LocalChanges
	}{
		{name: "empty", in: "", want: LocalChanges{}},
		{
			name: "worktree_only",
			in:   "1 .M N... 100644 100644 100644 abcdef0 abcdef0 path.txt\n",
			want: LocalChanges{HasWorktree: true},
		},
		{
			name: "staged_only",
			in:   "1 M. N... 100644 100644 100644 abcdef0 abcdef0 path.txt\n",
			want: LocalChanges{HasStaged: true},
		},
		{
			name: "both",
			in:   "1 MM N... 100644 100644 100644 abcdef0 abcdef0 path.txt\n",
			want: LocalChanges{HasWorktree: true, HasStaged: true},
		},
		{
			name: "unmerged_counts_as_both",
			in:   "u UU N... 100644 100644 100644 abcdef0 abcdef0 path.txt\n",
			want: LocalChanges{HasWorktree: true, HasStaged: true, HasUnmerged: true},
		},
		{
			name: "deleted_by_us_counts_as_unmerged",
			in:   "u DU N... 100644 000000 100644 100644 abcdef0 0000000 abcdef1 path.txt\n",
			want: LocalChanges{HasWorktree: true, HasStaged: true, HasUnmerged: true},
		},
		{
			name: "untracked_counts_as_worktree",
			in:   "? untracked.txt\n",
			want: LocalChanges{HasWorktree: true},
		},
		{
			name: "ignored_ignored",
			in:   "! ignored.txt\n",
			want: LocalChanges{},
		},
		{
			name: "short_lines_ignored",
			in:   "1\n1 .\n1 .M\n?\n",
			want: LocalChanges{HasWorktree: true},
		},
		{
			name: "scans_until_both",
			in: strings.Join([]string{
				"1 .M N... 100644 100644 100644 abcdef0 abcdef0 a.txt",
				"1 M. N... 100644 100644 100644 abcdef0 abcdef0 b.txt",
				"1 .. N... 100644 100644 100644 abcdef0 abcdef0 c.txt",
			}, "\n") + "\n",
			want: LocalChanges{HasWorktree: true, HasStaged: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseStatusPorcelainV2(strings.NewReader(tt.in))
			if err != nil {
				t.Fatalf("parseStatusPorcelainV2() error = %v", err)
			}
			if got.HasWorktree != tt.want.HasWorktree || got.HasStaged != tt.want.HasStaged {
				t.Fatalf("parseStatusPorcelainV2() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseStatusPorcelainV2_Error(t *testing.T) {
	t.Parallel()

	_, err := parseStatusPorcelainV2(failingReader{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGitCLI_LocalChangesStatusAndWorktreeDiffIncludeUntracked(t *testing.T) {
	dir := initBackendTestRepo(t)
	const untrackedPath = "new file.txt"
	if err := os.WriteFile(filepath.Join(dir, untrackedPath), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cli := &gitCLI{path: dir}
	status, err := cli.LocalChangesStatus()
	if err != nil {
		t.Fatalf("LocalChangesStatus: %v", err)
	}
	if !status.HasWorktree || status.HasStaged {
		t.Fatalf("LocalChangesStatus = %+v, want worktree-only", status)
	}

	diffText, err := cli.WorktreeDiffText(false)
	if err != nil {
		t.Fatalf("WorktreeDiffText(false): %v", err)
	}
	if !strings.Contains(diffText, "diff --git a/"+untrackedPath+" b/"+untrackedPath) {
		t.Fatalf("diff missing untracked file header: %q", diffText)
	}
	if !strings.Contains(diffText, "+++ b/"+untrackedPath) {
		t.Fatalf("diff missing untracked file path: %q", diffText)
	}
	if !strings.Contains(diffText, "+hello") {
		t.Fatalf("diff missing untracked file contents: %q", diffText)
	}
}

func TestConcatDiffText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		parts []string
		want  string
	}{
		{
			name:  "empty",
			parts: nil,
			want:  "",
		},
		{
			name:  "skips_blank_parts",
			parts: []string{"", " \n\t", "diff --git a/a.txt b/a.txt\n"},
			want:  "diff --git a/a.txt b/a.txt\n",
		},
		{
			name:  "adds_separator_newline",
			parts: []string{"diff --git a/a.txt b/a.txt", "diff --git a/b.txt b/b.txt\n"},
			want:  "diff --git a/a.txt b/a.txt\ndiff --git a/b.txt b/b.txt\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := concatDiffText(tt.parts...); got != tt.want {
				t.Fatalf("concatDiffText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseRefsFromShowRef(t *testing.T) {
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

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errors.New("boom")
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

func initBackendTestRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	runGitBackend(t, dir, "init", "-q")
	runGitBackend(t, dir, "config", "user.name", "Alice")
	runGitBackend(t, dir, "config", "user.email", "alice@example.com")
	if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("tracked\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	runGitBackend(t, dir, "add", "tracked.txt")
	runGitBackend(t, dir, "commit", "-m", "init", "--quiet", "--no-gpg-sign")
	return dir
}

func runGitBackend(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmdArgs := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", cmdArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, msg)
		}
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(stdout.String())
}
