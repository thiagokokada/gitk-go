package model

import (
	"github.com/thiagokokada/gitk-go/internal/git"
	"github.com/thiagokokada/gitk-go/internal/gui/selection"
)

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
