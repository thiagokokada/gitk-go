package gui

import (
	"testing"
	"time"
)

func TestScrollRestoreTarget(t *testing.T) {
	tests := []struct {
		name      string
		prevStart float64
		prevTotal int
		newTotal  int
		want      float64
		wantOK    bool
	}{
		{name: "invalid prev start", prevStart: -1, prevTotal: 10, newTotal: 10, wantOK: false},
		{name: "invalid totals", prevStart: 0.5, prevTotal: 0, newTotal: 10, wantOK: false},
		{name: "invalid new total", prevStart: 0.5, prevTotal: 10, newTotal: 0, wantOK: false},
		{name: "same totals keeps start", prevStart: 0.25, prevTotal: 100, newTotal: 100, want: 0.25, wantOK: true},
		{name: "growing list scales down", prevStart: 0.5, prevTotal: 100, newTotal: 200, want: 0.25, wantOK: true},
		{name: "shrinking list scales up", prevStart: 0.25, prevTotal: 200, newTotal: 100, want: 0.5, wantOK: true},
		{name: "clamps high", prevStart: 10, prevTotal: 100, newTotal: 1, want: 1, wantOK: true},
		{name: "clamps low", prevStart: -0.1, prevTotal: 100, newTotal: 1, wantOK: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state := scrollState{start: tc.prevStart, total: tc.prevTotal}
			got, ok := state.restoreTarget(tc.newTotal)
			if ok != tc.wantOK {
				t.Fatalf("want ok=%v, got %v (target=%f)", tc.wantOK, ok, got)
			}
			if !ok {
				return
			}
			if got != tc.want {
				t.Fatalf("want %f, got %f", tc.want, got)
			}
		})
	}
}

func TestApplyFilterDoesNotStopDebounce(t *testing.T) {
	a := &Controller{}
	a.state.filter.debounce.Configure(time.Hour, func(string) {})
	a.state.filter.debounce.Trigger("stale")

	a.applyFilterState("foo")

	if !a.state.filter.debounce.Active() {
		t.Fatalf("expected debouncer to remain set")
	}
	if got := a.state.filter.value; got != "foo" {
		t.Fatalf("expected filter value %q, got %q", "foo", got)
	}
}

func TestScheduleFilterApplyEmptyStopsDebounce(t *testing.T) {
	a := &Controller{}
	a.state.filter.debounce.Configure(time.Hour, func(string) {})
	a.state.filter.debounce.Trigger("foo")
	a.state.filter.debounce.SetPending("foo")
	a.state.filter.value = "foo"

	a.scheduleFilterApplyState("")

	if a.state.filter.debounce.Active() {
		t.Fatalf("expected debouncer to be stopped")
	}
	if a.state.filter.debounce.HasPending() {
		t.Fatalf("expected pending filter to be cleared")
	}
	if got := a.state.filter.value; got != "" {
		t.Fatalf("expected filter value cleared, got %q", got)
	}
}

func TestShouldAutoLoadForFilter(t *testing.T) {
	tests := []struct {
		name         string
		filterValue  string
		visibleLen   int
		hasMore      bool
		loadingBatch bool
		want         bool
	}{
		{name: "empty filter", filterValue: "", visibleLen: 0, hasMore: true, want: false},
		{name: "blank filter", filterValue: "   ", visibleLen: 0, hasMore: true, want: false},
		{name: "has visible matches", filterValue: "feat", visibleLen: 1, hasMore: true, want: false},
		{name: "no more commits", filterValue: "feat", visibleLen: 0, hasMore: false, want: false},
		{name: "loading already", filterValue: "feat", visibleLen: 0, hasMore: true, loadingBatch: true, want: false},
		{name: "needs load", filterValue: "feat", visibleLen: 0, hasMore: true, want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldAutoLoadForFilter(tc.filterValue, tc.visibleLen, tc.hasMore, tc.loadingBatch)
			if got != tc.want {
				t.Fatalf("expected auto load=%v, got %v", tc.want, got)
			}
		})
	}
}

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
