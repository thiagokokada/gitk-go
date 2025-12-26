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

func (a *Controller) insertLocalRows() {
	index := 0
	if a.state.tree.showLocalUnstaged {
		vals := []string{"", localUnstagedLabel, "", ""}
		a.ui.treeView.Insert("", index, Id(localUnstagedRowID), Values(vals), Tags("localUnstaged"))
		index++
	}
	if a.state.tree.showLocalStaged {
		vals := []string{"", localStagedLabel, "", ""}
		a.ui.treeView.Insert("", index, Id(localStagedRowID), Values(vals), Tags("localStaged"))
	}
}

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
		if !a.treeItemExists(id) {
			a.insertSingleLocalRow(staged)
		}
		return
	}
	if a.treeItemExists(id) {
		a.ui.treeView.Delete(id)
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

func (a *Controller) treeItemExists(id string) bool {
	if id == "" {
		return false
	}
	out, err := tkutil.Eval("%s exists %s", a.ui.treeView, id)
	if err != nil {
		slog.Error("tree exists", slog.String("id", id), slog.Any("error", err))
		return false
	}
	return strings.TrimSpace(out) == "1"
}

func (a *Controller) clearTreeRows() {
	children := a.ui.treeView.Children("")
	if len(children) == 0 {
		return
	}
	args := make([]any, len(children))
	for i, child := range children {
		args[i] = child
	}
	a.ui.treeView.Delete(args...)
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
	out, err := tkutil.Eval("%s yview", path)
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
	idx, err := strconv.Atoi(id)
	if err != nil {
		return nil, 0, false
	}
	entry, ok := a.commitEntryAt(idx)
	if !ok {
		return nil, 0, false
	}
	return entry, idx, true
}
