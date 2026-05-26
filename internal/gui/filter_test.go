package gui

import (
	"testing"
	"time"

	"github.com/thiagokokada/gitk-go/internal/gui/model"
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
			state := model.ScrollState{Start: tc.prevStart, Total: tc.prevTotal}
			got, ok := state.RestoreTarget(tc.newTotal)
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
	a.runtime.actions.filterDebounce.Configure(time.Hour, func(string) {})
	a.runtime.actions.filterDebounce.Trigger("stale")

	a.applyFilterState("foo")

	if !a.runtime.actions.filterDebounce.Active() {
		t.Fatalf("expected debouncer to remain set")
	}
	if got := a.model.State.Filter.Value; got != "foo" {
		t.Fatalf("expected filter value %q, got %q", "foo", got)
	}
}

func TestScheduleFilterApplyEmptyStopsDebounce(t *testing.T) {
	a := &Controller{}
	a.runtime.actions.filterDebounce.Configure(time.Hour, func(string) {})
	a.runtime.actions.filterDebounce.Trigger("foo")
	a.runtime.actions.filterDebounce.SetPending("foo")
	a.model.State.Filter.Value = "foo"

	a.scheduleFilterApplyState("")

	if a.runtime.actions.filterDebounce.Active() {
		t.Fatalf("expected debouncer to be stopped")
	}
	if a.runtime.actions.filterDebounce.HasPending() {
		t.Fatalf("expected pending filter to be cleared")
	}
	if got := a.model.State.Filter.Value; got != "" {
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
