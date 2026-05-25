package gui

import (
	"sync"

	"github.com/thiagokokada/gitk-go/internal/git"
	"github.com/thiagokokada/gitk-go/internal/gui/widgets"
)

func newAppModel(repoPath string) appModel {
	return appModel{
		repo: controllerRepo{
			path: repoPath,
		},
		state: controllerState{
			tree:      newTreeState(),
			diff:      newDiffState(),
			localDiff: newLocalDiffCache(),
		},
	}
}

func (m *appModel) resetRepository(repoPath string) {
	*m = newAppModel(repoPath)
}

func (m *appModel) resetBranch() {
	filterValue := m.state.filter.value
	repoPath := m.repo.path
	*m = newAppModel(repoPath)
	m.state.filter.value = filterValue
}

func (m *appModel) applyFilter(raw string) {
	m.state.filter.value = raw
	m.data.visible = filterEntries(m.data.commits, raw)
	m.state.tree.rows.setVisibleIndex(m.data.visible)
}

func (m *appModel) setReloadedCommits(entries []*git.Entry, head string, hasMore bool) {
	m.data.commits = entries
	m.data.visible = entries
	m.repo.headRef = head
	m.state.tree.setReloadedCommits(entries, hasMore)
}

func (m *appModel) appendCommits(entries []*git.Entry, hasMore bool) {
	m.data.commits = append(m.data.commits, entries...)
	m.state.tree.appendCommits(entries, hasMore)
}

func newDiffState() diffState {
	return diffState{
		syntaxTags: make(map[string]string),
	}
}

type diffState struct {
	syntaxGeneration      uint64
	fileSections          []git.FileSection
	syntaxTags            map[string]string
	selectedFileIndex     int
	suppressFileSelection bool
	skipNextSync          bool
}

type diffRequest struct {
	entry *git.Entry
	hash  string
}

type treeState struct {
	branchLabels      map[string][]string
	contextTargetID   string
	hasMore           bool
	loadingBatch      bool
	showLocalUnstaged bool
	showLocalStaged   bool

	rows treeRowState

	graphCanvas *widgets.GraphCanvas
}

func newTreeState() treeState {
	return treeState{
		branchLabels: make(map[string][]string),
	}
}

func (t *treeState) setReloadedCommits(entries []*git.Entry, hasMore bool) {
	t.hasMore = hasMore
	t.rows.setCommitIDs(entries)
	t.rows.refreshValues = true
}

func (t *treeState) appendCommits(entries []*git.Entry, hasMore bool) {
	t.hasMore = hasMore
	t.rows.addCommitIDs(entries)
	t.rows.refreshValues = true
}

func (t *treeState) markNoMoreCommits() {
	t.hasMore = false
}

func (t *treeState) beginCommitBatchLoad(prefetch bool) bool {
	if t.loadingBatch || (!prefetch && !t.hasMore) {
		return false
	}
	t.loadingBatch = true
	return true
}

func (t *treeState) finishCommitBatchLoad() {
	t.loadingBatch = false
}

func (t *treeState) setLocalRowVisible(staged bool, show bool) bool {
	if staged {
		if t.showLocalStaged == show {
			return false
		}
		t.showLocalStaged = show
		return true
	}
	if t.showLocalUnstaged == show {
		return false
	}
	t.showLocalUnstaged = show
	return true
}

func (t treeState) localRowVisible(staged bool) bool {
	if staged {
		return t.showLocalStaged
	}
	return t.showLocalUnstaged
}

func (t treeState) localRowInsertIndex(staged bool) int {
	if staged && t.showLocalUnstaged {
		return 1
	}
	return 0
}

func (t treeState) localRowIDs() []string {
	ids := make([]string, 0, 2)
	if t.showLocalUnstaged {
		ids = append(ids, localUnstagedRowID)
	}
	if t.showLocalStaged {
		ids = append(ids, localStagedRowID)
	}
	return ids
}

type treeRowState struct {
	commitIDs     map[string]struct{}
	visibleByID   map[string]int
	items         map[string]struct{}
	values        map[string]treeRow
	specialItems  map[string]struct{}
	refreshValues bool
}

func (s *treeRowState) resetTracking() {
	s.items = nil
	s.values = nil
	s.specialItems = nil
}

func (s treeRowState) trackedItemIDs() []string {
	if len(s.items) == 0 {
		return nil
	}
	ids := make([]string, 0, len(s.items))
	for id := range s.items {
		ids = append(ids, id)
	}
	return ids
}

func (s *treeRowState) pruneStaleCommitRows() []string {
	if len(s.items) == 0 || s.commitIDs == nil {
		return nil
	}
	ids := make([]string, 0)
	for id := range s.items {
		if _, ok := s.commitIDs[id]; ok {
			continue
		}
		ids = append(ids, id)
		delete(s.items, id)
		delete(s.values, id)
	}
	return ids
}

type filterState struct {
	value string
}

type scrollState struct {
	start float64
	total int
}

type localDiffCache struct {
	mu    sync.Mutex
	items map[bool]*localDiffState
}

func newLocalDiffCache() localDiffCache {
	return localDiffCache{items: make(map[bool]*localDiffState)}
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
