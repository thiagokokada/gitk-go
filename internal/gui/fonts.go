package gui

import (
	"log/slog"
	"runtime"

	"github.com/thiagokokada/gitk-go/internal/gui/view"

	. "modernc.org/tk9.0"
)

func diffDetailFontSpec() []any {
	if runtime.GOOS == "linux" {
		return []any{FixedFont}
	}
	return []any{CourierFont(), 11}
}

func (a *Controller) applyUIFontSpec(spec []string, save bool) bool {
	selection, ok := view.FontSelectionFromSpec(spec)
	if !ok {
		return false
	}
	a.prefs.uiFontSpec = selection.PreferenceSpec()
	view.ApplyNamedFontOptions(view.UINamedFonts, selection.FontOptions())
	view.ApplyUIFontToStyles()
	a.ui.ApplyUIFontToWidgets()
	a.scheduleGraphCanvasDraw()
	if save {
		a.savePreferences(false)
	}
	return true
}

func (a *Controller) applyFixedFontSpec(spec []string, save bool) bool {
	selection, ok := view.FontSelectionFromSpec(spec)
	if !ok {
		return false
	}
	a.prefs.fixedFontSpec = selection.PreferenceSpec()
	view.ApplyNamedFontOptions([]string{FixedFont}, selection.FontOptions())
	a.ui.ApplyFixedFontToDiff()
	if save {
		a.savePreferences(false)
	}
	return true
}

func (a *Controller) applyStoredFontPreferences() {
	if len(a.prefs.uiFontSpec) > 0 {
		if !a.applyUIFontSpec(a.prefs.uiFontSpec, false) {
			slog.Error("stored ui font invalid")
		}
	}
	if len(a.prefs.fixedFontSpec) > 0 {
		if !a.applyFixedFontSpec(a.prefs.fixedFontSpec, false) {
			slog.Error("stored fixed font invalid")
		}
	}
}
