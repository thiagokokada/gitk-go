package view

import (
	"strings"
	"unicode"

	"github.com/thiagokokada/gitk-go/internal/gui/tkutil"
	tk "modernc.org/tk9.0"
)

func (a *App) BindFilterEntryShortcuts() {
	BindEmacsEntryShortcuts(a.FilterEntry)
}

func BindEmacsEntryShortcuts(entry *tk.TEntryWidget) {
	if entry == nil {
		return
	}
	tk.Bind(entry, "<Control-KeyPress-w>", tk.Command(func(e *tk.Event) { onEntryCtrlW(entry, e) }))
	tk.Bind(entry, "<Control-KeyPress-a>", tk.Command(func(e *tk.Event) { onEntryCtrlA(entry, e) }))
	tk.Bind(entry, "<Control-KeyPress-e>", tk.Command(func(e *tk.Event) { onEntryCtrlE(entry, e) }))
	tk.Bind(entry, "<Control-KeyPress-b>", tk.Command(func(e *tk.Event) { onEntryCtrlB(entry, e) }))
	tk.Bind(entry, "<Control-KeyPress-f>", tk.Command(func(e *tk.Event) { onEntryCtrlF(entry, e) }))
	tk.Bind(entry, "<Control-KeyPress-u>", tk.Command(func(e *tk.Event) { onEntryCtrlU(entry, e) }))
	tk.Bind(entry, "<Control-KeyPress-k>", tk.Command(func(e *tk.Event) { onEntryCtrlK(entry, e) }))
	tk.Bind(entry, "<Control-KeyPress-d>", tk.Command(func(e *tk.Event) { onEntryCtrlD(entry, e) }))
}

func (a *App) FilterText() string {
	return a.FilterEntry.Textvariable()
}

func (a *App) FilterHasFocus() bool {
	return tk.Focus() == a.FilterEntry.String()
}

func (a *App) FocusFilterEntry() {
	if a.FilterHasFocus() {
		return
	}
	tk.Focus(a.FilterEntry)
	tkutil.EvalfOrLog("%s selection range 0 end", a.FilterEntry)
	a.FilterEntry.Icursor("end")
}

func (a *App) BlurFilterEntry() {
	if !a.FilterHasFocus() {
		return
	}
	a.FocusTree()
}

func onEntryCtrlW(entry *tk.TEntryWidget, e *tk.Event) {
	if entry == nil {
		return
	}

	raw := entry.Textvariable()
	cursor := filterEntryCursorIndex(entry, raw)
	selStart, selEnd, hasSelection := filterEntrySelectionRange(entry)
	updated, newCursor, changed := deleteWordBeforeCursor(raw, cursor, selStart, selEnd, hasSelection)
	if !changed {
		e.SetReturnCodeBreak()
		return
	}

	entry.Configure(tk.Textvariable(updated))
	tkutil.EvalfOrLog("%s selection clear", entry)
	entry.Icursor(newCursor)
	e.SetReturnCodeBreak()
}

func onEntryCtrlA(entry *tk.TEntryWidget, e *tk.Event) {
	if entry == nil {
		return
	}

	_, _, hasSelection := filterEntrySelectionRange(entry)
	setFilterEntryCursor(entry, 0, hasSelection)
	e.SetReturnCodeBreak()
}

func onEntryCtrlE(entry *tk.TEntryWidget, e *tk.Event) {
	if entry == nil {
		return
	}

	raw := entry.Textvariable()
	_, _, hasSelection := filterEntrySelectionRange(entry)
	setFilterEntryCursor(entry, len([]rune(raw)), hasSelection)
	e.SetReturnCodeBreak()
}

func onEntryCtrlB(entry *tk.TEntryWidget, e *tk.Event) {
	if entry == nil {
		return
	}

	raw := entry.Textvariable()
	cursor := filterEntryCursorIndex(entry, raw)
	selStart, selEnd, hasSelection := filterEntrySelectionRange(entry)
	newCursor := moveCursorBackward(raw, cursor, selStart, selEnd, hasSelection)
	setFilterEntryCursor(entry, newCursor, hasSelection)
	e.SetReturnCodeBreak()
}

func onEntryCtrlF(entry *tk.TEntryWidget, e *tk.Event) {
	if entry == nil {
		return
	}

	raw := entry.Textvariable()
	cursor := filterEntryCursorIndex(entry, raw)
	selStart, selEnd, hasSelection := filterEntrySelectionRange(entry)
	newCursor := moveCursorForward(raw, cursor, selStart, selEnd, hasSelection)
	setFilterEntryCursor(entry, newCursor, hasSelection)
	e.SetReturnCodeBreak()
}

func onEntryCtrlU(entry *tk.TEntryWidget, e *tk.Event) {
	if entry == nil {
		return
	}

	raw := entry.Textvariable()
	cursor := filterEntryCursorIndex(entry, raw)
	selStart, selEnd, hasSelection := filterEntrySelectionRange(entry)
	updated, newCursor, changed := deleteToStart(raw, cursor, selStart, selEnd, hasSelection)
	if changed {
		updateFilterEntryText(entry, updated, newCursor)
	}
	e.SetReturnCodeBreak()
}

func onEntryCtrlK(entry *tk.TEntryWidget, e *tk.Event) {
	if entry == nil {
		return
	}

	raw := entry.Textvariable()
	cursor := filterEntryCursorIndex(entry, raw)
	selStart, selEnd, hasSelection := filterEntrySelectionRange(entry)
	updated, newCursor, changed := deleteToEnd(raw, cursor, selStart, selEnd, hasSelection)
	if changed {
		updateFilterEntryText(entry, updated, newCursor)
	}
	e.SetReturnCodeBreak()
}

func onEntryCtrlD(entry *tk.TEntryWidget, e *tk.Event) {
	if entry == nil {
		return
	}

	raw := entry.Textvariable()
	cursor := filterEntryCursorIndex(entry, raw)
	selStart, selEnd, hasSelection := filterEntrySelectionRange(entry)
	updated, newCursor, changed := deleteCharAtCursor(raw, cursor, selStart, selEnd, hasSelection)
	if changed {
		updateFilterEntryText(entry, updated, newCursor)
	}
	e.SetReturnCodeBreak()
}

func filterEntryCursorIndex(entry *tk.TEntryWidget, raw string) int {
	out := tkutil.EvalfOrLog("%s index insert", entry)
	if out == "" {
		return len([]rune(raw))
	}
	return tkutil.Atoi(out)
}

func filterEntrySelectionRange(entry *tk.TEntryWidget) (start, end int, ok bool) {
	out := tkutil.EvalfOrLog("%s selection present", entry)
	if out == "" {
		return 0, 0, false
	}
	if strings.TrimSpace(out) != "1" {
		return 0, 0, false
	}
	startRaw := tkutil.EvalfOrLog("%s index sel.first", entry)
	if startRaw == "" {
		return 0, 0, false
	}
	endRaw := tkutil.EvalfOrLog("%s index sel.last", entry)
	if endRaw == "" {
		return 0, 0, false
	}
	return tkutil.Atoi(startRaw), tkutil.Atoi(endRaw), true
}

func setFilterEntryCursor(entry *tk.TEntryWidget, cursor int, clearSelection bool) {
	if clearSelection {
		tkutil.EvalfOrLog("%s selection clear", entry)
	}
	entry.Icursor(cursor)
}

func updateFilterEntryText(entry *tk.TEntryWidget, text string, cursor int) {
	entry.Configure(tk.Textvariable(text))
	setFilterEntryCursor(entry, cursor, true)
}

func deleteWordBeforeCursor(text string, cursor, selStart, selEnd int, hasSelection bool) (string, int, bool) {
	runes := []rune(text)
	lenRunes := len(runes)
	cursor = clampIndex(cursor, lenRunes)
	if hasSelection {
		start, end := normalizeSelection(selStart, selEnd, lenRunes)
		if start < end {
			out := string(append(runes[:start], runes[end:]...))
			return out, start, true
		}
	}
	if cursor == 0 {
		return text, 0, false
	}
	i := cursor
	for i > 0 && unicode.IsSpace(runes[i-1]) {
		i--
	}
	for i > 0 && !unicode.IsSpace(runes[i-1]) {
		i--
	}
	if i == cursor {
		return text, cursor, false
	}
	out := string(append(runes[:i], runes[cursor:]...))
	return out, i, true
}

func moveCursorBackward(text string, cursor, selStart, selEnd int, hasSelection bool) int {
	runes := []rune(text)
	lenRunes := len(runes)
	cursor = clampIndex(cursor, lenRunes)
	if hasSelection {
		start, end := normalizeSelection(selStart, selEnd, lenRunes)
		if start < end {
			return start
		}
	}
	if cursor > 0 {
		return cursor - 1
	}
	return cursor
}

func moveCursorForward(text string, cursor, selStart, selEnd int, hasSelection bool) int {
	runes := []rune(text)
	lenRunes := len(runes)
	cursor = clampIndex(cursor, lenRunes)
	if hasSelection {
		start, end := normalizeSelection(selStart, selEnd, lenRunes)
		if start < end {
			return end
		}
	}
	if cursor < lenRunes {
		return cursor + 1
	}
	return cursor
}

func deleteToStart(text string, cursor, selStart, selEnd int, hasSelection bool) (string, int, bool) {
	runes := []rune(text)
	lenRunes := len(runes)
	cursor = clampIndex(cursor, lenRunes)
	if hasSelection {
		start, end := normalizeSelection(selStart, selEnd, lenRunes)
		if start < end {
			out := string(append(runes[:start], runes[end:]...))
			return out, start, true
		}
	}
	if cursor == 0 {
		return text, 0, false
	}
	out := string(runes[cursor:])
	return out, 0, true
}

func deleteToEnd(text string, cursor, selStart, selEnd int, hasSelection bool) (string, int, bool) {
	runes := []rune(text)
	lenRunes := len(runes)
	cursor = clampIndex(cursor, lenRunes)
	if hasSelection {
		start, end := normalizeSelection(selStart, selEnd, lenRunes)
		if start < end {
			out := string(append(runes[:start], runes[end:]...))
			return out, start, true
		}
	}
	if cursor >= lenRunes {
		return text, cursor, false
	}
	out := string(runes[:cursor])
	return out, cursor, true
}

func deleteCharAtCursor(text string, cursor, selStart, selEnd int, hasSelection bool) (string, int, bool) {
	runes := []rune(text)
	lenRunes := len(runes)
	cursor = clampIndex(cursor, lenRunes)
	if hasSelection {
		start, end := normalizeSelection(selStart, selEnd, lenRunes)
		if start < end {
			out := string(append(runes[:start], runes[end:]...))
			return out, start, true
		}
	}
	if cursor >= lenRunes {
		return text, cursor, false
	}
	out := string(append(runes[:cursor], runes[cursor+1:]...))
	return out, cursor, true
}

func normalizeSelection(start, end, runes int) (s, e int) {
	if start > end {
		start, end = end, start
	}
	return clampIndex(start, runes), clampIndex(end, runes)
}

func clampIndex(idx, runes int) int {
	if idx < 0 {
		return 0
	}
	if idx > runes {
		return runes
	}
	return idx
}
