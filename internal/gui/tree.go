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
	if a.tree.widget == nil {
		return
	}
	index := 0
	if a.tree.showLocalUnstaged {
		vals := tclList("", localUnstagedLabel, "", "")
		a.tree.widget.Insert("", index, Id(localUnstagedRowID), Values(vals), Tags("localUnstaged"))
		index++
	}
	if a.tree.showLocalStaged {
		vals := tclList("", localStagedLabel, "", "")
		a.tree.widget.Insert("", index, Id(localStagedRowID), Values(vals), Tags("localStaged"))
	}
}

func (a *Controller) onTreeSelectionChanged() {
	if a.tree.widget == nil {
		return
	}
	sel := a.tree.widget.Selection("")
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
	if a.tree.widget == nil {
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
		a.tree.widget.Delete(id)
	}
}

func (a *Controller) insertSingleLocalRow(staged bool) {
	if a.tree.widget == nil {
		return
	}
	label := localRowLabel(staged)
	tag := localRowTag(staged)
	index := 0
	if staged && a.tree.showLocalUnstaged {
		index = 1
	}
	vals := tclList("", label, "", "")
	a.tree.widget.Insert("", index, Id(localRowID(staged)), Values(vals), Tags(tag))
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
	if a.tree.widget == nil || id == "" {
		return false
	}
	out, err := evalext.Eval(fmt.Sprintf("%s exists %s", a.tree.widget, id))
	if err != nil {
		log.Printf("tree exists %s: %v", id, err)
		return false
	}
	return strings.TrimSpace(out) == "1"
}

func (a *Controller) scheduleAutoLoadCheck() {
	if a.tree.widget == nil || a.filter.value == "" || !a.tree.hasMore {
		return
	}
	a.debugf("scheduleAutoLoadCheck: filter=%q visible=%d more=%t", a.filter.value, len(a.visible), a.tree.hasMore)
	PostEvent(func() {
		a.maybeLoadMoreOnScroll()
	}, false)
}

func (a *Controller) maybeLoadMoreOnScroll() {
	if a.tree.widget == nil || a.tree.loadingBatch || !a.tree.hasMore {
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
	if a.filter.value == "" && len(a.visible) >= a.batch && start <= 0 && end >= 1 {
		return
	}
	if end >= autoLoadThreshold {
		a.loadMoreCommitsAsync(false)
	}
}

func (a *Controller) treeYviewRange() (float64, float64, error) {
	if a.tree.widget == nil {
		return 0, 0, fmt.Errorf("tree widget not ready")
	}
	path := a.tree.widget.String()
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
