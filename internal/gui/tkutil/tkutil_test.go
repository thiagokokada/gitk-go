package tkutil

import "testing"

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
