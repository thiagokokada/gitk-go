package model

import (
	"sync"

	"github.com/thiagokokada/gitk-go/internal/git"
)

type LocalDiffCache struct {
	mu    sync.Mutex
	Items map[bool]*LocalDiffState
}

func NewLocalDiffCache() LocalDiffCache {
	return LocalDiffCache{Items: make(map[bool]*LocalDiffState)}
}

func (c *LocalDiffCache) State(staged bool, create bool) *LocalDiffState {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Items == nil {
		if !create {
			return nil
		}
		c.Items = make(map[bool]*LocalDiffState)
	}
	if st, ok := c.Items[staged]; ok {
		return st
	}
	if !create {
		return nil
	}
	st := &LocalDiffState{}
	c.Items[staged] = st
	return st
}

func (c *LocalDiffCache) Snapshot(staged bool) LocalDiffSnapshot {
	state := c.State(staged, false)
	if state == nil {
		return LocalDiffSnapshot{}
	}
	state.Lock()
	defer state.Unlock()
	return state.SnapshotLocked()
}

func (c *LocalDiffCache) BeginLoading(staged bool, force bool) (generation int, started bool) {
	state := c.State(staged, true)
	state.Lock()
	defer state.Unlock()
	return state.StartLoadingLocked(force)
}

func (c *LocalDiffCache) CompleteLoading(
	staged bool,
	generation int,
	diff string,
	sections []git.FileSection,
	err error,
) bool {
	state := c.State(staged, true)
	state.Lock()
	defer state.Unlock()
	if generation != state.Generation {
		return false
	}
	state.Loading = false
	state.Ready = true
	state.Diff = diff
	if len(sections) > 0 {
		state.Sections = append([]git.FileSection(nil), sections...)
	} else {
		state.Sections = nil
	}
	state.Err = err
	return true
}

func (c *LocalDiffCache) Reset(staged bool) {
	state := c.State(staged, false)
	if state == nil {
		return
	}
	state.Lock()
	defer state.Unlock()
	state.ResetLocked()
}

type LocalDiffState struct {
	sync.Mutex
	Ready      bool
	Loading    bool
	Diff       string
	Sections   []git.FileSection
	Err        error
	Generation int
}

func (s *LocalDiffState) SnapshotLocked() LocalDiffSnapshot {
	snap := LocalDiffSnapshot{Ready: s.Ready, Loading: s.Loading, Diff: s.Diff, Err: s.Err}
	if len(s.Sections) > 0 {
		snap.Sections = append([]git.FileSection(nil), s.Sections...)
	}
	return snap
}

func (s *LocalDiffState) StartLoadingLocked(force bool) (int, bool) {
	if s.Loading {
		return 0, false
	}
	if s.Ready && !force {
		return 0, false
	}
	s.Loading = true
	s.Ready = false
	s.Diff = ""
	s.Sections = nil
	s.Err = nil
	s.Generation++
	return s.Generation, true
}

func (s *LocalDiffState) ResetLocked() {
	s.Loading = false
	s.Ready = false
	s.Diff = ""
	s.Sections = nil
	s.Err = nil
	s.Generation++
}

type LocalDiffSnapshot struct {
	Ready    bool
	Loading  bool
	Diff     string
	Sections []git.FileSection
	Err      error
}
