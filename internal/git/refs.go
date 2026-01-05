package git

import (
	"strings"

	gitbackend "github.com/thiagokokada/gitk-go/internal/git/backend"
)

func (s *Service) BranchLabels() (map[string][]string, error) {
	labels := map[string][]string{}
	if s.backend == nil || s.backend.RepoPath() == "" {
		return labels, nil
	}

	refs, err := s.backend.ListRefs()
	if err != nil {
		return nil, err
	}
	for _, ref := range refs {
		if ref.Hash == "" || ref.Name == "" {
			continue
		}
		if ref.Kind == gitbackend.RefKindRemoteBranch && strings.HasSuffix(ref.Name, "/HEAD") {
			continue
		}
		label := ref.Name
		if ref.Kind == gitbackend.RefKindTag {
			label = "tag: " + ref.Name
		}
		labels[ref.Hash] = append(labels[ref.Hash], label)
	}

	headHash, headName, ok, err := s.backend.HeadState()
	if err != nil {
		return nil, err
	}
	if ok && headHash != "" {
		label := "HEAD"
		if headName != "" && headName != "HEAD" {
			label = "HEAD -> " + headName
		}
		labels[headHash] = append([]string{label}, labels[headHash]...)
	}
	return labels, nil
}
