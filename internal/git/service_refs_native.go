//go:build !gitcli

package git

import (
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
)

func (s *Service) BranchLabels() (map[string][]string, error) {
	labels := map[string][]string{}
	if s.repo.Repository == nil {
		return labels, nil
	}
	refs, err := s.repo.References()
	if err != nil {
		return nil, err
	}
	defer refs.Close()
	headRef, err := s.repo.Head()
	var headHash plumbing.Hash
	var headBranch string
	if err == nil && headRef != nil {
		headHash = headRef.Hash()
		if headRef.Name().IsBranch() {
			headBranch = headRef.Name().Short()
		}
	}
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Type() != plumbing.HashReference {
			return nil
		}
		name := ref.Name()
		isBranch := name.IsBranch()
		isRemote := name.IsRemote()
		isTag := name.IsTag()
		if !isBranch && !isRemote && !isTag {
			return nil
		}
		short := name.Short()
		if isRemote && strings.HasSuffix(short, "/HEAD") {
			return nil
		}
		hash := ref.Hash()
		label := short
		if isTag {
			label = fmt.Sprintf("tag: %s", short)
			if peeled, ok := s.peelTagCommitHash(hash); ok {
				hash = peeled
			}
		}
		labels[hash.String()] = append(labels[hash.String()], label)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if headHash != plumbing.ZeroHash {
		key := headHash.String()
		label := "HEAD"
		if headBranch != "" {
			label = fmt.Sprintf("HEAD -> %s", headBranch)
		}
		labels[key] = append([]string{label}, labels[key]...)
	}
	return labels, nil
}

func (s *Service) peelTagCommitHash(hash plumbing.Hash) (plumbing.Hash, bool) {
	if s == nil || s.repo.Repository == nil || hash == plumbing.ZeroHash {
		return plumbing.ZeroHash, false
	}
	// Lightweight tags point directly at a commit; annotated tags point at a tag object.
	if _, err := s.repo.CommitObject(hash); err == nil {
		return hash, true
	}
	cur := hash
	for range 8 {
		tag, err := s.repo.TagObject(cur)
		if err != nil {
			return plumbing.ZeroHash, false
		}
		switch tag.TargetType {
		case plumbing.CommitObject:
			return tag.Target, true
		case plumbing.TagObject:
			cur = tag.Target
		default:
			return plumbing.ZeroHash, false
		}
	}
	return plumbing.ZeroHash, false
}
