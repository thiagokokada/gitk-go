package gui

import (
	"log"
	"strings"

	darkmode "github.com/thiagokokada/dark-mode-go"
)

type ThemePreference int

const (
	ThemeAuto ThemePreference = iota
	ThemeLight
	ThemeDark
)

func (p ThemePreference) String() string {
	switch p {
	case ThemeLight:
		return "light"
	case ThemeDark:
		return "dark"
	default:
		return "auto"
	}
}

type colorPalette struct {
	ThemeName        string
	DiffAdd          string
	DiffDel          string
	DiffHeader       string
	LocalUnstagedRow string
	LocalStagedRow   string
}

var (
	lightPalette = colorPalette{
		ThemeName:        "azure light",
		DiffAdd:          "#dff5de",
		DiffDel:          "#f9d6d5",
		DiffHeader:       "#e4e4e4",
		LocalUnstagedRow: "#fde2e1",
		LocalStagedRow:   "#e2f7e1",
	}
	darkPalette = colorPalette{
		ThemeName:        "azure dark",
		DiffAdd:          "#1f3d2b",
		DiffDel:          "#3d1f29",
		DiffHeader:       "#2f2f2f",
		LocalUnstagedRow: "#4a1f23",
		LocalStagedRow:   "#1f3b2a",
	}
	detectDarkMode = darkmode.IsDarkMode
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

func paletteForPreference(pref ThemePreference) colorPalette {
	switch pref {
	case ThemeDark:
		return darkPalette
	case ThemeLight:
		return lightPalette
	default:
		if detectDarkMode != nil {
			if dark, err := detectDarkMode(); err == nil {
				if dark {
					return darkPalette
				}
			} else {
				log.Printf("detect dark-mode: %v", err)
			}
		}
		return lightPalette
	}
}

func (p colorPalette) isDark() bool {
	return strings.Contains(strings.ToLower(p.ThemeName), "dark")
}
