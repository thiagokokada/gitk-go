package gui

import (
	"testing"

	. "modernc.org/tk9.0"
)

func TestFontSelectionFromSpec(t *testing.T) {
	selection, ok := fontSelectionFromSpec([]string{
		"DejaVu Sans",
		"11",
		"bold",
		"italic",
		"underline",
		"overstrike",
	})
	if !ok {
		t.Fatal("expected selection to be valid")
	}
	if selection.Family != "DejaVu Sans" {
		t.Fatalf("family = %q, want %q", selection.Family, "DejaVu Sans")
	}
	if selection.Size != 11 {
		t.Fatalf("size = %d, want %d", selection.Size, 11)
	}
	if selection.Weight != BOLD {
		t.Fatalf("weight = %q, want %q", selection.Weight, BOLD)
	}
	if selection.Slant != ITALIC {
		t.Fatalf("slant = %q, want %q", selection.Slant, ITALIC)
	}
	if !selection.Underline {
		t.Fatal("underline = false, want true")
	}
	if !selection.Overstrike {
		t.Fatal("overstrike = false, want true")
	}
}

func TestFontSelectionFromSpecDefaults(t *testing.T) {
	selection, ok := fontSelectionFromSpec([]string{"DejaVu Sans Mono", "10"})
	if !ok {
		t.Fatal("expected selection to be valid")
	}
	if selection.Weight != NORMAL {
		t.Fatalf("weight = %q, want %q", selection.Weight, NORMAL)
	}
	if selection.Slant != ROMAN {
		t.Fatalf("slant = %q, want %q", selection.Slant, ROMAN)
	}
	if selection.Underline {
		t.Fatal("underline = true, want false")
	}
	if selection.Overstrike {
		t.Fatal("overstrike = true, want false")
	}
}

func TestFontSelectionPreferenceSpec(t *testing.T) {
	selection, ok := fontSelectionFromSpec([]string{
		"DejaVu Sans",
		"11",
		"normal",
		"roman",
		"bold",
		"italic",
		"underline",
	})
	if !ok {
		t.Fatal("expected selection to be valid")
	}
	got := selection.preferenceSpec()
	want := []string{"DejaVu Sans", "11", "bold", "italic", "underline"}
	if len(got) != len(want) {
		t.Fatalf("preferenceSpec len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("preferenceSpec[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestFontSelectionFromSpecInvalid(t *testing.T) {
	if _, ok := fontSelectionFromSpec(nil); ok {
		t.Fatal("expected nil spec to be invalid")
	}
	if _, ok := fontSelectionFromSpec([]string{"Sans"}); ok {
		t.Fatal("expected spec missing size to be invalid")
	}
	if _, ok := fontSelectionFromSpec([]string{"", "11"}); ok {
		t.Fatal("expected empty family to be invalid")
	}
	if _, ok := fontSelectionFromSpec([]string{"Sans", ""}); ok {
		t.Fatal("expected empty size to be invalid")
	}
	if _, ok := fontSelectionFromSpec([]string{"Sans", "large"}); ok {
		t.Fatal("expected non-numeric size to be invalid")
	}
	if _, ok := fontSelectionFromSpec([]string{"Sans", "0"}); ok {
		t.Fatal("expected zero size to be invalid")
	}
}
