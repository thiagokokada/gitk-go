package tkutil

import (
	"errors"
	"testing"
)

func TestTclSafeString(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: "{}"},
		{name: "plain", in: "abc", want: "abc"},
		{name: "space", in: "a b", want: "a\\x20b"},
		{name: "brackets", in: "[x]", want: "\\x5bx\\x5d"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TclSafeString(tt.in); got != tt.want {
				t.Fatalf("TclSafeString(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestTclSafeStrings(t *testing.T) {
	if got := TclSafeStrings(); got != "" {
		t.Fatalf("TclSafeStrings() = %q, want empty", got)
	}
	if got := TclSafeStrings("a", "b c"); got != "a b\\x20c" {
		t.Fatalf("TclSafeStrings(...) = %q, want %q", got, "a b\\x20c")
	}
}

func TestWidgetExists(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		out        string
		err        error
		want       bool
		wantCalled bool
	}{
		{name: "empty path", path: "", want: false, wantCalled: false},
		{name: "blank path", path: " ", want: false, wantCalled: false},
		{name: "exists", path: ".tree", out: "1", want: true, wantCalled: true},
		{name: "missing", path: ".tree", out: "0", want: false, wantCalled: true},
		{name: "trimmed output", path: ".tree", out: " 1 ", want: true, wantCalled: true},
		{name: "eval error", path: ".tree", err: errors.New("boom"), want: false, wantCalled: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			called := 0
			eval := func(format string, a ...any) (string, error) {
				called++
				return tc.out, tc.err
			}
			got := widgetExistsWithEval(eval, tc.path)
			if got != tc.want {
				t.Fatalf("widgetExistsWithEval(%q) = %v, want %v", tc.path, got, tc.want)
			}
			if tc.wantCalled && called != 1 {
				t.Fatalf("expected eval to be called once, got %d", called)
			}
			if !tc.wantCalled && called != 0 {
				t.Fatalf("expected eval not to be called, got %d", called)
			}
		})
	}
}
