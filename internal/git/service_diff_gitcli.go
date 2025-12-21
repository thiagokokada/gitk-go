package git

import (
	"fmt"
	"strings"
)

func (s *Service) Diff(commit *Commit) (string, []FileSection, error) {
	if commit == nil {
		return "", nil, fmt.Errorf("commit not specified")
	}
	header := FormatCommitHeader(commit)
	diffText, err := s.commitDiffText(commit)
	if err != nil {
		return "", nil, err
	}
	if strings.TrimSpace(diffText) == "" {
		return header + "\nNo file level changes.", nil, nil
	}
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

func (s *Service) commitDiffText(commit *Commit) (string, error) {
	if commit != nil && len(commit.ParentHashes) > 0 {
		parent := commit.ParentHashes[0]
		return s.runGitCommand(
			[]string{"diff", "--no-color", parent, commit.Hash},
			true,
			"git diff",
		)
	}
	return s.runGitCommand(
		[]string{"show", "--no-color", "--pretty=format:", commit.Hash},
		false,
		"git show",
	)
}
