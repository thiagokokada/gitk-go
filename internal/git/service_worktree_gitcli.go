package git

import (
	"fmt"
	"strings"
)

func (s *Service) WorktreeDiff(staged bool) (string, []FileSection, error) {
	if s.repo.path == "" {
		return "", nil, fmt.Errorf("repository root not set")
	}
	args := []string{"diff", "--no-color"}
	if staged {
		args = append(args, "--cached")
	}
	diffText, err := s.runGitCommand(args, true, "git diff")
	if err != nil {
		return "", nil, err
	}
	if strings.TrimSpace(diffText) == "" {
		return "", nil, nil
	}

	header := localDiffHeader(staged)
	if !strings.HasSuffix(header, "\n") {
		header += "\n"
	}
	var b strings.Builder
	b.WriteString(header)
	b.WriteString(diffText)
	if !strings.HasSuffix(diffText, "\n") {
		b.WriteByte('\n')
	}
	lineOffset := strings.Count(header, "\n")
	sections := parseGitDiffSections(diffText, lineOffset)
	return b.String(), sections, nil
}
