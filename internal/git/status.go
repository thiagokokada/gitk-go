package git

import (
	"bufio"
	"fmt"
	"io"
)

func (s *Service) LocalChanges() (LocalChanges, error) {
	if s.backend == nil {
		return LocalChanges{}, fmt.Errorf("repository root not set")
	}
	return s.backend.LocalChangesStatus()
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
		default:
			// '?' untracked, '!' ignored, etc.
		}
		if res.HasWorktree && res.HasStaged {
			break
		}
	}
	return res, scanner.Err()
}
