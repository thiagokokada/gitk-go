package gui

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	. "modernc.org/tk9.0"
	evalext "modernc.org/tk9.0/extensions/eval"
)

func (a *Controller) insertLocalRows() {
	if a.tree == nil {
		return
	}
	index := 0
	if a.showLocalUnstaged {
		vals := tclList("", localUnstagedLabel, "", "")
		a.tree.Insert("", index, Id(localUnstagedRowID), Values(vals), Tags("localUnstaged"))
		index++
	}
	if a.showLocalStaged {
		vals := tclList("", localStagedLabel, "", "")
		a.tree.Insert("", index, Id(localStagedRowID), Values(vals), Tags("localStaged"))
	}
}

func (a *Controller) onTreeSelectionChanged() {
	if a.tree == nil {
		return
	}
	sel := a.tree.Selection("")
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
		current = a.showLocalStaged
	} else {
		current = a.showLocalUnstaged
	}
	if current == show {
		return
	}
	if staged {
		a.showLocalStaged = show
	} else {
		a.showLocalUnstaged = show
	}
	if a.tree == nil {
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
		a.tree.Delete(id)
	}
}

func (a *Controller) insertSingleLocalRow(staged bool) {
	if a.tree == nil {
		return
	}
	label := localRowLabel(staged)
	tag := localRowTag(staged)
	index := 0
	if staged && a.showLocalUnstaged {
		index = 1
	}
	vals := tclList("", label, "", "")
	a.tree.Insert("", index, Id(localRowID(staged)), Values(vals), Tags(tag))
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
	if a.tree == nil || id == "" {
		return false
	}
	out, err := evalext.Eval(fmt.Sprintf("%s exists %s", a.tree, id))
	if err != nil {
		log.Printf("tree exists %s: %v", id, err)
		return false
	}
	return strings.TrimSpace(out) == "1"
}

func (a *Controller) applyFilter(raw string) {
	a.filterValue = raw
	a.visible = filterEntries(a.commits, raw)
	if a.tree == nil {
		return
	}

	children := a.tree.Children("")
	if len(children) != 0 {
		args := make([]any, len(children))
		for i, child := range children {
			args[i] = child
		}
		a.tree.Delete(args...)
	}
	a.insertLocalRows()
	rows := buildTreeRows(a.visible, a.branchLabels)
	for _, row := range rows {
		vals := tclList(row.Graph, row.Commit, row.Author, row.Date)
		a.tree.Insert("", "end", Id(row.ID), Values(vals))
	}
	if a.hasMore && len(a.visible) > 0 {
		vals := tclList("", "Loading more commits...", "", "")
		a.tree.Insert("", "end", Id(moreIndicatorID), Values(vals))
	}

	if len(a.visible) == 0 {
		if len(a.commits) == 0 {
			a.clearDetailText("Repository has no commits yet.")
		} else {
			a.clearDetailText("No commits match the current filter.")
		}
		a.setStatus(a.statusSummary())
		return
	}

	firstID := strconv.Itoa(0)
	a.tree.Selection("set", firstID)
	a.tree.Focus(firstID)
	a.showCommitDetails(0)
	a.setStatus(a.statusSummary())
	a.scheduleAutoLoadCheck()
}

func (a *Controller) scheduleAutoLoadCheck() {
	if a.tree == nil || a.filterValue == "" || !a.hasMore {
		return
	}
	PostEvent(func() {
		a.maybeLoadMoreOnScroll()
	}, false)
}

func (a *Controller) maybeLoadMoreOnScroll() {
	if a.tree == nil || a.loadingBatch || !a.hasMore {
		return
	}
	if len(a.visible) == 0 {
		return
	}
	start, end, err := a.treeYviewRange()
	if err != nil {
		log.Printf("tree yview: %v", err)
		return
	}
	if a.filterValue == "" && len(a.visible) >= a.batch && start <= 0 && end >= 1 {
		return
	}
	if end >= autoLoadThreshold {
		a.loadMoreCommitsAsync(false)
	}
}

func (a *Controller) treeYviewRange() (float64, float64, error) {
	if a.tree == nil {
		return 0, 0, fmt.Errorf("tree widget not ready")
	}
	path := a.tree.String()
	if path == "" {
		return 0, 0, fmt.Errorf("tree widget has empty path")
	}
	out, err := evalext.Eval(fmt.Sprintf("%s yview", path))
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
