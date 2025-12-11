//go:build !gitcli

package git

import (
	"github.com/go-git/go-git/v5/plumbing/object"
)

func (s *Service) Diff(commit *object.Commit) (string, []FileSection, error) {
	currentTree, err := commit.Tree()
	if err != nil {
		return "", nil, err
	}
	var parentTree *object.Tree
	if commit.NumParents() > 0 {
		parent, err := commit.Parent(0)
		if err != nil {
			return "", nil, err
		}
		parentTree, err = parent.Tree()
		if err != nil {
			return "", nil, err
		}
	}
	changes, err := object.DiffTree(parentTree, currentTree)
	if err != nil {
		return "", nil, err
	}
	if len(changes) == 0 {
		header := FormatCommitHeader(commit)
		return header + "\nNo file level changes.", nil, nil
	}
	patch, err := changes.Patch()
	if err != nil {
		return "", nil, err
	}
	header := FormatCommitHeader(commit)
	return renderPatch(header, patch)
}
