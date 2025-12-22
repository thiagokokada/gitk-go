package git

import (
	"fmt"
	"slices"
	"strings"

	gitbackend "github.com/thiagokokada/gitk-go/internal/git/backend"
)

// LocalBranchNames returns a sorted list of local branch names and the current
// HEAD name when available.
func (s *Service) LocalBranchNames() (branches []string, headName string, err error) {
	if s.backend == nil || s.backend.RepoPath() == "" {
		return nil, "", fmt.Errorf("repository root not set")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	refs, err := s.backend.ListRefs()
	if err != nil {
		return nil, "", err
	}
	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		if ref.Kind != gitbackend.RefKindBranch {
			continue
		}
		name := strings.TrimSpace(ref.Name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		branches = append(branches, name)
	}
	slices.Sort(branches)

	_, headName, ok, err := s.backend.HeadState()
	if err != nil {
		return nil, "", err
	}
	if !ok || strings.TrimSpace(headName) == "" {
		headName = "HEAD"
	}
	return branches, headName, nil
}

func (s *Service) SwitchBranch(branch string) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return fmt.Errorf("branch not specified")
	}
	if s.backend == nil || s.backend.RepoPath() == "" {
		return fmt.Errorf("repository root not set")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.backend.SwitchBranch(branch); err != nil {
		return err
	}
	if s.scan != nil {
		s.scan.close()
		s.scan = nil
	}
	return nil
}
