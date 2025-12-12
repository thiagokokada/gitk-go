//go:build !gitcli

package git

import gitlib "github.com/go-git/go-git/v5"

func (s *Service) LocalChanges() (LocalChanges, error) {
	var res LocalChanges
	if s.repo.Repository == nil {
		return res, nil
	}
	s.repo.mu.Lock()
	defer s.repo.mu.Unlock()
	wt, err := s.repo.Worktree()
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
