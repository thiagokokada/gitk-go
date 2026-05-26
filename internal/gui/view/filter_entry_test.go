package view

import "testing"

func TestDeleteWordBeforeCursor(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		cursor       int
		selStart     int
		selEnd       int
		hasSelection bool
		wantText     string
		wantCursor   int
		wantChanged  bool
	}{
		{
			name:         "delete selection",
			text:         "foo bar",
			cursor:       7,
			selStart:     0,
			selEnd:       3,
			hasSelection: true,
			wantText:     " bar",
			wantCursor:   0,
			wantChanged:  true,
		},
		{
			name:        "delete word before cursor",
			text:        "foo bar",
			cursor:      7,
			wantText:    "foo ",
			wantCursor:  4,
			wantChanged: true,
		},
		{
			name:        "delete whitespace then word",
			text:        "foo  bar",
			cursor:      4,
			wantText:    " bar",
			wantCursor:  0,
			wantChanged: true,
		},
		{
			name:        "at start does nothing",
			text:        "foo",
			cursor:      0,
			wantText:    "foo",
			wantCursor:  0,
			wantChanged: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotText, gotCursor, gotChanged := deleteWordBeforeCursor(
				tc.text,
				tc.cursor,
				tc.selStart,
				tc.selEnd,
				tc.hasSelection,
			)
			if gotText != tc.wantText || gotCursor != tc.wantCursor || gotChanged != tc.wantChanged {
				t.Fatalf(
					"got (%q, %d, %v), want (%q, %d, %v)",
					gotText,
					gotCursor,
					gotChanged,
					tc.wantText,
					tc.wantCursor,
					tc.wantChanged,
				)
			}
		})
	}
}

func TestMoveCursorBackward(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		cursor       int
		selStart     int
		selEnd       int
		hasSelection bool
		wantCursor   int
	}{
		{
			name:       "move back one rune",
			text:       "ab",
			cursor:     2,
			wantCursor: 1,
		},
		{
			name:       "at start stays",
			text:       "ab",
			cursor:     0,
			wantCursor: 0,
		},
		{
			name:         "selection collapses to start",
			text:         "hello",
			cursor:       5,
			selStart:     1,
			selEnd:       4,
			hasSelection: true,
			wantCursor:   1,
		},
		{
			name:         "selection reversed collapses to start",
			text:         "hello",
			cursor:       5,
			selStart:     4,
			selEnd:       1,
			hasSelection: true,
			wantCursor:   1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := moveCursorBackward(tc.text, tc.cursor, tc.selStart, tc.selEnd, tc.hasSelection)
			if got != tc.wantCursor {
				t.Fatalf("got %d, want %d", got, tc.wantCursor)
			}
		})
	}
}

func TestMoveCursorForward(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		cursor       int
		selStart     int
		selEnd       int
		hasSelection bool
		wantCursor   int
	}{
		{
			name:       "move forward one rune",
			text:       "ab",
			cursor:     0,
			wantCursor: 1,
		},
		{
			name:       "at end stays",
			text:       "ab",
			cursor:     2,
			wantCursor: 2,
		},
		{
			name:         "selection collapses to end",
			text:         "hello",
			cursor:       0,
			selStart:     1,
			selEnd:       4,
			hasSelection: true,
			wantCursor:   4,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := moveCursorForward(tc.text, tc.cursor, tc.selStart, tc.selEnd, tc.hasSelection)
			if got != tc.wantCursor {
				t.Fatalf("got %d, want %d", got, tc.wantCursor)
			}
		})
	}
}

func TestDeleteToStart(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		cursor       int
		selStart     int
		selEnd       int
		hasSelection bool
		wantText     string
		wantCursor   int
		wantChanged  bool
	}{
		{
			name:        "delete prefix",
			text:        "hello",
			cursor:      2,
			wantText:    "llo",
			wantCursor:  0,
			wantChanged: true,
		},
		{
			name:         "delete selection",
			text:         "hello",
			cursor:       5,
			selStart:     1,
			selEnd:       4,
			hasSelection: true,
			wantText:     "ho",
			wantCursor:   1,
			wantChanged:  true,
		},
		{
			name:        "at start no change",
			text:        "hello",
			cursor:      0,
			wantText:    "hello",
			wantCursor:  0,
			wantChanged: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotText, gotCursor, gotChanged := deleteToStart(
				tc.text,
				tc.cursor,
				tc.selStart,
				tc.selEnd,
				tc.hasSelection,
			)
			if gotText != tc.wantText || gotCursor != tc.wantCursor || gotChanged != tc.wantChanged {
				t.Fatalf(
					"got (%q, %d, %v), want (%q, %d, %v)",
					gotText,
					gotCursor,
					gotChanged,
					tc.wantText,
					tc.wantCursor,
					tc.wantChanged,
				)
			}
		})
	}
}

func TestDeleteToEnd(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		cursor       int
		selStart     int
		selEnd       int
		hasSelection bool
		wantText     string
		wantCursor   int
		wantChanged  bool
	}{
		{
			name:        "delete suffix",
			text:        "hello",
			cursor:      2,
			wantText:    "he",
			wantCursor:  2,
			wantChanged: true,
		},
		{
			name:         "delete selection",
			text:         "hello",
			cursor:       0,
			selStart:     1,
			selEnd:       4,
			hasSelection: true,
			wantText:     "ho",
			wantCursor:   1,
			wantChanged:  true,
		},
		{
			name:        "at end no change",
			text:        "hello",
			cursor:      5,
			wantText:    "hello",
			wantCursor:  5,
			wantChanged: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotText, gotCursor, gotChanged := deleteToEnd(
				tc.text,
				tc.cursor,
				tc.selStart,
				tc.selEnd,
				tc.hasSelection,
			)
			if gotText != tc.wantText || gotCursor != tc.wantCursor || gotChanged != tc.wantChanged {
				t.Fatalf(
					"got (%q, %d, %v), want (%q, %d, %v)",
					gotText,
					gotCursor,
					gotChanged,
					tc.wantText,
					tc.wantCursor,
					tc.wantChanged,
				)
			}
		})
	}
}

func TestDeleteCharAtCursor(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		cursor       int
		selStart     int
		selEnd       int
		hasSelection bool
		wantText     string
		wantCursor   int
		wantChanged  bool
	}{
		{
			name:        "delete char",
			text:        "hello",
			cursor:      1,
			wantText:    "hllo",
			wantCursor:  1,
			wantChanged: true,
		},
		{
			name:         "delete selection",
			text:         "hello",
			cursor:       0,
			selStart:     1,
			selEnd:       4,
			hasSelection: true,
			wantText:     "ho",
			wantCursor:   1,
			wantChanged:  true,
		},
		{
			name:        "at end no change",
			text:        "hello",
			cursor:      5,
			wantText:    "hello",
			wantCursor:  5,
			wantChanged: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotText, gotCursor, gotChanged := deleteCharAtCursor(
				tc.text,
				tc.cursor,
				tc.selStart,
				tc.selEnd,
				tc.hasSelection,
			)
			if gotText != tc.wantText || gotCursor != tc.wantCursor || gotChanged != tc.wantChanged {
				t.Fatalf(
					"got (%q, %d, %v), want (%q, %d, %v)",
					gotText,
					gotCursor,
					gotChanged,
					tc.wantText,
					tc.wantCursor,
					tc.wantChanged,
				)
			}
		})
	}
}
