package model

import (
	"strings"
	"sync/atomic"

	"github.com/thiagokokada/gitk-go/internal/git"
)

type selectionStateKind int

const (
	selectionStateNone selectionStateKind = iota
	selectionStateCommit
	selectionStateLocalUnstaged
	selectionStateLocalStaged
)

type selectionSnapshot struct {
	kind selectionStateKind
	hash string
	idx  int
}

type SelectionState struct {
	snapshot atomic.Pointer[selectionSnapshot]
}

func (s *SelectionState) snapshotValue() selectionSnapshot {
	if snap := s.snapshot.Load(); snap != nil {
		return *snap
	}
	return selectionSnapshot{}
}

func (s *SelectionState) storeSnapshot(snapshot selectionSnapshot) {
	s.snapshot.Store(&snapshot)
}

func (s *SelectionState) Clear() {
	s.snapshot.Store(nil)
}

func (s *SelectionState) SetCommit(entry *git.Entry, idx int) bool {
	if entry == nil || entry.Commit == nil || idx < 0 {
		s.Clear()
		return false
	}
	s.storeSnapshot(selectionSnapshot{
		kind: selectionStateCommit,
		hash: entry.Commit.Hash,
		idx:  idx,
	})
	return true
}

func (s *SelectionState) SetLocal(staged bool) {
	kind := selectionStateLocalUnstaged
	if staged {
		kind = selectionStateLocalStaged
	}
	s.storeSnapshot(selectionSnapshot{kind: kind})
}

func (s *SelectionState) CommitHash() string {
	snap := s.snapshotValue()
	if snap.kind != selectionStateCommit {
		return ""
	}
	return snap.hash
}

func (s *SelectionState) LocalSelection() (staged bool, ok bool) {
	snap := s.snapshotValue()
	switch snap.kind {
	case selectionStateLocalUnstaged:
		return false, true
	case selectionStateLocalStaged:
		return true, true
	default:
		return false, false
	}
}

func (s *SelectionState) CommitIndex(visible []*git.Entry) int {
	snap := s.snapshotValue()
	if snap.kind != selectionStateCommit {
		return -1
	}
	if snap.idx >= 0 && snap.idx < len(visible) {
		entry := visible[snap.idx]
		if entry != nil && entry.Commit != nil && entry.Commit.Hash == snap.hash {
			return snap.idx
		}
	}
	if snap.hash == "" {
		return -1
	}
	for idx, entry := range visible {
		if entry == nil || entry.Commit == nil {
			continue
		}
		if entry.Commit.Hash == snap.hash {
			return idx
		}
	}
	return -1
}

func (s *SelectionState) MatchesTreeID(id string) bool {
	id = strings.TrimSpace(id)
	if staged, ok := s.LocalSelection(); ok {
		return id == LocalRowID(staged)
	}
	hash := s.CommitHash()
	return hash != "" && hash == id
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
	if m.State.Selection.MatchesTreeID(id) {
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
