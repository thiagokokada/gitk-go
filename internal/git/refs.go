package git

import (
	"fmt"
	"strings"
)

func (s *Service) BranchLabels() (map[string][]string, error) {
	labels := map[string][]string{}
	if s.repo.path == "" {
		return labels, nil
	}

	out, err := s.runGitCommand(
		[]string{
			"--no-pager",
			"show-ref",
			"--dereference",
		},
		true,
		"git show-ref",
	)
	if err != nil {
		return nil, err
	}

	parsed, err := parseRefLabelsFromShowRef(out)
	if err != nil {
		return nil, err
	}
	for hash, values := range parsed {
		labels[hash] = append(labels[hash], values...)
	}

	headHashOut, err := s.runGitCommand([]string{"rev-parse", "-q", "--verify", "HEAD"}, true, "git rev-parse")
	if err != nil {
		return nil, err
	}
	headHash := strings.TrimSpace(headHashOut)
	if headHash != "" {
		refOut, err := s.runGitCommand([]string{"symbolic-ref", "-q", "--short", "HEAD"}, true, "git symbolic-ref")
		if err != nil {
			return nil, err
		}
		headBranch := strings.TrimSpace(refOut)
		label := "HEAD"
		if headBranch != "" {
			label = fmt.Sprintf("HEAD -> %s", headBranch)
		}
		labels[headHash] = append([]string{label}, labels[headHash]...)
	}
	return labels, nil
}

func parseRefLabelsFromShowRef(out string) (map[string][]string, error) {
	type refEntry struct {
		hash string
		ref  string
	}

	peeledByTagRef := map[string]string{}
	var entries []refEntry

	for _, rawLine := range strings.Split(out, "\n") {
		line := strings.TrimRight(rawLine, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			return nil, fmt.Errorf("unexpected show-ref output line: %q", rawLine)
		}
		hash := strings.TrimSpace(parts[0])
		refName := strings.TrimSpace(parts[1])
		if hash == "" || refName == "" {
			return nil, fmt.Errorf("unexpected show-ref output line: %q", rawLine)
		}
		if strings.HasSuffix(refName, "^{}") {
			base := strings.TrimSuffix(refName, "^{}")
			if base != "" {
				peeledByTagRef[base] = hash
			}
			continue
		}
		entries = append(entries, refEntry{hash: hash, ref: refName})
	}

	labels := map[string][]string{}
	for _, entry := range entries {
		refName := entry.ref
		isTag := strings.HasPrefix(refName, "refs/tags/")
		isBranch := strings.HasPrefix(refName, "refs/heads/")
		isRemote := strings.HasPrefix(refName, "refs/remotes/")
		if !isTag && !isBranch && !isRemote {
			continue
		}

		short := refName
		switch {
		case isTag:
			short = strings.TrimPrefix(refName, "refs/tags/")
		case isBranch:
			short = strings.TrimPrefix(refName, "refs/heads/")
		case isRemote:
			short = strings.TrimPrefix(refName, "refs/remotes/")
		}
		if short == "" {
			continue
		}
		if isRemote && strings.HasSuffix(short, "/HEAD") {
			continue
		}

		hash := entry.hash
		if isTag {
			if peeled, ok := peeledByTagRef[refName]; ok && peeled != "" {
				hash = peeled
			}
		}

		label := short
		if isTag {
			label = fmt.Sprintf("tag: %s", short)
		}
		labels[hash] = append(labels[hash], label)
	}
	return labels, nil
}
