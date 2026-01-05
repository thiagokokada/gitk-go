package gui

import "testing"

func TestPaletteForThemeChange(t *testing.T) {
	pal, changed := paletteForThemeChange(ThemeAuto, lightPalette, true)
	if !changed || pal != darkPalette {
		t.Fatalf("expected auto preference to switch to dark palette, got %+v changed=%t", pal, changed)
	}

	pal, changed = paletteForThemeChange(ThemeAuto, darkPalette, true)
	if changed || pal != darkPalette {
		t.Fatalf("expected no change when already dark, got %+v changed=%t", pal, changed)
	}

	pal, changed = paletteForThemeChange(ThemeAuto, darkPalette, false)
	if !changed || pal != lightPalette {
		t.Fatalf("expected auto preference to switch to light palette, got %+v changed=%t", pal, changed)
	}

	pal, changed = paletteForThemeChange(ThemeLight, darkPalette, true)
	if changed || pal != darkPalette {
		t.Fatalf("expected explicit preference to ignore updates, got %+v changed=%t", pal, changed)
	}
}
