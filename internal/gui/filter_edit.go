package gui

import (
	"log/slog"
	"strings"
	"unicode"

	"github.com/thiagokokada/gitk-go/internal/gui/tkutil"

	. "modernc.org/tk9.0"
)

func (a *Controller) bindEmacsEntryShortcuts(entry *TEntryWidget) {
	if entry == nil {
		return
	}
	Bind(entry, "<Control-KeyPress-w>", Command(func(e *Event) { a.onEntryCtrlW(entry, e) }))
	Bind(entry, "<Control-KeyPress-a>", Command(func(e *Event) { a.onEntryCtrlA(entry, e) }))
	Bind(entry, "<Control-KeyPress-e>", Command(func(e *Event) { a.onEntryCtrlE(entry, e) }))
	Bind(entry, "<Control-KeyPress-b>", Command(func(e *Event) { a.onEntryCtrlB(entry, e) }))
	Bind(entry, "<Control-KeyPress-f>", Command(func(e *Event) { a.onEntryCtrlF(entry, e) }))
	Bind(entry, "<Control-KeyPress-u>", Command(func(e *Event) { a.onEntryCtrlU(entry, e) }))
	Bind(entry, "<Control-KeyPress-k>", Command(func(e *Event) { a.onEntryCtrlK(entry, e) }))
	Bind(entry, "<Control-KeyPress-d>", Command(func(e *Event) { a.onEntryCtrlD(entry, e) }))
}

func (a *Controller) onEntryCtrlW(entry *TEntryWidget, e *Event) {
	if entry == nil {
		return
	}

	raw := entry.Textvariable()
	cursor := a.filterEntryCursorIndex(entry, raw)
	selStart, selEnd, hasSelection := a.filterEntrySelectionRange(entry)
	updated, newCursor, changed := deleteWordBeforeCursor(raw, cursor, selStart, selEnd, hasSelection)
	if !changed {
		e.SetReturnCodeBreak()
		return
	}

	entry.Configure(Textvariable(updated))
	if _, err := tkutil.Evalf("%s selection clear", entry); err != nil {
		slog.Debug("filter selection clear", slog.Any("error", err))
	}
	entry.Icursor(newCursor)
	e.SetReturnCodeBreak()
}

func (a *Controller) onEntryCtrlA(entry *TEntryWidget, e *Event) {
	if entry == nil {
		return
	}

	_, _, hasSelection := a.filterEntrySelectionRange(entry)
	a.setFilterEntryCursor(entry, 0, hasSelection)
	e.SetReturnCodeBreak()
}

func (a *Controller) onEntryCtrlE(entry *TEntryWidget, e *Event) {
	if entry == nil {
		return
	}

	raw := entry.Textvariable()
	_, _, hasSelection := a.filterEntrySelectionRange(entry)
	a.setFilterEntryCursor(entry, len([]rune(raw)), hasSelection)
	e.SetReturnCodeBreak()
}

func (a *Controller) onEntryCtrlB(entry *TEntryWidget, e *Event) {
	if entry == nil {
		return
	}

	raw := entry.Textvariable()
	cursor := a.filterEntryCursorIndex(entry, raw)
	selStart, selEnd, hasSelection := a.filterEntrySelectionRange(entry)
	newCursor := moveCursorBackward(raw, cursor, selStart, selEnd, hasSelection)
	a.setFilterEntryCursor(entry, newCursor, hasSelection)
	e.SetReturnCodeBreak()
}

func (a *Controller) onEntryCtrlF(entry *TEntryWidget, e *Event) {
	if entry == nil {
		return
	}

	raw := entry.Textvariable()
	cursor := a.filterEntryCursorIndex(entry, raw)
	selStart, selEnd, hasSelection := a.filterEntrySelectionRange(entry)
	newCursor := moveCursorForward(raw, cursor, selStart, selEnd, hasSelection)
	a.setFilterEntryCursor(entry, newCursor, hasSelection)
	e.SetReturnCodeBreak()
}

func (a *Controller) onEntryCtrlU(entry *TEntryWidget, e *Event) {
	if entry == nil {
		return
	}

	raw := entry.Textvariable()
	cursor := a.filterEntryCursorIndex(entry, raw)
	selStart, selEnd, hasSelection := a.filterEntrySelectionRange(entry)
	updated, newCursor, changed := deleteToStart(raw, cursor, selStart, selEnd, hasSelection)
	if changed {
		a.updateFilterEntryText(entry, updated, newCursor)
	}
	e.SetReturnCodeBreak()
}

func (a *Controller) onEntryCtrlK(entry *TEntryWidget, e *Event) {
	if entry == nil {
		return
	}

	raw := entry.Textvariable()
	cursor := a.filterEntryCursorIndex(entry, raw)
	selStart, selEnd, hasSelection := a.filterEntrySelectionRange(entry)
	updated, newCursor, changed := deleteToEnd(raw, cursor, selStart, selEnd, hasSelection)
	if changed {
		a.updateFilterEntryText(entry, updated, newCursor)
	}
	e.SetReturnCodeBreak()
}

func (a *Controller) onEntryCtrlD(entry *TEntryWidget, e *Event) {
	if entry == nil {
		return
	}

	raw := entry.Textvariable()
	cursor := a.filterEntryCursorIndex(entry, raw)
	selStart, selEnd, hasSelection := a.filterEntrySelectionRange(entry)
	updated, newCursor, changed := deleteCharAtCursor(raw, cursor, selStart, selEnd, hasSelection)
	if changed {
		a.updateFilterEntryText(entry, updated, newCursor)
	}
	e.SetReturnCodeBreak()
}

func (*Controller) filterEntryCursorIndex(entry *TEntryWidget, raw string) int {
	out, err := tkutil.Evalf("%s index insert", entry)
	if err != nil {
		slog.Debug("filter cursor index", slog.Any("error", err))
		return len([]rune(raw))
	}
	return tkutil.Atoi(out)
}

func (*Controller) filterEntrySelectionRange(entry *TEntryWidget) (start, end int, ok bool) {
	out, err := tkutil.Evalf("%s selection present", entry)
	if err != nil {
		slog.Debug("filter selection present", slog.Any("error", err))
		return 0, 0, false
	}
	if strings.TrimSpace(out) != "1" {
		return 0, 0, false
	}
	startRaw, err := tkutil.Evalf("%s index sel.first", entry)
	if err != nil {
		slog.Debug("filter selection start", slog.Any("error", err))
		return 0, 0, false
	}
	endRaw, err := tkutil.Evalf("%s index sel.last", entry)
	if err != nil {
		slog.Debug("filter selection end", slog.Any("error", err))
		return 0, 0, false
	}
	return tkutil.Atoi(startRaw), tkutil.Atoi(endRaw), true
}

func (*Controller) setFilterEntryCursor(entry *TEntryWidget, cursor int, clearSelection bool) {
	if clearSelection {
		if _, err := tkutil.Evalf("%s selection clear", entry); err != nil {
			slog.Debug("filter selection clear", slog.Any("error", err))
		}
	}
	entry.Icursor(cursor)
}

func (a *Controller) updateFilterEntryText(entry *TEntryWidget, text string, cursor int) {
	entry.Configure(Textvariable(text))
	a.setFilterEntryCursor(entry, cursor, true)
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
