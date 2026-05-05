package gui

import "testing"

func TestTextIndexLineNumber(t *testing.T) {
	tests := []struct {
		index  string
		want   int
		wantOK bool
	}{
		{index: "1.0", want: 1, wantOK: true},
		{index: "27.end", want: 27, wantOK: true},
		{index: "@0,0", want: 0, wantOK: false},
		{index: "", want: 0, wantOK: false},
		{index: "bad.value", want: 0, wantOK: false},
	}

	for _, tc := range tests {
		got, ok := textIndexLineNumber(tc.index)
		if got != tc.want || ok != tc.wantOK {
			t.Fatalf("index=%q: want (%d,%v), got (%d,%v)", tc.index, tc.want, tc.wantOK, got, ok)
		}
	}
}
