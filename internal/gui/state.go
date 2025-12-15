package gui

import (
	"sync"
	"sync/atomic"

	"github.com/thiagokokada/gitk-go/internal/debounce"
	"github.com/thiagokokada/gitk-go/internal/git"
)

type treeState struct {
	branchLabels      map[string][]string
	contextTargetID   string
	hasMore           bool
	loadingBatch      bool
	showLocalUnstaged bool
	showLocalStaged   bool
}

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

type localDiffSnapshot struct {
	ready    bool
	loading  bool
	diff     string
	sections []git.FileSection
	err      error
}
