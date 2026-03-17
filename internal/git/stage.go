package git

import (
	"fmt"
	"strings"
)

func (s *Service) StagePatchToIndex(patch string, reverse bool) error {
	if s.backend == nil || s.backend.RepoPath() == "" {
		return fmt.Errorf("repository root not set")
	}
	if strings.TrimSpace(patch) == "" {
		return fmt.Errorf("patch not specified")
	}
	return s.backend.ApplyPatchToIndex(patch, reverse)
}
