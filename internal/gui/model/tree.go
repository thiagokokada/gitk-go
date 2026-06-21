package model

import "github.com/thiagokokada/gitk-go/internal/git"

type TreeState struct {
	BranchLabels      map[string][]string
	ContextTargetID   string
	HasMore           bool
	LoadingBatch      bool
	ShowLocalUnstaged bool
	ShowLocalStaged   bool

	Rows TreeRowState
}

func NewTreeState() TreeState {
	return TreeState{BranchLabels: make(map[string][]string)}
}

func (t *TreeState) SetReloadedCommits(entries []git.Entry, hasMore bool) {
	t.HasMore = hasMore
	t.Rows.SetCommitIDs(entries)
	t.Rows.RefreshValues = true
}

func (t *TreeState) AppendCommits(entries []git.Entry, hasMore bool) {
	t.HasMore = hasMore
	t.Rows.AddCommitIDs(entries)
	t.Rows.RefreshValues = true
}

func (t *TreeState) MarkNoMoreCommits() {
	t.HasMore = false
}

func (t *TreeState) BeginCommitBatchLoad(prefetch bool) bool {
	if t.LoadingBatch || (!prefetch && !t.HasMore) {
		return false
	}
	t.LoadingBatch = true
	return true
}

func (t *TreeState) FinishCommitBatchLoad() {
	t.LoadingBatch = false
}

func (t *TreeState) SetLocalRowVisible(staged bool, show bool) bool {
	if staged {
		if t.ShowLocalStaged == show {
			return false
		}
		t.ShowLocalStaged = show
		return true
	}
	if t.ShowLocalUnstaged == show {
		return false
	}
	t.ShowLocalUnstaged = show
	return true
}

func (t TreeState) LocalRowVisible(staged bool) bool {
	if staged {
		return t.ShowLocalStaged
	}
	return t.ShowLocalUnstaged
}

func (t TreeState) LocalRowInsertIndex(staged bool) int {
	if staged && t.ShowLocalUnstaged {
		return 1
	}
	return 0
}

func (t TreeState) LocalRowIDs() []string {
	ids := make([]string, 0, 2)
	if t.ShowLocalUnstaged {
		ids = append(ids, LocalUnstagedRowID)
	}
	if t.ShowLocalStaged {
		ids = append(ids, LocalStagedRowID)
	}
	return ids
}

func (t TreeState) ShouldLoadMoreOnScroll(
	filterValue string,
	visibleLen int,
	batch int,
	yStart float64,
	yEnd float64,
) bool {
	if t.LoadingBatch || !t.HasMore {
		return false
	}
	if visibleLen == 0 {
		return true
	}
	if filterValue == "" && visibleLen >= batch && yStart <= 0 && yEnd >= 1 {
		return false
	}
	return yEnd >= 0.98
}

type LocalChangeActions struct {
	ShowUnstaged  bool
	ShowStaged    bool
	ResetUnstaged bool
	ResetStaged   bool
	LoadUnstaged  bool
	LoadStaged    bool
}

func (t TreeState) LocalChangePlan(repoReady bool, prefetch bool, status git.LocalChanges) LocalChangeActions {
	if !repoReady {
		return LocalChangeActions{
			ShowUnstaged:  false,
			ShowStaged:    false,
			ResetUnstaged: true,
			ResetStaged:   true,
		}
	}
	prevUnstaged := t.LocalRowVisible(false)
	prevStaged := t.LocalRowVisible(true)
	actions := LocalChangeActions{
		ShowUnstaged:  status.HasWorktree,
		ShowStaged:    status.HasStaged,
		ResetUnstaged: !status.HasWorktree,
		ResetStaged:   !status.HasStaged,
	}
	if prefetch {
		actions.LoadUnstaged = status.HasWorktree
		actions.LoadStaged = status.HasStaged
		return actions
	}
	actions.LoadUnstaged = status.HasWorktree && !prevUnstaged
	actions.LoadStaged = status.HasStaged && !prevStaged
	return actions
}

func LocalRowID(staged bool) string {
	if staged {
		return LocalStagedRowID
	}
	return LocalUnstagedRowID
}

type TreeRow struct {
	ID     string
	Graph  string
	Commit string
	Author string
	Date   string
}

func (r TreeRow) Values() []string {
	return []string{r.Graph, r.Commit, r.Author, r.Date}
}

type TreeRowState struct {
	CommitIDs     map[string]struct{}
	VisibleByID   map[string]int
	Items         map[string]struct{}
	Values        map[string]TreeRow
	SpecialItems  map[string]struct{}
	RefreshValues bool
}

func (s *TreeRowState) ResetTracking() {
	s.Items = nil
	s.Values = nil
	s.SpecialItems = nil
}

func (s TreeRowState) TrackedItemIDs() []string {
	if len(s.Items) == 0 && len(s.SpecialItems) == 0 {
		return nil
	}
	ids := make([]string, 0, len(s.Items)+len(s.SpecialItems))
	for id := range s.Items {
		ids = append(ids, id)
	}
	for id := range s.SpecialItems {
		ids = append(ids, id)
	}
	return ids
}

func (s *TreeRowState) PruneStaleCommitRows() []string {
	if len(s.Items) == 0 || s.CommitIDs == nil {
		return nil
	}
	ids := make([]string, 0)
	for id := range s.Items {
		if _, ok := s.CommitIDs[id]; ok {
			continue
		}
		ids = append(ids, id)
		delete(s.Items, id)
		delete(s.Values, id)
	}
	return ids
}

func BuildVisibleIndex(entries []git.Entry) map[string]int {
	return BuildVisibleIndexInto(entries, nil)
}

func BuildVisibleIndexInto(entries []git.Entry, index map[string]int) map[string]int {
	if len(entries) == 0 {
		if index == nil {
			return nil
		}
		for k := range index {
			delete(index, k)
		}
		return index
	}
	if index == nil {
		index = make(map[string]int, len(entries))
	} else {
		for k := range index {
			delete(index, k)
		}
	}
	for i, entry := range entries {
		id := entry.Commit.Hash
		if id == "" {
			continue
		}
		index[id] = i
	}
	return index
}

func BuildCommitIDSet(entries []git.Entry) map[string]struct{} {
	if len(entries) == 0 {
		return nil
	}
	ids := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		id := entry.Commit.Hash
		if id == "" {
			continue
		}
		ids[id] = struct{}{}
	}
	return ids
}

func (s *TreeRowState) SetCommitIDs(entries []git.Entry) {
	if len(entries) == 0 {
		s.CommitIDs = map[string]struct{}{}
		return
	}
	s.CommitIDs = BuildCommitIDSet(entries)
}

func (s *TreeRowState) AddCommitIDs(entries []git.Entry) {
	if len(entries) == 0 {
		return
	}
	if s.CommitIDs == nil {
		s.CommitIDs = make(map[string]struct{}, len(entries))
	}
	for _, entry := range entries {
		id := entry.Commit.Hash
		if id == "" {
			continue
		}
		s.CommitIDs[id] = struct{}{}
	}
}

func (s *TreeRowState) SetVisibleIndex(entries []git.Entry) {
	s.VisibleByID = BuildVisibleIndexInto(entries, s.VisibleByID)
}

func (s *TreeRowState) HasItem(id string) bool {
	if s.Items == nil {
		return false
	}
	_, ok := s.Items[id]
	return ok
}

func (s *TreeRowState) AddItem(id string) {
	if id == "" {
		return
	}
	if s.Items == nil {
		s.Items = make(map[string]struct{})
	}
	s.Items[id] = struct{}{}
}

func (s *TreeRowState) ItemValueChanged(id string, row TreeRow) bool {
	if s.Values == nil {
		return true
	}
	prev, ok := s.Values[id]
	if !ok {
		return true
	}
	return !TreeRowEqual(prev, row)
}

func (s *TreeRowState) SetItemValue(id string, row TreeRow) {
	if id == "" {
		return
	}
	if s.Values == nil {
		s.Values = make(map[string]TreeRow)
	}
	s.Values[id] = row
}

func (s *TreeRowState) HasSpecialItem(id string) bool {
	if s.SpecialItems == nil {
		return false
	}
	_, ok := s.SpecialItems[id]
	return ok
}

func (s *TreeRowState) AddSpecialItem(id string) {
	if id == "" {
		return
	}
	if s.SpecialItems == nil {
		s.SpecialItems = make(map[string]struct{})
	}
	s.SpecialItems[id] = struct{}{}
}

func (s *TreeRowState) RemoveSpecialItem(id string) {
	if s.SpecialItems == nil {
		return
	}
	delete(s.SpecialItems, id)
}

func TreeRowEqual(a TreeRow, b TreeRow) bool {
	return a.Graph == b.Graph &&
		a.Commit == b.Commit &&
		a.Author == b.Author &&
		a.Date == b.Date
}
