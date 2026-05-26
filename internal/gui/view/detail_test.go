package view

import "testing"

func TestStripDiffLineMarkers(t *testing.T) {
	t.Parallel()

	input := "+added\n-removed\n context\n++keeps second marker\n--keeps second marker\n"
	want := "added\nremoved\n context\n+keeps second marker\n-keeps second marker\n"
	if got := StripDiffLineMarkers(input); got != want {
		t.Fatalf("StripDiffLineMarkers() = %q, want %q", got, want)
	}
}
