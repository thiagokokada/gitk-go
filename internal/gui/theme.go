package gui

import (
	"log/slog"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"

	darkmode "github.com/thiagokokada/dark-mode-go"
)

type ThemePreference int

const (
	ThemeAuto ThemePreference = iota
	ThemeLight
	ThemeDark
)

func (p ThemePreference) String() string {
	return []string{"auto", "light", "dark"}[p]
}

type colorPalette struct {
	ThemeName        string
	HighlightTheme   string
	DiffAdd          string
	DiffDel          string
	DiffHeader       string
	LocalUnstagedRow string
	LocalStagedRow   string
}

var (
	lightPalette = colorPalette{
		ThemeName:        "azure light",
		HighlightTheme:   "github",
		DiffAdd:          "#dff5de",
		DiffDel:          "#f9d6d5",
		DiffHeader:       "#e4e4e4",
		LocalUnstagedRow: "#fde2e1",
		LocalStagedRow:   "#e2f7e1",
	}
	darkPalette = colorPalette{
		ThemeName:        "azure dark",
		HighlightTheme:   "github-dark",
		DiffAdd:          "#1c6135",
		DiffDel:          "#612238",
		DiffHeader:       "#3a3a3a",
		LocalUnstagedRow: "#4a1f23",
		LocalStagedRow:   "#1f3b2a",
	}
	detectDarkMode = darkmode.IsDarkMode
	watchDarkMode  = darkmode.WatchDarkMode
)

func ThemePreferenceFromString(raw string) ThemePreference {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case ThemeDark.String():
		return ThemeDark
	case ThemeLight.String():
		return ThemeLight
	default:
		return ThemeAuto
	}
}

func paletteForDarkMode(dark bool) colorPalette {
	if dark {
		return darkPalette
	}
	return lightPalette
}

func paletteForThemeChange(pref ThemePreference, current colorPalette, dark bool) (colorPalette, bool) {
	if pref != ThemeAuto {
		return current, false
	}
	next := paletteForDarkMode(dark)
	if next == current {
		return current, false
	}
	return next, true
}

func paletteForPreference(pref ThemePreference) colorPalette {
	switch pref {
	case ThemeDark:
		return darkPalette
	case ThemeLight:
		return lightPalette
	default:
		if detectDarkMode != nil {
			if dark, err := detectDarkMode(); err == nil {
				return paletteForDarkMode(dark)
			} else {
				slog.Error("detect dark-mode", slog.Any("error", err))
			}
		}
		return darkPalette
	}
}

func (p colorPalette) isDark() bool {
	return p == darkPalette
}

func (p colorPalette) chromaStyle() *chroma.Style {
	if p.HighlightTheme != "" {
		if st := styles.Get(p.HighlightTheme); st != nil {
			return st
		}
	}
	return styles.Fallback
}
