package gui

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/thiagokokada/gitk-go/internal/gui/tkutil"

	. "modernc.org/tk9.0"
)

func (a *Controller) bindShortcuts() {
	bindNav := func(sequence string, handler func()) {
		Bind(App, sequence, Command(func() {
			if a.filterHasFocus() {
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
			handler:     func() { a.scrollTreePages(-1) },
		},
		{
			category:    "Commit list",
			display:     "Ctrl/Cmd + Page Down",
			description: "Scroll commit list down a page",
			sequences:   []string{"<Control-Next>", "<Command-Next>"},
			navigation:  true,
			handler:     func() { a.scrollTreePages(1) },
		},
		{
			category:    "Diff view",
			display:     "Delete / Backspace / b",
			description: "Scroll diff up one page",
			sequences:   []string{"<KeyPress-Delete>", "<KeyPress-BackSpace>", "<KeyPress-b>", "<KeyPress-B>"},
			navigation:  true,
			handler:     func() { a.scrollDetailPages(-1) },
		},
		{
			category:    "Diff view",
			display:     "Space",
			description: "Scroll diff down one page",
			sequences:   []string{"<KeyPress-space>"},
			navigation:  true,
			handler:     func() { a.scrollDetailPages(1) },
		},
		{
			category:    "Diff view",
			display:     "U",
			description: "Scroll diff up 18 lines",
			sequences:   []string{"<KeyPress-u>", "<KeyPress-U>"},
			navigation:  true,
			handler:     func() { a.scrollDetailLines(-18) },
		},
		{
			category:    "Diff view",
			display:     "D",
			description: "Scroll diff down 18 lines",
			sequences:   []string{"<KeyPress-d>", "<KeyPress-D>"},
			navigation:  true,
			handler:     func() { a.scrollDetailLines(18) },
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
			handler:     a.focusFilterEntry,
		},
		{
			category:    "General",
			display:     "Escape",
			description: "Leave the filter box",
			sequences:   []string{"<KeyPress-Escape>"},
			navigation:  false,
			handler:     a.blurFilterEntry,
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
			handler:     a.showShortcutsDialog,
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

func (a *Controller) filterHasFocus() bool {
	return Focus() == a.ui.filterEntry.String()
}

func (a *Controller) showShortcutsDialog() {
	if a.ui.shortcutsWindow != nil {
		Destroy(a.ui.shortcutsWindow.Window)
		a.ui.shortcutsWindow = nil
	}
	dialog := App.Toplevel()
	a.ui.shortcutsWindow = dialog
	dialog.WmTitle("Keyboard Shortcuts")
	WmTransient(dialog.Window, App)
	WmAttributes(dialog.Window, "-topmost", 1)

	frame := dialog.TFrame(Padding("12p"))
	Grid(frame, Row(0), Column(0), Sticky(NEWS))
	GridColumnConfigure(frame.Window, 0, Weight(1))
	GridRowConfigure(frame.Window, 1, Weight(1))

	header := frame.TLabel(Txt("Keyboard Shortcuts"), Anchor(W))
	Grid(header, Row(0), Column(0), Sticky(W), Pady("0 8p"))

	text := frame.Text(Width(62), Height(18), Wrap(WORD), Exportselection(false))
	text.Insert("1.0", formatShortcutsHelpText(a.shortcutBindings()))
	text.Configure(State("disabled"))
	Grid(text, Row(1), Column(0), Sticky(NEWS))

	closeBtn := frame.TButton(Txt("Close"), Command(func() { Destroy(dialog.Window) }))
	Grid(closeBtn, Row(2), Column(0), Sticky(E), Pady("8p 0"))

	Bind(dialog.Window, "<KeyPress-Escape>", Command(func() { Destroy(dialog.Window) }))
	Bind(dialog.Window, "<Destroy>", Command(func() {
		if a.ui.shortcutsWindow == dialog {
			a.ui.shortcutsWindow = nil
		}
	}))
	dialog.Center()
}

func (a *Controller) moveSelection(delta int) {
	sel := a.ui.treeView.Selection("")
	if len(sel) > 0 {
		if a.handleSpecialRowNav(sel[0], delta) {
			return
		}
	}
	if len(a.data.visible) == 0 {
		return
	}
	idx := a.currentSelectionIndex() + delta
	if idx < 0 && delta < 0 {
		if a.state.tree.showLocalStaged {
			a.selectSpecialRow(localStagedRowID)
			return
		}
		if a.state.tree.showLocalUnstaged {
			a.selectSpecialRow(localUnstagedRowID)
			return
		}
	}
	if a.shouldLoadMoreCommits(idx) {
		a.loadMoreCommitsAsync(false)
	}
	idx = max(0, idx)
	a.selectTreeIndex(min(idx, len(a.data.visible)))
}

func (a *Controller) selectFirst() {
	if len(a.data.visible) == 0 {
		return
	}
	a.selectTreeIndex(0)
}

func (a *Controller) selectLast() {
	if len(a.data.visible) == 0 {
		return
	}
	a.selectTreeIndex(len(a.data.visible) - 1)
}

func (a *Controller) selectSpecialRow(id string) {
	a.ui.treeView.Selection("set", id)
	a.ui.treeView.Focus(id)
	a.ui.treeView.See(id)
	switch id {
	case localUnstagedRowID:
		a.showLocalChanges(false)
	case localStagedRowID:
		a.showLocalChanges(true)
	default:
	}
}

func (a *Controller) currentSelectionIndex() int {
	sel := a.ui.treeView.Selection("")
	if len(sel) == 0 || sel[0] == moreIndicatorID {
		return 0
	}
	if idx, err := strconv.Atoi(sel[0]); err == nil {
		return idx
	}
	return 0
}

func (a *Controller) selectTreeIndex(idx int) {
	if idx < 0 || idx >= len(a.data.visible) {
		return
	}
	id := strconv.Itoa(idx)
	a.ui.treeView.Selection("set", id)
	a.ui.treeView.Focus(id)
	a.ui.treeView.See(id)
	a.showCommitDetails(idx)
}

func (a *Controller) handleSpecialRowNav(id string, delta int) bool {
	if delta == 0 {
		return true
	}
	switch id {
	case localUnstagedRowID:
		if delta > 0 {
			if a.state.tree.showLocalStaged {
				a.selectSpecialRow(localStagedRowID)
			} else if len(a.data.visible) > 0 {
				a.selectTreeIndex(0)
			}
		}
		return true
	case localStagedRowID:
		if delta < 0 {
			if a.state.tree.showLocalUnstaged {
				a.selectSpecialRow(localUnstagedRowID)
			}
			return true
		}
		if delta > 0 {
			if len(a.data.visible) > 0 {
				a.selectTreeIndex(0)
			}
			return true
		}
		return true
	default:
		return false
	}
}

func (a *Controller) scrollTreePages(delta int) {
	if delta == 0 {
		return
	}
	if _, err := tkutil.Eval("%s yview scroll %d pages", a.ui.treeView, delta); err != nil {
		slog.Error("tree scroll", slog.Any("error", err))
	}
}

func (a *Controller) scrollDetailPages(delta int) {
	a.scrollDetail(delta, "pages")
}

func (a *Controller) scrollDetailLines(delta int) {
	a.scrollDetail(delta, "units")
}

func (a *Controller) scrollDetail(delta int, unit string) {
	if delta == 0 {
		return
	}
	if _, err := tkutil.Eval("%s yview scroll %d %s", a.ui.diffDetail, delta, unit); err != nil {
		slog.Error("detail scroll", slog.Any("error", err))
	}
}

func (a *Controller) focusFilterEntry() {
	if a.filterHasFocus() {
		return
	}
	if _, err := tkutil.Eval("focus %s", a.ui.filterEntry); err != nil {
		slog.Error("focus filter", slog.Any("error", err))
	}
	if _, err := tkutil.Eval("%s selection range 0 end", a.ui.filterEntry); err != nil {
		slog.Error("select filter", slog.Any("error", err))
	}
	if _, err := tkutil.Eval("%s icursor end", a.ui.filterEntry); err != nil {
		slog.Error("cursor filter", slog.Any("error", err))
	}
}

func (a *Controller) blurFilterEntry() {
	if !a.filterHasFocus() {
		return
	}
	target := a.ui.treeView.String()
	if target == "" {
		target = App.String()
	}
	if target == "" {
		target = "."
	}
	if _, err := tkutil.Eval("focus %s", target); err != nil {
		slog.Error("blur filter", slog.Any("error", err))
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
	return float64(idx)/float64(len(a.data.visible)) >= autoLoadThreshold
}
