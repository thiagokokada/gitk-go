package gui

import (
	"unicode"
)

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
