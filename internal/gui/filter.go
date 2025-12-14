package gui

import (
	"log/slog"
	"strconv"

	"github.com/thiagokokada/gitk-go/internal/debounce"

	. "modernc.org/tk9.0"
)

func (a *Controller) applyFilter(raw string) {
	a.stopFilterDebounce()
	a.filter.value = raw
	a.applyFilterContent(raw)
}

func (a *Controller) applyFilterContent(raw string) {
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
		vals := []string{row.Graph, row.Commit, row.Author, row.Date}
		a.tree.widget.Insert("", "end", Id(row.ID), Values(vals))
	}
	if a.tree.hasMore && len(a.visible) > 0 {
		vals := []string{"", "There are more commits...", "", ""}
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
	slog.Debug("scheduleFilterApply", slog.String("value", raw))
	debouncer := func() *debounce.Debouncer {
		a.filter.mu.Lock()
		defer a.filter.mu.Unlock()
		a.filter.pending = raw
		return debounce.Ensure(&a.filter.debouncer, filterDebounceDelay, func() {
			a.flushFilterDebounce()
		})
	}()
	debouncer.Trigger()
}

func (a *Controller) flushFilterDebounce() {
	value := func() string {
		a.filter.mu.Lock()
		defer a.filter.mu.Unlock()
		val := a.filter.pending
		a.filter.pending = ""
		return val
	}()
	if value == "" {
		return
	}
	PostEvent(func() {
		a.applyFilter(value)
	}, false)
}

func (a *Controller) stopFilterDebounce() {
	a.filter.mu.Lock()
	defer a.filter.mu.Unlock()
	if deb := a.filter.debouncer; deb != nil {
		deb.Stop()
	}
	a.filter.debouncer = nil
	a.filter.pending = ""
}
