package model

import (
	"strings"
	"sync"
	"sync/atomic"

	"github.com/thiagokokada/gitk-go/internal/git"
	"github.com/thiagokokada/gitk-go/internal/gui/selection"
)

const (
	LocalUnstagedRowID = "__local_unstaged__"
	LocalStagedRowID   = "__local_staged__"
	MoreIndicatorID    = "__more__"
	LoadingIndicatorID = "__loading__"

	DiffCommitSectionLabel = "Commit"
)

type App struct {
	Repo  Repository
	Data  Data
	State State
}

type Repository struct {
	Path    string
	HeadRef string
}

type Data struct {
	Commits []*git.Entry
	Visible []*git.Entry
}

type State struct {
	Tree      TreeState
	Diff      DiffState
	Filter    FilterState
	LocalDiff LocalDiffCache
	Scroll    ScrollState
	Selection selection.State
}

func NewApp(repoPath string) App {
	return App{
		Repo: Repository{Path: repoPath},
		State: State{
			Tree:      NewTreeState(),
			Diff:      NewDiffState(),
			LocalDiff: NewLocalDiffCache(),
		},
	}
}

func (m *App) ResetRepository(repoPath string) {
	*m = NewApp(repoPath)
}

func (m *App) ResetBranch() {
	filterValue := m.State.Filter.Value
	repoPath := m.Repo.Path
	*m = NewApp(repoPath)
	m.State.Filter.Value = filterValue
}

func (m *App) ApplyFilter(raw string) {
	m.State.Filter.Value = raw
	m.Data.Visible = FilterEntries(m.Data.Commits, raw)
	m.State.Tree.Rows.SetVisibleIndex(m.Data.Visible)
}

func (m *App) SetReloadedCommits(entries []*git.Entry, head string, hasMore bool) {
	m.Data.Commits = entries
	m.Data.Visible = entries
	m.Repo.HeadRef = head
	m.State.Tree.SetReloadedCommits(entries, hasMore)
}

func (m *App) AppendCommits(entries []*git.Entry, hasMore bool) {
	m.Data.Commits = append(m.Data.Commits, entries...)
	m.State.Tree.AppendCommits(entries, hasMore)
}

func (m *App) CommitEntryAt(idx int) (*git.Entry, bool) {
	if idx < 0 || idx >= len(m.Data.Visible) {
		return nil, false
	}
	entry := m.Data.Visible[idx]
	if entry == nil || entry.Commit == nil {
		return nil, false
	}
	return entry, true
}

func (m *App) CommitEntryForTreeID(id string) (*git.Entry, int, bool) {
	idx, ok := m.State.Tree.Rows.VisibleByID[id]
	if !ok {
		return nil, 0, false
	}
	entry, ok := m.CommitEntryAt(idx)
	if !ok || entry.Commit.Hash != id {
		return nil, 0, false
	}
	return entry, idx, true
}

type SelectionDisplayKind int

const (
	SelectionDisplayNone SelectionDisplayKind = iota
	SelectionDisplayMessage
	SelectionDisplayLocal
	SelectionDisplayCommit
)

type SelectionDisplayPlan struct {
	Kind       SelectionDisplayKind
	Staged     bool
	Entry      *git.Entry
	Index      int
	Message    string
	LoadDetail bool
}

func (m *App) EmptyCommitMessage() string {
	if len(m.Data.Commits) == 0 {
		return "Repository has no commits yet."
	}
	return "No commits match the current filter."
}

func (m *App) FallbackSelectionPlan() SelectionDisplayPlan {
	if len(m.Data.Visible) == 0 {
		m.State.Selection.Clear()
		return SelectionDisplayPlan{Kind: SelectionDisplayMessage, Message: m.EmptyCommitMessage()}
	}
	entry, ok := m.CommitEntryAt(0)
	if !ok {
		return SelectionDisplayPlan{}
	}
	return SelectionDisplayPlan{Kind: SelectionDisplayCommit, Entry: entry, Index: 0, LoadDetail: true}
}

func (m *App) FilterSelectionPlan() SelectionDisplayPlan {
	if staged, ok := m.State.Selection.LocalSelection(); ok {
		if m.State.Tree.Rows.HasSpecialItem(LocalRowID(staged)) {
			return SelectionDisplayPlan{Kind: SelectionDisplayLocal, Staged: staged}
		}
	}
	if len(m.Data.Visible) == 0 {
		return SelectionDisplayPlan{Kind: SelectionDisplayMessage, Message: m.EmptyCommitMessage()}
	}
	index := max(m.State.Selection.CommitIndex(m.Data.Visible), 0)
	entry, ok := m.CommitEntryAt(index)
	if !ok {
		return SelectionDisplayPlan{}
	}
	return SelectionDisplayPlan{
		Kind:       SelectionDisplayCommit,
		Entry:      entry,
		Index:      index,
		LoadDetail: entry.Commit.Hash != m.State.Selection.CommitHash(),
	}
}

type TreeSelectionKind int

const (
	TreeSelectionNone TreeSelectionKind = iota
	TreeSelectionClear
	TreeSelectionLocal
	TreeSelectionCommit
)

type TreeSelectionPlan struct {
	Kind   TreeSelectionKind
	Staged bool
	Entry  *git.Entry
	Index  int
}

func (m *App) TreeSelectionPlan(id string) TreeSelectionPlan {
	if id == "" {
		m.State.Selection.Clear()
		return TreeSelectionPlan{Kind: TreeSelectionClear}
	}
	if SelectionMatchesTreeID(&m.State.Selection, id) {
		return TreeSelectionPlan{}
	}
	switch id {
	case MoreIndicatorID, LoadingIndicatorID:
		m.State.Selection.Clear()
		return TreeSelectionPlan{Kind: TreeSelectionClear}
	case LocalUnstagedRowID:
		m.State.Selection.SetLocal(false)
		return TreeSelectionPlan{Kind: TreeSelectionLocal}
	case LocalStagedRowID:
		m.State.Selection.SetLocal(true)
		return TreeSelectionPlan{Kind: TreeSelectionLocal, Staged: true}
	}
	entry, idx, ok := m.CommitEntryForTreeID(id)
	if !ok {
		m.State.Selection.Clear()
		return TreeSelectionPlan{Kind: TreeSelectionClear}
	}
	return TreeSelectionPlan{Kind: TreeSelectionCommit, Entry: entry, Index: idx}
}

func SelectionMatchesTreeID(s *selection.State, id string) bool {
	if staged, ok := s.LocalSelection(); ok {
		return id == LocalRowID(staged)
	}
	return s.CommitHash() == id && id != ""
}

type FilterState struct {
	Value string
}

type ScrollState struct {
	Start float64
	Total int
}

func (s ScrollState) RestoreTarget(newTotal int) (float64, bool) {
	if s.Start < 0 || s.Total <= 0 || newTotal <= 0 {
		return 0, false
	}
	target := s.Start * float64(s.Total) / float64(newTotal)
	target = max(0.0, min(target, 1.0))
	return target, true
}

func FilterEntries(entries []*git.Entry, query string) []*git.Entry {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return entries
	}
	var filtered []*git.Entry
	for _, entry := range entries {
		if strings.Contains(entry.SearchText, q) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

type DiffState struct {
	SyntaxGeneration      atomic.Uint64
	FileSections          []git.FileSection
	SyntaxTags            map[string]string
	SelectedFileIndex     int
	SuppressFileSelection bool
	SkipNextSync          bool
}

func NewDiffState() DiffState {
	return DiffState{SyntaxTags: make(map[string]string), SelectedFileIndex: -1}
}

func (d *DiffState) SetFileSections(sections []git.FileSection) {
	d.FileSections = sections
	d.SelectedFileIndex = -1
}

func (d *DiffState) BeginUserFileSelection(idx int) (line int, ok bool) {
	if d.SuppressFileSelection {
		return 0, false
	}
	line, ok = DiffSectionLine(d.FileSections, idx)
	if !ok {
		return 0, false
	}
	d.SkipNextSync = true
	return line, true
}

func (d *DiffState) SyncSelectionIndexForLine(line int) (idx int, ok bool) {
	if line <= 0 {
		return 0, false
	}
	return DiffSectionIndexForLine(d.FileSections, line)
}

func (d *DiffState) ConsumeSkipNextSync() bool {
	if !d.SkipNextSync {
		return false
	}
	d.SkipNextSync = false
	return true
}

func (d *DiffState) SelectFileIndex(idx int) bool {
	if idx < 0 || idx >= len(d.FileSections) {
		return false
	}
	if d.SelectedFileIndex == idx {
		return false
	}
	d.SuppressFileSelection = true
	d.SelectedFileIndex = idx
	return true
}

func (d *DiffState) FinishProgrammaticFileSelection() {
	d.SuppressFileSelection = false
}

func (d *DiffState) SelectedFilePath() (string, bool) {
	if d.SelectedFileIndex < 0 {
		return "", false
	}
	return DiffFilePathForIndex(d.FileSections, d.SelectedFileIndex)
}

type DiffRequest struct {
	Entry *git.Entry
	Hash  string
}

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
