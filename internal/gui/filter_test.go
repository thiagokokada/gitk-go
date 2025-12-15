package gui

import "testing"

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
