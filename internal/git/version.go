package git

import gitbackend "github.com/thiagokokada/gitk-go/internal/git/backend"

func GitVersion() (string, error) {
	return gitbackend.GitVersion()
}

func MinGitVersion() string {
	return gitbackend.MinGitVersion()
}
