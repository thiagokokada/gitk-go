package backend

import "testing"

func TestParseGitVersionOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want gitVersion
		ok   bool
	}{
		{name: "empty", in: "", ok: false},
		{name: "plain", in: "git version 2.44.0\n", want: gitVersion{major: 2, minor: 44, patch: 0}, ok: true},
		{name: "apple_git", in: "git version 2.39.3 (Apple Git-146)\n", want: gitVersion{major: 2, minor: 39, patch: 3}, ok: true},
		{name: "windows_suffix", in: "git version 2.39.3.windows.1\n", want: gitVersion{major: 2, minor: 39, patch: 3}, ok: true},
		{name: "no_prefix", in: "2.42.1\n", want: gitVersion{major: 2, minor: 42, patch: 1}, ok: true},
		{name: "no_patch", in: "git version 2.42\n", want: gitVersion{major: 2, minor: 42, patch: 0}, ok: true},
		{name: "invalid", in: "git version not-a-version\n", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := parseGitVersionOutput(tt.in)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v (got=%+v)", ok, tt.ok, got)
			}
			if !ok {
				return
			}
			if got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestValidateGitVersionOutput(t *testing.T) {
	t.Parallel()

	old := minGitVersion
	t.Cleanup(func() { minGitVersion = old })
	minGitVersion = gitVersion{major: 2, minor: 23, patch: 0}

	if err := validateGitVersionOutput("git version 2.23.0\n"); err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
	if err := validateGitVersionOutput("git version 2.22.9\n"); err == nil {
		t.Fatal("expected error for old git")
	}
}
