//go:build gitcli

package git

import (
	"fmt"

	gitlib "github.com/go-git/go-git/v5"
)

func (s *Service) repoForWorktree() (*gitlib.Repository, error) {
	if s.repo.Repository == nil {
		return nil, fmt.Errorf("repository not initialized")
	}
	return s.repo.Repository, nil
}
