package gui

import (
	"strconv"
	"time"

	. "modernc.org/tk9.0"
)

func (a *Controller) applyFilter(raw string) {
	a.stopFilterDebounce()
	a.filter.value = raw
	a.visible = filterEntries(a.commits, raw)
	if a.tree.widget == nil {
		return
	}

	children := a.tree.widget.Children("")
	if len(children) != 0 {
		args := make([]any, len(children))
		for i, child := range children {
			args[i] = child
		}
		a.tree.widget.Delete(args...)
	}
	a.insertLocalRows()
	rows := buildTreeRows(a.visible, a.tree.branchLabels)
	for _, row := range rows {
		vals := tclList(row.Graph, row.Commit, row.Author, row.Date)
		a.tree.widget.Insert("", "end", Id(row.ID), Values(vals))
	}
	if a.tree.hasMore && len(a.visible) > 0 {
		vals := tclList("", "Loading more commits...", "", "")
		a.tree.widget.Insert("", "end", Id(moreIndicatorID), Values(vals))
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

	index := a.visibleSelectionIndex()
	if index < 0 && len(a.visible) > 0 {
		index = 0
	}
	if index >= 0 {
		id := strconv.Itoa(index)
		a.tree.widget.Selection("set", id)
		a.tree.widget.Focus(id)
		a.tree.widget.See(id)
		a.showCommitDetails(index)
	}
	a.setStatus(a.statusSummary())
	a.scheduleAutoLoadCheck()
}

func (a *Controller) visibleSelectionIndex() int {
	hash := a.currentSelection()
	if hash == "" {
		return -1
	}
	for i, entry := range a.visible {
		if entry == nil || entry.Commit == nil {
			continue
		}
		if entry.Commit.Hash.String() == hash {
			return i
		}
	}
	return -1
}

func (a *Controller) scheduleFilterApply(raw string) {
	if raw == "" {
		a.applyFilter("")
		return
	}
	a.debugf("scheduleFilterApply: value=%q", raw)
	var timer *time.Timer
	timer = time.AfterFunc(filterDebounceDelay, func() {
		a.flushFilterDebounce(timer)
	})
	a.filter.mu.Lock()
	defer a.filter.mu.Unlock()
	if current := a.filter.timer; current != nil {
		current.Stop()
	}
	a.filter.pending = raw
	a.filter.timer = timer
}

func (a *Controller) flushFilterDebounce(timer *time.Timer) {
	value, ok := func() (string, bool) {
		a.filter.mu.Lock()
		defer a.filter.mu.Unlock()
		if a.filter.timer != timer {
			return "", false
		}
		val := a.filter.pending
		a.filter.timer = nil
		return val, true
	}()
	if !ok {
		return
	}
	PostEvent(func() {
		a.applyFilter(value)
	}, false)
}

func (a *Controller) stopFilterDebounce() {
	a.filter.mu.Lock()
	defer a.filter.mu.Unlock()
	if timer := a.filter.timer; timer != nil {
		timer.Stop()
		a.filter.timer = nil
	}
}
