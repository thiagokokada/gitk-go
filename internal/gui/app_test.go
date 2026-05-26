package gui

import (
	"strings"
	"testing"

	"github.com/thiagokokada/gitk-go/internal/git"
	"github.com/thiagokokada/gitk-go/internal/gui/model"
)

func TestStatusSummary(t *testing.T) {
	ctrl := &Controller{
		model: model.App{
			Repo: model.Repository{
				Path:    "/repo/path",
				HeadRef: "main",
			},
			Data: model.Data{
				Commits: []*git.Entry{{}, {}},
				Visible: []*git.Entry{{}},
			},
			State: model.State{
				Tree: model.TreeState{
					HasMore: true,
				},
				Filter: model.FilterState{
					Value: "feature",
				},
			},
		},
	}
	summary := ctrl.statusSummary()
	if !strings.Contains(summary, "Showing 1/2") {
		t.Fatalf("unexpected summary counts: %s", summary)
	}
	if !strings.Contains(summary, "filter") && !strings.Contains(summary, "Filter") {
		t.Fatalf("expected filter mention in summary: %s", summary)
	}
	if !strings.Contains(summary, "/repo/path") {
		t.Fatalf("expected repo path in summary: %s", summary)
	}
}

func TestThemePreferenceFromString(t *testing.T) {
	if got := ThemePreferenceFromString("Dark"); got != ThemeDark {
		t.Fatalf("expected ThemeDark, got %v", got)
	}
	if got := ThemePreferenceFromString("light"); got != ThemeLight {
		t.Fatalf("expected ThemeLight, got %v", got)
	}
	if got := ThemePreferenceFromString("other"); got != ThemeDark {
		t.Fatalf("expected ThemeDark fallback, got %v", got)
	}
}

func TestPaletteForPreference(t *testing.T) {
	if pal := paletteForPreference(ThemeLight); pal.ThemeName != lightPalette.ThemeName {
		t.Fatalf("explicit light preference should use light palette, got %+v", pal)
	}
	if pal := paletteForPreference(ThemeDark); pal.ThemeName != darkPalette.ThemeName {
		t.Fatalf("explicit dark preference should use dark palette, got %+v", pal)
	}
}
