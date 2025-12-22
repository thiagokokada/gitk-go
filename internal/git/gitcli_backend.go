package git

import (
	"fmt"
	"strings"
)

func (r repo) HeadState() (hash string, headName string, ok bool, err error) {
	if r.path == "" {
		return "", "", false, fmt.Errorf("repository root not set")
	}
	out, err := r.runGitCommand([]string{"rev-parse", "-q", "--verify", "HEAD"}, true, "git rev-parse")
	if err != nil {
		return "", "", false, err
	}
	hash = strings.TrimSpace(out)
	if hash == "" {
		return "", "", false, nil
	}
	ref, err := r.runGitCommand([]string{"symbolic-ref", "-q", "--short", "HEAD"}, true, "git symbolic-ref")
	if err != nil {
		return "", "", false, err
	}
	headName = strings.TrimSpace(ref)
	if headName == "" {
		headName = "HEAD"
	}
	return hash, headName, true, nil
}

func (r repo) CommitDiffText(commitHash string, parentHash string) (string, error) {
	commitHash = strings.TrimSpace(commitHash)
	parentHash = strings.TrimSpace(parentHash)
	if commitHash == "" {
		return "", fmt.Errorf("commit not specified")
	}
	if parentHash != "" {
		return r.runGitCommand(
			[]string{"diff", "--no-color", parentHash, commitHash},
			true,
			"git diff",
		)
	}
	return r.runGitCommand(
		[]string{"show", "--no-color", "--pretty=format:", commitHash},
		false,
		"git show",
	)
}

func (r repo) WorktreeDiffText(staged bool) (string, error) {
	if r.path == "" {
		return "", fmt.Errorf("repository root not set")
	}
	args := []string{"diff", "--no-color"}
	if staged {
		args = append(args, "--cached")
	}
	return r.runGitCommand(args, true, "git diff")
}

func (r repo) LocalChangesStatus() (LocalChanges, error) {
	var res LocalChanges
	if r.path == "" {
		return res, fmt.Errorf("repository root not set")
	}
	out, err := r.runGitCommand([]string{"status", "--porcelain=v2"}, false, "git status")
	if err != nil {
		return res, err
	}
	res, err = parseStatusPorcelainV2(strings.NewReader(out))
	if err != nil {
		return res, fmt.Errorf("parse git status: %w", err)
	}
	return res, nil
}

func (r repo) ListRefs() ([]Ref, error) {
	if r.path == "" {
		return nil, nil
	}
	out, err := r.runGitCommand(
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
	return parseRefsFromShowRef(out)
}

func parseRefsFromShowRef(out string) ([]Ref, error) {
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

	var refs []Ref
	for _, entry := range entries {
		refName := entry.ref
		switch {
		case strings.HasPrefix(refName, "refs/tags/"):
			short := strings.TrimPrefix(refName, "refs/tags/")
			if short == "" {
				continue
			}
			hash := entry.hash
			if peeled, ok := peeledByTagRef[refName]; ok && peeled != "" {
				hash = peeled
			}
			refs = append(refs, Ref{Hash: hash, Kind: RefKindTag, Name: short})
		case strings.HasPrefix(refName, "refs/heads/"):
			short := strings.TrimPrefix(refName, "refs/heads/")
			if short == "" {
				continue
			}
			refs = append(refs, Ref{Hash: entry.hash, Kind: RefKindBranch, Name: short})
		case strings.HasPrefix(refName, "refs/remotes/"):
			short := strings.TrimPrefix(refName, "refs/remotes/")
			if short == "" {
				continue
			}
			refs = append(refs, Ref{Hash: entry.hash, Kind: RefKindRemoteBranch, Name: short})
		default:
			continue
		}
	}
	return refs, nil
}
