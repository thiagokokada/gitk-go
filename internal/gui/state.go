package gui

import (
	"sync"
	"sync/atomic"

	"github.com/thiagokokada/gitk-go/internal/debounce"
	"github.com/thiagokokada/gitk-go/internal/git"
	"github.com/thiagokokada/gitk-go/internal/gui/widgets"
)

type diffState struct {
	fileSections          []git.FileSection
	syntaxTags            map[string]string
	suppressFileSelection bool
	skipNextSync          bool

	mu          sync.Mutex
	debouncer   *debounce.Debouncer
	pendingDiff *git.Entry
	pendingHash string
}

type treeState struct {
	branchLabels      map[string][]string
	contextTargetID   string
	hasMore           bool
	loadingBatch      bool
	showLocalUnstaged bool
	showLocalStaged   bool

	graphCanvas *widgets.GraphCanvas
}

type filterState struct {
	value string

	mu        sync.Mutex
	debouncer *debounce.Debouncer
	pending   string
}

type selectionState struct {
	hash atomic.Pointer[string]
}

type scrollState struct {
	start float64
	total int
}

type localDiffCache struct {
	mu    sync.Mutex
	items map[bool]*localDiffState
}

func (c *localDiffCache) state(staged bool, create bool) *localDiffState {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.items == nil {
		if !create {
			return nil
		}
		c.items = make(map[bool]*localDiffState)
	}
	if st, ok := c.items[staged]; ok {
		return st
	}
	if !create {
		return nil
	}
	st := &localDiffState{}
	c.items[staged] = st
	return st
}

type localDiffState struct {
	sync.Mutex
	ready      bool
	loading    bool
	diff       string
	sections   []git.FileSection
	err        error
	generation int
}

func (s *localDiffState) snapshotLocked() localDiffSnapshot {
	snap := localDiffSnapshot{
		ready:   s.ready,
		loading: s.loading,
		diff:    s.diff,
		err:     s.err,
	}
	if len(s.sections) > 0 {
		snap.sections = append([]git.FileSection(nil), s.sections...)
	}
	return snap
}

func (s *localDiffState) startLoadingLocked(force bool) (int, bool) {
	if s.loading {
		return 0, false
	}
	if s.ready && !force {
		return 0, false
	}
	s.loading = true
	s.ready = false
	s.diff = ""
	s.sections = nil
	s.err = nil
	s.generation++
	return s.generation, true
}

func (s *localDiffState) resetLocked() {
	s.loading = false
	s.ready = false
	s.diff = ""
	s.sections = nil
	s.err = nil
	s.generation++
}

type localDiffSnapshot struct {
	ready    bool
	loading  bool
	diff     string
	sections []git.FileSection
	err      error
}
