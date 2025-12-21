//go:build !gitcli

package git

import (
	"fmt"
	"io"
	"log/slog"

	gitlib "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type scanSession struct {
	head     plumbing.Hash
	headName string

	displayIter object.CommitIter

	// buffered holds the next commit returned by hasMore() so ScanCommits can keep consuming in-order.
	buffered  *object.Commit
	exhausted bool
	returned  int

	graphIter      object.CommitIter
	graphBuilder   *graphBuilder
	graphCache     map[plumbing.Hash]string
	graphProcessed int
	graphColsMax   int
	graphEOF       bool
}

func (s *Service) ensureScanSessionLocked(ref *plumbing.Reference) error {
	if s.scan != nil && s.scan.head == ref.Hash() {
		return nil
	}
	return s.resetScanLocked(ref)
}

func (s *Service) resetScanLocked(ref *plumbing.Reference) error {
	if s.scan != nil {
		s.scan.close()
		s.scan = nil
	}
	displayIter, err := s.repo.Log(&gitlib.LogOptions{From: ref.Hash(), Order: gitlib.LogOrderCommitterTime})
	if err != nil {
		return fmt.Errorf("read commits: %w", err)
	}
	// Graph generation must follow the same order as the commit list so we can
	// attach graph strings to the corresponding visible rows by position.
	graphIter, err := s.repo.Log(&gitlib.LogOptions{From: ref.Hash(), Order: gitlib.LogOrderCommitterTime})
	if err != nil {
		displayIter.Close()
		return fmt.Errorf("read commits for graph: %w", err)
	}
	s.scan = &scanSession{
		head:        ref.Hash(),
		headName:    refName(ref),
		displayIter: displayIter,
		graphIter:   graphIter,

		graphBuilder: newGraphBuilder(s.graphMaxColumns),
		graphCache:   make(map[plumbing.Hash]string, DefaultBatch),
	}
	slog.Debug("ScanCommits session initialized", slog.String("head", s.scan.headName))
	return nil
}

func (s *scanSession) close() {
	if s == nil {
		return
	}
	if s.displayIter != nil {
		s.displayIter.Close()
	}
	if s.graphIter != nil {
		s.graphIter.Close()
	}
	s.displayIter = nil
	s.graphIter = nil
	s.buffered = nil
	s.exhausted = true
	s.graphEOF = true
}

func (s *scanSession) hasMore() (bool, error) {
	if s.exhausted {
		return false, nil
	}
	if s.buffered != nil {
		return true, nil
	}
	// Read-ahead a single commit into buffered so hasMore doesn't consume an extra commit.
	commit, err := s.displayIter.Next()
	if err != nil {
		if err == io.EOF {
			s.exhausted = true
			return false, nil
		}
		return false, fmt.Errorf("iterate commits: %w", err)
	}
	s.buffered = commit
	return true, nil
}

func (s *scanSession) next() (*object.Commit, error) {
	if s.exhausted {
		return nil, io.EOF
	}
	if s.buffered != nil {
		commit := s.buffered
		s.buffered = nil
		s.returned++
		return commit, nil
	}
	commit, err := s.displayIter.Next()
	if err != nil {
		if err == io.EOF {
			s.exhausted = true
		}
		return nil, err
	}
	s.returned++
	return commit, nil
}

func (s *scanSession) discard(count int) error {
	// Consume and drop commits to align the session position with the requested skip.
	for range count {
		if _, err := s.next(); err != nil {
			return err
		}
	}
	return nil
}

func (s *scanSession) ensureGraphProcessed(target int) error {
	if target <= 0 || s.graphEOF || s.graphIter == nil || s.graphBuilder == nil {
		return nil
	}
	for s.graphProcessed < target && !s.graphEOF {
		commit, err := s.graphIter.Next()
		if err != nil {
			if err == io.EOF {
				s.graphEOF = true
				break
			}
			return fmt.Errorf("iterate commits for graph: %w", err)
		}
		line := s.graphBuilder.Line(commit)
		s.graphProcessed++
		s.graphCache[commit.Hash] = line
		if cols := len(s.graphBuilder.columns); cols > s.graphColsMax {
			s.graphColsMax = cols
		}
	}
	return nil
}

func (s *scanSession) assignGraphStrings(entries []*Entry) {
	if len(entries) == 0 || len(s.graphCache) == 0 {
		return
	}
	for _, entry := range entries {
		if entry == nil || entry.Commit == nil {
			continue
		}
		entry.Graph = s.graphCache[entry.Commit.Hash]
	}
}
