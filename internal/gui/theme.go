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

type graphLabelPalette struct {
	Fill    string
	Outline string
	Text    string
}

type graphCanvasPalette struct {
	LaneColors      [7]string
	SelectedRowFill string
	NodeFill        string
	HeadNodeFill    string
	HeadLabel       graphLabelPalette
	TagLabel        graphLabelPalette
	BranchLabel     graphLabelPalette
	DefaultLabel    graphLabelPalette
}

type colorPalette struct {
	ThemeName        string
	HighlightTheme   string
	DiffAdd          string
	DiffAddText      string
	DiffDel          string
	DiffDelText      string
	DiffHeader       string
	LocalUnstagedRow string
	LocalStagedRow   string
	GraphCanvas      graphCanvasPalette
}

var (
	lightPalette = colorPalette{
		ThemeName:        "azure light",
		HighlightTheme:   "github",
		DiffAdd:          "#dff5de",
		DiffAddText:      "#1f7a1f",
		DiffDel:          "#f9d6d5",
		DiffDelText:      "#b42318",
		DiffHeader:       "#e4e4e4",
		LocalUnstagedRow: "#fde2e1",
		LocalStagedRow:   "#e2f7e1",
		GraphCanvas: graphCanvasPalette{
			// Based on gitk's default colors; keep a small, high-contrast palette.
			LaneColors:      [7]string{"#00cc00", "#cc0000", "#0055cc", "#aa00aa", "#555555", "#8b4513", "#ff8c00"},
			SelectedRowFill: "#cfe7ff",
			NodeFill:        "white",
			HeadNodeFill:    "#ffd75e",
			HeadLabel:       graphLabelPalette{Fill: "#ffd75e", Outline: "#c9a300", Text: "#111111"},
			TagLabel:        graphLabelPalette{Fill: "#e6e6e6", Outline: "#8a8a8a", Text: "#111111"},
			BranchLabel:     graphLabelPalette{Fill: "#dbeafe", Outline: "#2563eb", Text: "#111111"},
			DefaultLabel:    graphLabelPalette{Fill: "#dff5de", Text: "#111111"},
		},
	}
	darkPalette = colorPalette{
		ThemeName:        "azure dark",
		HighlightTheme:   "github-dark",
		DiffAdd:          "#1c6135",
		DiffAddText:      "#6ddf6d",
		DiffDel:          "#612238",
		DiffDelText:      "#ff7b72",
		DiffHeader:       "#3a3a3a",
		LocalUnstagedRow: "#4a1f23",
		LocalStagedRow:   "#1f3b2a",
		GraphCanvas: graphCanvasPalette{
			// Based on gitk's default colors; keep a small, high-contrast palette.
			LaneColors:      [7]string{"#00ff00", "#ff5c5c", "#4fa3ff", "#d56bff", "#a0a0a0", "#d09a6b", "#ffb347"},
			SelectedRowFill: "#253446",
			NodeFill:        "#1e1e1e",
			HeadNodeFill:    "#b58900",
			HeadLabel:       graphLabelPalette{Fill: "#b58900", Outline: "#8a6a00", Text: "#111111"},
			TagLabel:        graphLabelPalette{Fill: "#3a3a3a", Outline: "#6b6b6b", Text: "#eaeaea"},
			BranchLabel:     graphLabelPalette{Fill: "#253446", Outline: "#4fa3ff", Text: "#eaeaea"},
			DefaultLabel:    graphLabelPalette{Fill: "#1f3b2a", Text: "#eaeaea"},
		},
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

func (p colorPalette) chromaStyle() *chroma.Style {
	if p.HighlightTheme != "" {
		if st := styles.Get(p.HighlightTheme); st != nil {
			return st
		}
	}
	return styles.Fallback
}
