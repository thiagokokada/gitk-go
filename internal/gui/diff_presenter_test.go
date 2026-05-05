package gui

import "testing"

func TestDetailStatusText(t *testing.T) {
	tests := []struct {
		name   string
		header string
		body   string
		want   string
	}{
		{name: "header and body", header: "Header", body: "Body", want: "Header\nBody"},
		{name: "trim newline boundaries", header: "Header\n", body: "\nBody", want: "Header\nBody"},
		{name: "header only", header: "Header", body: "", want: "Header"},
		{name: "body only", header: "", body: "Body", want: "Body"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := detailStatusText(tc.header, tc.body); got != tc.want {
				t.Fatalf("want %q, got %q", tc.want, got)
			}
		})
	}
}
