package gui

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/thiagokokada/gitk-go/internal/gui/model"
	"github.com/thiagokokada/gitk-go/internal/gui/view"

	. "modernc.org/tk9.0"
)

func (a *Controller) bindShortcuts() {
	bindNav := func(sequence string, handler func()) {
		Bind(App, sequence, Command(func() {
			if a.ui.FilterHasFocus() {
				return
			}
			handler()
		}))
	}
	bindAny := func(sequence string, handler func()) {
		Bind(App, sequence, Command(handler))
	}
	for _, sc := range a.shortcutBindings() {
		if sc.handler == nil {
			continue
		}
		for _, seq := range sc.sequences {
			if seq == "" {
				continue
			}
			if sc.navigation {
				bindNav(seq, sc.handler)
			} else {
				bindAny(seq, sc.handler)
			}
		}
	}
}

type shortcutBinding struct {
	sequences   []string
	display     string
	description string
	category    string
	navigation  bool
	handler     func()
}

func (a *Controller) shortcutBindings() []shortcutBinding {
	return []shortcutBinding{
		{
			category:    "Commit list",
			display:     "p / k",
			description: "Move up one commit",
			sequences:   []string{"<KeyPress-Up>", "<KeyPress-p>", "<KeyPress-k>"},
			navigation:  true,
			handler:     func() { a.moveSelection(-1) },
		},
		{
			category:    "Commit list",
			display:     "n / j",
			description: "Move down one commit",
			sequences:   []string{"<KeyPress-Down>", "<KeyPress-n>", "<KeyPress-j>"},
			navigation:  true,
			handler:     func() { a.moveSelection(1) },
		},
		{
			category:    "Commit list",
			display:     "Home",
			description: "Jump to the first commit",
			sequences:   []string{"<KeyPress-Home>"},
			navigation:  true,
			handler:     a.selectFirst,
		},
		{
			category:    "Commit list",
			display:     "End",
			description: "Jump to the last loaded commit",
			sequences:   []string{"<KeyPress-End>"},
			navigation:  true,
			handler:     a.selectLast,
		},
		{
			category:    "Commit list",
			display:     "Ctrl/Cmd + Page Up",
			description: "Scroll commit list up a page",
			sequences:   []string{"<Control-Prior>", "<Command-Prior>"},
			navigation:  true,
			handler:     func() { a.scrollTree(-1, view.ScrollPages) },
		},
		{
			category:    "Commit list",
			display:     "Ctrl/Cmd + Page Down",
			description: "Scroll commit list down a page",
			sequences:   []string{"<Control-Next>", "<Command-Next>"},
			navigation:  true,
			handler:     func() { a.scrollTree(1, view.ScrollPages) },
		},
		{
			category:    "Diff view",
			display:     "Delete / Backspace / b",
			description: "Scroll diff up one page",
			sequences:   []string{"<KeyPress-Delete>", "<KeyPress-BackSpace>", "<KeyPress-b>", "<KeyPress-B>"},
			navigation:  true,
			handler:     func() { a.scrollDetail(-1, view.ScrollPages) },
		},
		{
			category:    "Diff view",
			display:     "Space",
			description: "Scroll diff down one page",
			sequences:   []string{"<KeyPress-space>"},
			navigation:  true,
			handler:     func() { a.scrollDetail(1, view.ScrollPages) },
		},
		{
			category:    "Diff view",
			display:     "U",
			description: "Scroll diff up 18 lines",
			sequences:   []string{"<KeyPress-u>", "<KeyPress-U>"},
			navigation:  true,
			handler:     func() { a.scrollDetail(-18, view.ScrollUnits) },
		},
		{
			category:    "Diff view",
			display:     "D",
			description: "Scroll diff down 18 lines",
			sequences:   []string{"<KeyPress-d>", "<KeyPress-D>"},
			navigation:  true,
			handler:     func() { a.scrollDetail(18, view.ScrollUnits) },
		},
		{
			category:    "Diff view",
			display:     "Ctrl/Cmd + Shift + C",
			description: "Copy selection without +/- markers",
			sequences:   []string{"<Control-Shift-KeyPress-c>", "<Command-Shift-KeyPress-c>"},
			navigation:  false,
			handler:     func() { a.copyDetailSelection(true) },
		},
		{
			category:    "General",
			display:     "/",
			description: "Focus the filter box",
			sequences:   []string{"<KeyPress-/>"},
			navigation:  false,
			handler:     a.ui.FocusFilterEntry,
		},
		{
			category:    "General",
			display:     "Escape",
			description: "Leave the filter box",
			sequences:   []string{"<KeyPress-Escape>"},
			navigation:  false,
			handler:     a.ui.BlurFilterEntry,
		},
		{
			category:    "Filter box",
			display:     "Ctrl+A",
			description: "Move to start of line",
		},
		{
			category:    "Filter box",
			display:     "Ctrl+E",
			description: "Move to end of line",
		},
		{
			category:    "Filter box",
			display:     "Ctrl+B",
			description: "Move back one character",
		},
		{
			category:    "Filter box",
			display:     "Ctrl+F",
			description: "Move forward one character",
		},
		{
			category:    "Filter box",
			display:     "Ctrl+W",
			description: "Delete word before cursor",
		},
		{
			category:    "Filter box",
			display:     "Ctrl+U",
			description: "Delete to start of line",
		},
		{
			category:    "Filter box",
			display:     "Ctrl+K",
			description: "Delete to end of line",
		},
		{
			category:    "Filter box",
			display:     "Ctrl+D",
			description: "Delete character at cursor",
		},
		{
			category:    "General",
			display:     "F5",
			description: "Reload commits",
			sequences:   []string{"<F5>"},
			navigation:  false,
			handler:     a.reloadCommitsAsync,
		},
		{
			category:    "General",
			display:     "F1",
			description: "Show shortcut list",
			sequences:   []string{"<F1>"},
			navigation:  false,
			handler: func() {
				a.ui.ShowShortcutsDialog(formatShortcutsHelpText(a.shortcutBindings()))
			},
		},
		{
			category:    "General",
			display:     "Ctrl/Cmd + O",
			description: "Open repository",
			sequences:   []string{"<Control-KeyPress-o>", "<Command-KeyPress-o>"},
			navigation:  false,
			handler:     a.promptRepositorySwitch,
		},
		{
			category:    "General",
			display:     "Ctrl/Cmd + B",
			description: "Switch branches",
			sequences:   []string{"<Control-KeyPress-b>", "<Command-KeyPress-b>"},
			navigation:  false,
			handler:     a.promptBranchSwitch,
		},
		{
			category:    "Shortcuts dialog",
			display:     "Escape",
			description: "Close this dialog",
			// No handler: Esc is bound on the dialog window so it only closes the
			// dialog when it is open.
		},
		{
			category:    "General",
			display:     "Ctrl+Q",
			description: "Quit gitk-go",
			sequences:   []string{"<Control-KeyPress-q>"},
			navigation:  false,
			handler:     func() { Destroy(App) },
		},
	}
}

func (a *Controller) moveSelection(delta int) {
	if a.handleSpecialRowNav(a.ui.SelectedTreeRow(), delta) {
		return
	}
	if len(a.model.Data.Visible) == 0 {
		return
	}
	idx := a.currentSelectionIndex() + delta
	if idx < 0 && delta < 0 {
		if a.model.State.Tree.LocalRowVisible(true) {
			a.selectSpecialRow(model.LocalStagedRowID)
			return
		}
		if a.model.State.Tree.LocalRowVisible(false) {
			a.selectSpecialRow(model.LocalUnstagedRowID)
			return
		}
	}
	if a.shouldLoadMoreCommits(idx) {
		a.loadMoreCommitsAsync(false)
	}
	idx = max(0, idx)
	a.selectTreeIndex(min(idx, len(a.model.Data.Visible)))
}

func (a *Controller) selectFirst() {
	if len(a.model.Data.Visible) == 0 {
		return
	}
	a.selectTreeIndex(0)
}

func (a *Controller) selectLast() {
	if len(a.model.Data.Visible) == 0 {
		return
	}
	a.selectTreeIndex(len(a.model.Data.Visible) - 1)
}

func (a *Controller) selectSpecialRow(id string) {
	a.ui.SelectTreeRow(id)
	switch id {
	case model.LocalUnstagedRowID:
		a.showLocalChanges(false)
	case model.LocalStagedRowID:
		a.showLocalChanges(true)
	default:
	}
}

func (a *Controller) currentSelectionIndex() int {
	id := a.ui.SelectedTreeRow()
	if _, idx, ok := a.model.CommitEntryForTreeID(id); ok {
		return idx
	}
	return 0
}

func (a *Controller) selectTreeIndex(idx int) {
	entry, ok := a.model.CommitEntryAt(idx)
	if !ok {
		return
	}
	id := entry.Commit.Hash
	if id == "" {
		return
	}
	a.ui.SelectTreeRow(id)
	a.showCommitDetails(entry, idx)
}

func (a *Controller) handleSpecialRowNav(id string, delta int) bool {
	if delta == 0 {
		return true
	}
	switch id {
	case model.LocalUnstagedRowID:
		if delta > 0 {
			if a.model.State.Tree.LocalRowVisible(true) {
				a.selectSpecialRow(model.LocalStagedRowID)
			} else if len(a.model.Data.Visible) > 0 {
				a.selectTreeIndex(0)
			}
		}
		return true
	case model.LocalStagedRowID:
		if delta < 0 {
			if a.model.State.Tree.LocalRowVisible(false) {
				a.selectSpecialRow(model.LocalUnstagedRowID)
			}
			return true
		}
		if delta > 0 {
			if len(a.model.Data.Visible) > 0 {
				a.selectTreeIndex(0)
			}
			return true
		}
		return true
	default:
		return false
	}
}

func (a *Controller) scrollTree(delta int, unit view.ScrollUnit) {
	if err := a.ui.ScrollTreeYview(delta, unit); err != nil {
		slog.Error("tree scroll", slog.Any("error", err))
	}
}

func (a *Controller) scrollTreeUnits(delta int) {
	a.scrollTree(delta, view.ScrollUnits)
}

func (a *Controller) scrollDetail(delta int, unit view.ScrollUnit) {
	if err := a.ui.ScrollDiff(delta, unit); err != nil {
		slog.Error("detail scroll", slog.Any("error", err))
	}
}

func formatShortcutsHelpText(bindings []shortcutBinding) string {
	var b strings.Builder
	currentCategory := ""
	for _, sc := range bindings {
		if sc.category == "" || sc.display == "" || sc.description == "" {
			continue
		}
		if sc.category != currentCategory {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			currentCategory = sc.category
			b.WriteString(currentCategory)
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "  %s — %s\n", sc.display, sc.description)
	}
	return strings.TrimRight(b.String(), "\n")
}

func (a *Controller) shouldLoadMoreCommits(idx int) bool {
	return float64(idx)/float64(len(a.model.Data.Visible)) >= autoLoadThreshold
}
