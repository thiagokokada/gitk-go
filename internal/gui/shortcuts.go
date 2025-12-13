package gui

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	. "modernc.org/tk9.0"
)

func (a *Controller) bindShortcuts() {
	if a.tree.widget == nil {
		return
	}
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
			sequences:   []string{"<KeyPress-p>", "<KeyPress-k>"},
			navigation:  true,
			handler:     func() { a.moveSelection(-1) },
		},
		{
			category:    "Commit list",
			display:     "n / j",
			description: "Move down one commit",
			sequences:   []string{"<KeyPress-n>", "<KeyPress-j>"},
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
			display:     "Ctrl+Q",
			description: "Quit gitk-go",
			sequences:   []string{"<Control-KeyPress-q>"},
			navigation:  false,
			handler:     func() { Destroy(App) },
		},
	}
}

func (a *Controller) filterHasFocus() bool {
	if a.filter.entry == nil {
		return false
	}
	return Focus() == a.filter.entry.String()
}

func (a *Controller) showShortcutsDialog() {
	if a.shortcuts.window != nil {
		Destroy(a.shortcuts.window.Window)
		a.shortcuts.window = nil
	}
	dialog := App.Toplevel()
	a.shortcuts.window = dialog
	dialog.Window.WmTitle("Keyboard Shortcuts")
	WmTransient(dialog.Window, App)
	WmAttributes(dialog.Window, "-topmost", 1)

	frame := dialog.TFrame(Padding("12p"))
	Grid(frame, Row(0), Column(0), Sticky(NEWS))
	GridColumnConfigure(frame.Window, 0, Weight(1))
	GridRowConfigure(frame.Window, 1, Weight(1))

	header := frame.TLabel(Txt("Keyboard Shortcuts"), Anchor(W))
	Grid(header, Row(0), Column(0), Sticky(W), Pady("0 8p"))

	text := frame.Text(Width(62), Height(18), Wrap(WORD), Exportselection(false))
	text.Insert("1.0", a.shortcutsHelpText())
	text.Configure(State("disabled"))
	Grid(text, Row(1), Column(0), Sticky(NEWS))

	closeBtn := frame.TButton(Txt("Close"), Command(func() { Destroy(dialog.Window) }))
	Grid(closeBtn, Row(2), Column(0), Sticky(E), Pady("8p 0"))

	Bind(dialog.Window, "<Destroy>", Command(func() {
		if a.shortcuts.window == dialog {
			a.shortcuts.window = nil
		}
	}))
	dialog.Window.Center()
}

func (a *Controller) moveSelection(delta int) {
	if a.tree.widget == nil {
		return
	}
	sel := a.tree.widget.Selection("")
	if len(sel) > 0 {
		if a.handleSpecialRowNav(sel[0], delta) {
			return
		}
	}
	if len(a.visible) == 0 {
		return
	}
	idx := a.currentSelectionIndex() + delta
	if idx < 0 && delta < 0 {
		if a.tree.showLocalStaged {
			a.selectSpecialRow(localStagedRowID)
			return
		}
		if a.tree.showLocalUnstaged {
			a.selectSpecialRow(localUnstagedRowID)
			return
		}
		idx = 0
	}
	if idx < 0 {
		idx = 0
	}
	if idx >= len(a.visible) {
		idx = len(a.visible) - 1
	}
	a.selectTreeIndex(idx)
}

func (a *Controller) selectFirst() {
	if len(a.visible) == 0 {
		return
	}
	a.selectTreeIndex(0)
}

func (a *Controller) selectLast() {
	if len(a.visible) == 0 {
		return
	}
	a.selectTreeIndex(len(a.visible) - 1)
}

func (a *Controller) selectSpecialRow(id string) {
	if a.tree.widget == nil {
		return
	}
	a.tree.widget.Selection("set", id)
	a.tree.widget.Focus(id)
	a.tree.widget.See(id)
	switch id {
	case localUnstagedRowID:
		a.showLocalChanges(false)
	case localStagedRowID:
		a.showLocalChanges(true)
	}
}

func (a *Controller) currentSelectionIndex() int {
	if a.tree.widget == nil {
		return 0
	}
	sel := a.tree.widget.Selection("")
	if len(sel) == 0 || sel[0] == moreIndicatorID {
		return 0
	}
	if idx, err := strconv.Atoi(sel[0]); err == nil {
		return idx
	}
	return 0
}

func (a *Controller) selectTreeIndex(idx int) {
	if a.tree.widget == nil || idx < 0 || idx >= len(a.visible) {
		return
	}
	id := strconv.Itoa(idx)
	a.tree.widget.Selection("set", id)
	a.tree.widget.Focus(id)
	a.tree.widget.See(id)
	a.showCommitDetails(idx)
}

func (a *Controller) handleSpecialRowNav(id string, delta int) bool {
	if delta == 0 {
		return true
	}
	switch id {
	case localUnstagedRowID:
		if delta > 0 {
			if a.tree.showLocalStaged {
				a.selectSpecialRow(localStagedRowID)
			} else if len(a.visible) > 0 {
				a.selectTreeIndex(0)
			}
		}
		return true
	case localStagedRowID:
		if delta < 0 {
			if a.tree.showLocalUnstaged {
				a.selectSpecialRow(localUnstagedRowID)
			}
			return true
		}
		if delta > 0 {
			if len(a.visible) > 0 {
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
	if a.tree.widget == nil || delta == 0 {
		return
	}
	if _, err := tkSafeEval("%s yview scroll %d pages", a.tree.widget, delta); err != nil {
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
	if a.diff.detail == nil || delta == 0 {
		return
	}
	if _, err := tkSafeEval("%s yview scroll %d %s", a.diff.detail, delta, unit); err != nil {
		slog.Error("detail scroll", slog.Any("error", err))
	}
}

func (a *Controller) focusFilterEntry() {
	if a.filter.entry == nil || a.filterHasFocus() {
		return
	}
	if _, err := tkSafeEval("focus %s", a.filter.entry); err != nil {
		slog.Error("focus filter", slog.Any("error", err))
	}
	if _, err := tkSafeEval("%s selection range 0 end", a.filter.entry); err != nil {
		slog.Error("select filter", slog.Any("error", err))
	}
	if _, err := tkSafeEval("%s icursor end", a.filter.entry); err != nil {
		slog.Error("cursor filter", slog.Any("error", err))
	}
}

func (a *Controller) blurFilterEntry() {
	if !a.filterHasFocus() {
		return
	}
	target := App.String()
	if a.tree.widget != nil {
		target = a.tree.widget.String()
	}
	if target == "" {
		target = "."
	}
	if _, err := tkSafeEval("focus %s", target); err != nil {
		slog.Error("blur filter", slog.Any("error", err))
	}
}

func (a *Controller) shortcutsHelpText() string {
	var b strings.Builder
	currentCategory := ""
	for _, sc := range a.shortcutBindings() {
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
