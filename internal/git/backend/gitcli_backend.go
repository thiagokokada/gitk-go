package backend

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

func (g *gitCLI) HeadState() (hash string, headName string, ok bool, err error) {
	if g == nil || g.path == "" {
		return "", "", false, fmt.Errorf("repository root not set")
	}
	out, err := g.runGitCommand([]string{"rev-parse", "-q", "--verify", "HEAD"}, true, "git rev-parse")
	if err != nil {
		return "", "", false, err
	}
	hash = strings.TrimSpace(out)
	if hash == "" {
		return "", "", false, nil
	}
	ref, err := g.runGitCommand([]string{"symbolic-ref", "-q", "--short", "HEAD"}, true, "git symbolic-ref")
	if err != nil {
		return "", "", false, err
	}
	headName = strings.TrimSpace(ref)
	if headName == "" {
		headName = "HEAD"
	}
	return hash, headName, true, nil
}

func (g *gitCLI) CommitDiffText(commitHash string, parentHash string) (string, error) {
	commitHash = strings.TrimSpace(commitHash)
	parentHash = strings.TrimSpace(parentHash)
	if commitHash == "" {
		return "", fmt.Errorf("commit not specified")
	}
	if parentHash != "" {
		return g.runGitCommand(
			[]string{"diff", "--no-color", parentHash, commitHash},
			true,
			"git diff",
		)
	}
	return g.runGitCommand(
		[]string{"show", "--no-color", "--pretty=format:", commitHash},
		false,
		"git show",
	)
}

func (g *gitCLI) WorktreeDiffText(staged bool) (string, error) {
	if g == nil || g.path == "" {
		return "", fmt.Errorf("repository root not set")
	}
	args := []string{"diff", "--no-color"}
	if staged {
		args = append(args, "--cached")
		return g.runGitCommand(args, true, "git diff")
	}
	trackedDiff, err := g.runGitCommand(args, true, "git diff")
	if err != nil {
		return "", err
	}
	untrackedDiff, err := g.untrackedWorktreeDiffText()
	if err != nil {
		return "", err
	}
	return concatDiffText(trackedDiff, untrackedDiff), nil
}

func (g *gitCLI) LocalChangesStatus() (LocalChanges, error) {
	var res LocalChanges
	if g == nil || g.path == "" {
		return res, fmt.Errorf("repository root not set")
	}
	out, err := g.runGitCommand([]string{"status", "--porcelain=v2"}, false, "git status")
	if err != nil {
		return res, err
	}
	res, err = parseStatusPorcelainV2(strings.NewReader(out))
	if err != nil {
		return res, fmt.Errorf("parse git status: %w", err)
	}
	return res, nil
}

func parseStatusPorcelainV2(r io.Reader) (LocalChanges, error) {
	var res LocalChanges
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 2 {
			continue
		}
		switch line[0] {
		case '1', '2', 'u':
			if len(line) < 4 {
				continue
			}
			stagedState := line[2]
			worktreeState := line[3]
			if stagedState != '.' {
				res.HasStaged = true
			}
			if worktreeState != '.' && worktreeState != '?' {
				res.HasWorktree = true
			}
		case '?':
			res.HasWorktree = true
		default:
			// '!' ignored, etc.
		}
		if res.HasWorktree && res.HasStaged {
			break
		}
	}
	return res, scanner.Err()
}

func (g *gitCLI) ApplyPatchToIndex(patch string, reverse bool) error {
	if g == nil || g.path == "" {
		return fmt.Errorf("repository root not set")
	}
	if strings.TrimSpace(patch) == "" {
		return fmt.Errorf("patch not specified")
	}
	args := []string{"apply", "--cached", "--unidiff-zero", "--recount"}
	if reverse {
		args = append(args, "--reverse")
	}
	_, err := g.runGitCommandWithInput(args, patch, "git apply")
	return err
}

func (g *gitCLI) untrackedWorktreeDiffText() (string, error) {
	paths, err := g.untrackedPaths()
	if err != nil {
		return "", err
	}
	var diffs []string
	for _, path := range paths {
		diffText, err := g.runGitCommand(
			[]string{
				"diff",
				"--no-color",
				"--no-index",
				"--src-prefix=a/",
				"--dst-prefix=b/",
				"--",
				"/dev/null",
				path,
			},
			true,
			"git diff --no-index",
		)
		if err != nil {
			return "", err
		}
		diffs = append(diffs, diffText)
	}
	return concatDiffText(diffs...), nil
}

func (g *gitCLI) untrackedPaths() ([]string, error) {
	out, err := g.runGitCommand(
		[]string{"ls-files", "--others", "--exclude-standard", "-z"},
		false,
		"git ls-files",
	)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	var paths []string
	for rawPath := range strings.SplitSeq(out, "\x00") {
		if rawPath == "" {
			continue
		}
		paths = append(paths, rawPath)
	}
	return paths, nil
}

func concatDiffText(parts ...string) string {
	var b strings.Builder
	needsTrailingNewline := false
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		if needsTrailingNewline {
			b.WriteByte('\n')
		}
		b.WriteString(part)
		needsTrailingNewline = !strings.HasSuffix(part, "\n")
	}
	if b.Len() == 0 {
		return ""
	}
	if needsTrailingNewline {
		b.WriteByte('\n')
	}
	return b.String()
}

func (g *gitCLI) ListRefs() ([]Ref, error) {
	if g == nil || g.path == "" {
		return nil, nil
	}
	out, err := g.runGitCommand(
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

func (g *gitCLI) SwitchBranch(branch string) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return fmt.Errorf("branch not specified")
	}
	_, err := g.runGitCommand([]string{"switch", "--", branch}, false, "git switch")
	return err
}

func parseRefsFromShowRef(out string) ([]Ref, error) {
	type refEntry struct {
		hash string
		ref  string
	}

	peeledByTagRef := map[string]string{}
	var entries []refEntry

	for rawLine := range strings.SplitSeq(out, "\n") {
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
		if base, ok := strings.CutSuffix(refName, "^{}"); ok {
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
