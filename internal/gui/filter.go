package gui

import (
	"log/slog"
	"strconv"
	"strings"

	"github.com/thiagokokada/gitk-go/internal/debounce"

	. "modernc.org/tk9.0"
)

func (a *Controller) applyFilter(raw string) {
	if a.ui.filterEntry != nil && a.ui.filterEntry.Textvariable() != raw {
		return
	}
	a.filter.value = raw
	a.applyFilterContent(raw)
}

func (a *Controller) applyFilterImmediate(raw string) {
	a.stopFilterDebounce()
	a.applyFilter(raw)
}

func (a *Controller) applyFilterContent(raw string) {
	a.visible = filterEntries(a.commits, raw)
	if a.ui.treeView == nil {
		return
	}

	a.storeScrollState()
	a.clearTreeRows()
	a.insertLocalRows()
	rows := buildTreeRows(a.visible, a.tree.branchLabels)
	for _, row := range rows {
		graph := row.Graph
		if graphCanvasEnabled {
			// Keep the graph column data-less; the canvas overlay renders the graph.
			graph = ""
		}
		vals := []string{graph, row.Commit, row.Author, row.Date}
		a.ui.treeView.Insert("", "end", Id(row.ID), Values(vals))
	}
	if a.tree.hasMore && len(a.visible) > 0 {
		vals := []string{"", "There are more commits...", "", ""}
		a.ui.treeView.Insert("", "end", Id(moreIndicatorID), Values(vals))
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
		a.ui.treeView.Selection("set", id)
		a.ui.treeView.Focus(id)
		a.ui.treeView.See(id)
		a.showCommitDetails(index)
	}
	a.setStatus(a.statusSummary())
	a.scheduleAutoLoadCheck()
	a.restoreScrollState()
	a.scheduleGraphCanvasRedraw()
}

func (a *Controller) storeScrollState() {
	a.scroll.total = a.treeChildCount()
	if a.scroll.total > 0 {
		if start, _, err := a.treeYviewRange(); err == nil {
			a.scroll.start = start
		}
	}
}

func (a *Controller) restoreScrollState() {
	if a.ui.treeView == nil {
		return
	}
	newTotal := a.treeChildCount()
	target, ok := a.scroll.restoreTarget(newTotal)
	if !ok {
		return
	}
	tkMustEval("%s yview moveto %f", a.ui.treeView, target)
}

func (a *Controller) treeChildCount() int {
	if a.ui.treeView == nil {
		return 0
	}
	path := a.ui.treeView.String()
	if path == "" {
		return 0
	}
	out, err := tkEval("llength [%s children {}]", path)
	if err != nil {
		return 0
	}
	count, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0
	}
	return count
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
		a.applyFilterImmediate("")
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

func (s scrollState) restoreTarget(newTotal int) (float64, bool) {
	if s.start < 0 || s.total <= 0 || newTotal <= 0 {
		return 0, false
	}
	target := s.start * float64(s.total) / float64(newTotal)
	target = max(0.0, min(target, 1.0))
	return target, true
}
