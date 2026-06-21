package model

import (
	"github.com/thiagokokada/gitk-go/internal/git"
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
	Commits []git.Entry
	Visible []git.Entry
}

type State struct {
	Tree      TreeState
	Diff      DiffState
	Filter    FilterState
	LocalDiff LocalDiffCache
	Scroll    ScrollState
	Selection SelectionState
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

func (m *App) SetReloadedCommits(entries []git.Entry, head string, hasMore bool) {
	m.Data.Commits = entries
	m.Data.Visible = entries
	m.Repo.HeadRef = head
	m.State.Tree.SetReloadedCommits(entries, hasMore)
}

func (m *App) AppendCommits(entries []git.Entry, hasMore bool) {
	m.Data.Commits = append(m.Data.Commits, entries...)
	m.State.Tree.AppendCommits(entries, hasMore)
}

func (m *App) CommitEntryAt(idx int) (git.Entry, bool) {
	if idx < 0 || idx >= len(m.Data.Visible) {
		return git.Entry{}, false
	}
	entry := m.Data.Visible[idx]
	if entry.Commit.Hash == "" {
		return git.Entry{}, false
	}
	return entry, true
}

func (m *App) CommitEntryForTreeID(id string) (git.Entry, int, bool) {
	idx, ok := m.State.Tree.Rows.VisibleByID[id]
	if !ok {
		return git.Entry{}, 0, false
	}
	entry, ok := m.CommitEntryAt(idx)
	if !ok || entry.Commit.Hash != id {
		return git.Entry{}, 0, false
	}
	return entry, idx, true
}
