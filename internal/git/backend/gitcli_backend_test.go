package backend

import (
	"errors"
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
			want: LocalChanges{HasWorktree: true, HasStaged: true},
		},
		{
			name: "untracked_ignored",
			in:   "? untracked.txt\n",
			want: LocalChanges{},
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
