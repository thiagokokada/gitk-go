package gui

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	. "modernc.org/tk9.0"

	"github.com/thiagokokada/gitk-go/internal/git"
	"github.com/thiagokokada/gitk-go/internal/gui/tkutil"
)

func (a *Controller) onTreeSelectionChanged() {
	a.scheduleGraphCanvasDraw()
	sel := a.ui.treeView.Selection("")
	if len(sel) == 0 {
		a.state.selection.Clear()
		return
	}
	switch sel[0] {
	case moreIndicatorID, loadingIndicatorID:
		a.state.selection.Clear()
		return
	case localUnstagedRowID:
		a.state.selection.SetLocal(false)
		a.showLocalChanges(false)
		return
	case localStagedRowID:
		a.state.selection.SetLocal(true)
		a.showLocalChanges(true)
		return
	}
	entry, idx, ok := a.commitEntryForTreeID(sel[0])
	if !ok {
		a.state.selection.Clear()
		return
	}
	a.showCommitDetails(entry, idx)
}

func (a *Controller) setLocalRowVisibility(staged bool, show bool) {
	var current bool
	if staged {
		current = a.state.tree.showLocalStaged
	} else {
		current = a.state.tree.showLocalUnstaged
	}
	if current == show {
		return
	}
	if staged {
		a.state.tree.showLocalStaged = show
	} else {
		a.state.tree.showLocalUnstaged = show
	}
	id := localRowID(staged)
	if show {
		if !a.state.tree.rows.hasSpecialItem(id) {
			a.insertSingleLocalRow(staged)
		}
		return
	}
	if a.state.tree.rows.hasSpecialItem(id) {
		a.ui.treeView.Delete(id)
		a.state.tree.rows.removeSpecialItem(id)
	}
}

func (a *Controller) insertSingleLocalRow(staged bool) {
	label := localRowLabel(staged)
	tag := localRowTag(staged)
	index := 0
	if staged && a.state.tree.showLocalUnstaged {
		index = 1
	}
	vals := []string{"", label, "", ""}
	a.ui.treeView.Insert("", index, Id(localRowID(staged)), Values(vals), Tags(tag))
	a.state.tree.rows.addSpecialItem(localRowID(staged))
}

func localRowID(staged bool) string {
	if staged {
		return localStagedRowID
	}
	return localUnstagedRowID
}

func localRowLabel(staged bool) string {
	if staged {
		return localStagedLabel
	}
	return localUnstagedLabel
}

func localRowTag(staged bool) string {
	if staged {
		return "localStaged"
	}
	return "localUnstaged"
}

func (a *Controller) clearTreeRows() {
	children := a.ui.treeView.Children("")
	attached := make(map[string]struct{}, len(children))
	if len(children) == 0 {
		children = nil
	}
	if len(children) > 0 {
		args := make([]any, len(children))
		for i, child := range children {
			args[i] = child
			attached[child] = struct{}{}
		}
		a.ui.treeView.Delete(args...)
	}
	for id := range a.state.tree.rows.items {
		if _, ok := attached[id]; ok {
			continue
		}
		a.ui.treeView.Delete(id)
	}
	a.state.tree.rows.items = nil
	a.state.tree.rows.values = nil
	a.state.tree.rows.specialItems = nil
}

func (a *Controller) syncTreeRows() {
	if a.ui.treeView == nil {
		return
	}
	a.ensureLocalRows()
	a.pruneCommitRows()

	refresh := a.state.tree.rows.refreshValues
	ordered := make([]string, 0, len(a.data.visible)+3)
	if a.state.tree.showLocalUnstaged {
		ordered = append(ordered, localUnstagedRowID)
	}
	if a.state.tree.showLocalStaged {
		ordered = append(ordered, localStagedRowID)
	}
	for _, entry := range a.data.visible {
		id := commitRowID(entry)
		if id == "" {
			continue
		}
		if !a.state.tree.rows.hasItem(id) {
			a.insertCommitRow(id, entry)
			a.state.tree.rows.addItem(id)
		} else if refresh {
			a.updateCommitRow(id, entry)
		}
		ordered = append(ordered, id)
	}

	if a.state.tree.hasMore && len(a.data.visible) > 0 {
		a.ensureMoreIndicatorRow()
		ordered = append(ordered, moreIndicatorID)
	}

	if a.state.tree.loadingBatch && len(a.data.visible) == 0 {
		a.ensureLoadingIndicatorRow()
		ordered = append(ordered, loadingIndicatorID)
	}

	a.setTreeChildren(ordered)
	a.state.tree.rows.refreshValues = false
}

func (a *Controller) ensureLocalRows() {
	if a.state.tree.showLocalUnstaged {
		if !a.state.tree.rows.hasSpecialItem(localUnstagedRowID) {
			a.insertSingleLocalRow(false)
		}
	}
	if a.state.tree.showLocalStaged {
		if !a.state.tree.rows.hasSpecialItem(localStagedRowID) {
			a.insertSingleLocalRow(true)
		}
	}
}

func (a *Controller) ensureMoreIndicatorRow() {
	if a.state.tree.rows.hasSpecialItem(moreIndicatorID) {
		return
	}
	vals := []string{"", "There are more commits...", "", ""}
	a.ui.treeView.Insert("", "end", Id(moreIndicatorID), Values(vals))
	a.state.tree.rows.addSpecialItem(moreIndicatorID)
}

func (a *Controller) ensureLoadingIndicatorRow() {
	if a.state.tree.rows.hasSpecialItem(loadingIndicatorID) {
		return
	}
	vals := []string{"", "Loading commits...", "", ""}
	a.ui.treeView.Insert("", "end", Id(loadingIndicatorID), Values(vals))
	a.state.tree.rows.addSpecialItem(loadingIndicatorID)
}

func (a *Controller) insertCommitRow(id string, entry *git.Entry) {
	row, ok := treeRowData(entry, a.state.tree.branchLabels, a.cfg.graphCanvas)
	if !ok {
		return
	}
	a.ui.treeView.Insert("", "end", Id(id), Values(row.values()))
	a.state.tree.rows.setItemValue(id, row)
}

func (a *Controller) updateCommitRow(id string, entry *git.Entry) {
	treePath := a.ui.treeView.String()
	if treePath == "" {
		return
	}
	row, ok := treeRowData(entry, a.state.tree.branchLabels, a.cfg.graphCanvas)
	if !ok {
		return
	}
	if !a.state.tree.rows.itemValueChanged(id, row) {
		return
	}
	if _, err := tkutil.Evalf("%s item {%s} %s", treePath, id, Values(row.values())); err != nil {
		slog.Debug("tree item update", slog.Any("error", err))
		return
	}
	a.state.tree.rows.setItemValue(id, row)
}

func (a *Controller) setTreeChildren(ids []string) {
	treePath := a.ui.treeView.String()
	if treePath == "" {
		return
	}
	if len(ids) == 0 {
		if _, err := tkutil.Evalf("%s children {} {}", treePath); err != nil {
			slog.Debug("tree children clear", slog.Any("error", err))
		}
		return
	}
	children := tkutil.TclSafeStrings(ids...)
	if _, err := tkutil.Evalf("%s children {} {%s}", treePath, children); err != nil {
		slog.Debug("tree children set", slog.Any("error", err))
	}
}

func (a *Controller) pruneCommitRows() {
	if len(a.state.tree.rows.items) == 0 || a.state.tree.rows.commitIDs == nil {
		return
	}
	for id := range a.state.tree.rows.items {
		if _, ok := a.state.tree.rows.commitIDs[id]; ok {
			continue
		}
		a.ui.treeView.Delete(id)
		delete(a.state.tree.rows.items, id)
		delete(a.state.tree.rows.values, id)
	}
}

func (a *Controller) scheduleAutoLoadCheck() {
	if a.state.filter.value == "" || !a.state.tree.hasMore {
		return
	}
	slog.Debug("scheduleAutoLoadCheck",
		slog.String("filter", a.state.filter.value),
		slog.Int("visible", len(a.data.visible)),
		slog.Bool("has_more", a.state.tree.hasMore),
	)
	PostEvent(func() {
		a.maybeLoadMoreOnScroll()
	}, false)
}

func (a *Controller) maybeLoadMoreOnScroll() {
	if a.state.tree.loadingBatch || !a.state.tree.hasMore {
		return
	}
	start, end, err := a.treeYviewRange()
	if err != nil {
		slog.Error("tree yview", slog.Any("error", err))
		return
	}
	if a.state.tree.shouldLoadMoreOnScroll(a.state.filter.value, len(a.data.visible), int(a.cfg.batch), start, end) {
		a.loadMoreCommitsAsync(false)
	}
}

func (a *Controller) treeYviewRange() (start float64, end float64, err error) {
	path := a.ui.treeView.String()
	if path == "" {
		return 0, 0, fmt.Errorf("tree widget has empty path")
	}
	out, err := tkutil.Evalf("%s yview", path)
	if err != nil {
		return 0, 0, err
	}
	fields := strings.Fields(strings.TrimSpace(out))
	if len(fields) < 2 {
		return 0, 0, fmt.Errorf("unexpected treeview yview output %q", out)
	}
	start, err = strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, 0, err
	}
	end, err = strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return 0, 0, err
	}
	return start, end, nil
}

func (t treeState) shouldLoadMoreOnScroll(
	filterValue string,
	visibleLen int,
	batch int,
	yStart float64,
	yEnd float64,
) bool {
	if t.loadingBatch || !t.hasMore {
		return false
	}
	if visibleLen == 0 {
		return true
	}
	if filterValue == "" && visibleLen >= batch && yStart <= 0 && yEnd >= 1 {
		return false
	}
	return yEnd >= autoLoadThreshold
}

func (a *Controller) commitEntryAt(idx int) (*git.Entry, bool) {
	if idx < 0 || idx >= len(a.data.visible) {
		return nil, false
	}
	entry := a.data.visible[idx]
	if entry == nil || entry.Commit == nil {
		return nil, false
	}
	return entry, true
}

func (a *Controller) commitEntryForTreeID(id string) (*git.Entry, int, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, 0, false
	}
	idx, ok := a.state.tree.rows.visibleByID[id]
	if !ok {
		return nil, 0, false
	}
	entry, ok := a.commitEntryAt(idx)
	if !ok || entry.Commit == nil || entry.Commit.Hash != id {
		return nil, 0, false
	}
	return entry, idx, true
}

func commitRowID(entry *git.Entry) string {
	if entry == nil || entry.Commit == nil {
		return ""
	}
	return entry.Commit.Hash
}

func buildVisibleIndex(entries []*git.Entry) map[string]int {
	return buildVisibleIndexInto(entries, nil)
}

func buildVisibleIndexInto(entries []*git.Entry, index map[string]int) map[string]int {
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
		id := commitRowID(entry)
		if id == "" {
			continue
		}
		index[id] = i
	}
	return index
}

func buildCommitIDSet(entries []*git.Entry) map[string]struct{} {
	if len(entries) == 0 {
		return nil
	}
	ids := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		id := commitRowID(entry)
		if id == "" {
			continue
		}
		ids[id] = struct{}{}
	}
	return ids
}

func (s *treeRowState) setCommitIDs(entries []*git.Entry) {
	if len(entries) == 0 {
		s.commitIDs = map[string]struct{}{}
		return
	}
	s.commitIDs = buildCommitIDSet(entries)
}

func (s *treeRowState) addCommitIDs(entries []*git.Entry) {
	if len(entries) == 0 {
		return
	}
	if s.commitIDs == nil {
		s.commitIDs = make(map[string]struct{}, len(entries))
	}
	for _, entry := range entries {
		id := commitRowID(entry)
		if id == "" {
			continue
		}
		s.commitIDs[id] = struct{}{}
	}
}

func (s *treeRowState) setVisibleIndex(entries []*git.Entry) {
	s.visibleByID = buildVisibleIndexInto(entries, s.visibleByID)
}

func (s *treeRowState) hasItem(id string) bool {
	if s.items == nil {
		return false
	}
	_, ok := s.items[id]
	return ok
}

func (s *treeRowState) addItem(id string) {
	if id == "" {
		return
	}
	if s.items == nil {
		s.items = make(map[string]struct{})
	}
	s.items[id] = struct{}{}
}

func (s *treeRowState) itemValueChanged(id string, row treeRow) bool {
	if s.values == nil {
		return true
	}
	prev, ok := s.values[id]
	if !ok {
		return true
	}
	return !treeRowEqual(prev, row)
}

func (s *treeRowState) setItemValue(id string, row treeRow) {
	if id == "" {
		return
	}
	if s.values == nil {
		s.values = make(map[string]treeRow)
	}
	s.values[id] = row
}

func (s *treeRowState) hasSpecialItem(id string) bool {
	if s.specialItems == nil {
		return false
	}
	_, ok := s.specialItems[id]
	return ok
}

func (s *treeRowState) addSpecialItem(id string) {
	if id == "" {
		return
	}
	if s.specialItems == nil {
		s.specialItems = make(map[string]struct{})
	}
	s.specialItems[id] = struct{}{}
}

func (s *treeRowState) removeSpecialItem(id string) {
	if s.specialItems == nil {
		return
	}
	delete(s.specialItems, id)
}

func treeRowEqual(a treeRow, b treeRow) bool {
	return a.Graph == b.Graph &&
		a.Commit == b.Commit &&
		a.Author == b.Author &&
		a.Date == b.Date
}
