//go:build !gitcli

package git

import (
	"fmt"

	gitlib "github.com/go-git/go-git/v5"
)

func (s *Service) repoForWorktree() (*gitlib.Repository, error) {
	if s.repo.path == "" {
		return nil, fmt.Errorf("repository not initialized")
	}
	repo, err := gitlib.PlainOpenWithOptions(s.repo.path, &gitlib.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return nil, err
	}
	return repo, nil
}
