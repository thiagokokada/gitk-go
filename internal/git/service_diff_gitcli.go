//go:build gitcli

package git

import (
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/object"
)

func (s *Service) Diff(commit *object.Commit) (string, []FileSection, error) {
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

func (s *Service) commitDiffText(commit *object.Commit) (string, error) {
	if commit.NumParents() > 0 {
		parent, err := commit.Parent(0)
		if err != nil {
			return "", err
		}
		return s.runGitCommand(
			[]string{"diff", "--no-color", parent.Hash.String(), commit.Hash.String()},
			true,
			"git diff",
		)
	}
	return s.runGitCommand(
		[]string{"show", "--no-color", "--pretty=format:", commit.Hash.String()},
		false,
		"git show",
	)
}
