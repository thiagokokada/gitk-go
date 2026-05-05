package git

import (
	"fmt"
	"strings"
)

func (s *Service) WorktreeDiff(staged bool) (string, []FileSection, error) {
	if s.backend == nil || s.backend.RepoPath() == "" {
		return "", nil, fmt.Errorf("repository root not set")
	}
	diffText, err := s.backend.WorktreeDiffText(staged)
	if err != nil {
		return "", nil, err
	}
	if strings.TrimSpace(diffText) == "" {
		return "", nil, nil
	}

	header := localDiffHeader(staged)
	rendered, sections := buildDiffResult(header, diffText)
	return rendered, sections, nil
}
