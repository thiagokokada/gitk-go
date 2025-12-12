//go:build !gitcli

package git

import gitlib "github.com/go-git/go-git/v5"

func (s *Service) LocalChanges() (LocalChanges, error) {
	var res LocalChanges
	repo, err := s.repoForWorktree()
	if err != nil {
		return res, err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return res, err
	}
	status, err := wt.Status()
	if err != nil {
		return res, err
	}
	for _, st := range status {
		if st.Worktree != gitlib.Unmodified && st.Worktree != gitlib.Untracked {
			res.HasWorktree = true
		}
		if st.Staging != gitlib.Unmodified {
			res.HasStaged = true
		}
		if res.HasWorktree && res.HasStaged {
			break
		}
	}
	return res, nil
}
