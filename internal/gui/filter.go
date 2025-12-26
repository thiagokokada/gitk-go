package gui

import (
	"log/slog"
	"strconv"
	"strings"

	"github.com/thiagokokada/gitk-go/internal/debounce"
	"github.com/thiagokokada/gitk-go/internal/gui/tkutil"

	. "modernc.org/tk9.0"
)

func (a *Controller) applyFilter(raw string) {
	if a.ui.filterEntry != nil && a.ui.filterEntry.Textvariable() != raw {
		return
	}
	a.state.filter.value = raw
	a.applyFilterContent(raw)
}

func (a *Controller) applyFilterImmediate(raw string) {
	a.stopFilterDebounce()
	a.applyFilter(raw)
}

func (a *Controller) applyFilterContent(raw string) {
	a.data.visible = filterEntries(a.data.commits, raw)
	if a.ui.treeView == nil {
		return
	}

	a.storeScrollState()
	a.clearTreeRows()
	a.insertLocalRows()
	rows := buildTreeRows(a.data.visible, a.state.tree.branchLabels, a.cfg.graphCanvas)
	for _, row := range rows {
		graph := row.Graph
		if a.cfg.graphCanvas {
			// Keep the graph column data-less; the canvas overlay renders the graph.
			graph = ""
		}
		vals := []string{graph, row.Commit, row.Author, row.Date}
		a.ui.treeView.Insert("", "end", Id(row.ID), Values(vals))
	}
	if a.state.tree.hasMore && len(a.data.visible) > 0 {
		vals := []string{"", "There are more commits...", "", ""}
		a.ui.treeView.Insert("", "end", Id(moreIndicatorID), Values(vals))
	}

	if len(a.data.visible) == 0 {
		if len(a.data.commits) == 0 {
			a.clearDetailText("Repository has no commits yet.")
		} else {
			a.clearDetailText("No commits match the current filter.")
		}
		a.setStatus(a.statusSummary())
		return
	}

	index := a.visibleSelectionIndex()
	if index < 0 && len(a.data.visible) > 0 {
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
	a.scheduleGraphCanvasDraw()
}

func (a *Controller) storeScrollState() {
	a.state.scroll.total = a.treeChildCount()
	if a.state.scroll.total > 0 {
		if start, _, err := a.treeYviewRange(); err == nil {
			a.state.scroll.start = start
		}
	}
}

func (a *Controller) restoreScrollState() {
	if a.ui.treeView == nil {
		return
	}
	newTotal := a.treeChildCount()
	target, ok := a.state.scroll.restoreTarget(newTotal)
	if !ok {
		return
	}
	tkutil.MustEval("%s yview moveto %f", a.ui.treeView, target)
}

func (a *Controller) treeChildCount() int {
	if a.ui.treeView == nil {
		return 0
	}
	path := a.ui.treeView.String()
	if path == "" {
		return 0
	}
	out, err := tkutil.Eval("llength [%s children {}]", path)
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
	for i, entry := range a.data.visible {
		if entry == nil || entry.Commit == nil {
			continue
		}
		if entry.Commit.Hash == hash {
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
		a.state.filter.mu.Lock()
		defer a.state.filter.mu.Unlock()
		a.state.filter.pending = raw
		return debounce.Ensure(&a.state.filter.debouncer, filterDebounceDelay, func() {
			a.flushFilterDebounce()
		})
	}()
	debouncer.Trigger()
}

func (a *Controller) flushFilterDebounce() {
	value := func() string {
		a.state.filter.mu.Lock()
		defer a.state.filter.mu.Unlock()
		val := a.state.filter.pending
		a.state.filter.pending = ""
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
	a.state.filter.mu.Lock()
	defer a.state.filter.mu.Unlock()
	if deb := a.state.filter.debouncer; deb != nil {
		deb.Stop()
	}
	a.state.filter.debouncer = nil
	a.state.filter.pending = ""
}

func (s scrollState) restoreTarget(newTotal int) (float64, bool) {
	if s.start < 0 || s.total <= 0 || newTotal <= 0 {
		return 0, false
	}
	target := s.start * float64(s.total) / float64(newTotal)
	target = max(0.0, min(target, 1.0))
	return target, true
}
