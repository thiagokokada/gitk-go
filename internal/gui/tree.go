package gui

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	. "modernc.org/tk9.0"
)

func (a *Controller) insertLocalRows() {
	if a.ui.treeView == nil {
		return
	}
	index := 0
	if a.tree.showLocalUnstaged {
		vals := []string{"", localUnstagedLabel, "", ""}
		a.ui.treeView.Insert("", index, Id(localUnstagedRowID), Values(vals), Tags("localUnstaged"))
		index++
	}
	if a.tree.showLocalStaged {
		vals := []string{"", localStagedLabel, "", ""}
		a.ui.treeView.Insert("", index, Id(localStagedRowID), Values(vals), Tags("localStaged"))
	}
}

func (a *Controller) onTreeSelectionChanged() {
	if a.ui.treeView == nil {
		return
	}
	sel := a.ui.treeView.Selection("")
	if len(sel) == 0 {
		return
	}
	switch sel[0] {
	case moreIndicatorID:
		return
	case loadingIndicatorID:
		return
	case localUnstagedRowID:
		a.showLocalChanges(false)
		return
	case localStagedRowID:
		a.showLocalChanges(true)
		return
	}
	idx, err := strconv.Atoi(sel[0])
	if err != nil || idx < 0 || idx >= len(a.visible) {
		return
	}
	a.showCommitDetails(idx)
}

func (a *Controller) setLocalRowVisibility(staged bool, show bool) {
	var current bool
	if staged {
		current = a.tree.showLocalStaged
	} else {
		current = a.tree.showLocalUnstaged
	}
	if current == show {
		return
	}
	if staged {
		a.tree.showLocalStaged = show
	} else {
		a.tree.showLocalUnstaged = show
	}
	if a.ui.treeView == nil {
		return
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
	if a.ui.treeView == nil {
		return
	}
	label := localRowLabel(staged)
	tag := localRowTag(staged)
	index := 0
	if staged && a.tree.showLocalUnstaged {
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
	if a.ui.treeView == nil || id == "" {
		return false
	}
	out, err := tkEval("%s exists %s", a.ui.treeView, id)
	if err != nil {
		slog.Error("tree exists", slog.String("id", id), slog.Any("error", err))
		return false
	}
	return strings.TrimSpace(out) == "1"
}

func (a *Controller) clearTreeRows() {
	if a.ui.treeView == nil {
		return
	}
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
	if a.ui.treeView == nil || a.filter.value == "" || !a.tree.hasMore {
		return
	}
	slog.Debug("scheduleAutoLoadCheck",
		slog.String("filter", a.filter.value),
		slog.Int("visible", len(a.visible)),
		slog.Bool("has_more", a.tree.hasMore),
	)
	PostEvent(func() {
		a.maybeLoadMoreOnScroll()
	}, false)
}

func (a *Controller) maybeLoadMoreOnScroll() {
	if a.ui.treeView == nil || a.tree.loadingBatch || !a.tree.hasMore {
		return
	}
	if len(a.visible) == 0 {
		a.loadMoreCommitsAsync(false)
		return
	}
	start, end, err := a.treeYviewRange()
	if err != nil {
		slog.Error("tree yview", slog.Any("error", err))
		return
	}
	if a.filter.value == "" && len(a.visible) >= a.batch && start <= 0 && end >= 1 {
		return
	}
	if end >= autoLoadThreshold {
		a.loadMoreCommitsAsync(false)
	}
}

func (a *Controller) treeYviewRange() (float64, float64, error) {
	if a.ui.treeView == nil {
		return 0, 0, fmt.Errorf("tree widget not ready")
	}
	path := a.ui.treeView.String()
	if path == "" {
		return 0, 0, fmt.Errorf("tree widget has empty path")
	}
	out, err := tkEval("%s yview", path)
	if err != nil {
		return 0, 0, err
	}
	fields := strings.Fields(strings.TrimSpace(out))
	if len(fields) < 2 {
		return 0, 0, fmt.Errorf("unexpected treeview yview output %q", out)
	}
	start, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, 0, err
	}
	end, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return 0, 0, err
	}
	return start, end, nil
}
