package gui

import (
	"log/slog"
	"runtime"
	"strconv"
	"strings"

	. "modernc.org/tk9.0"
)

var uiNamedFonts = []string{
	DefaultFont,
	TextFont,
	MenuFont,
	HeadingFont,
	CaptionFont,
	SmallCaptionFont,
	IconFont,
	TooltipFont,
}

type fontSelection struct {
	Family     string
	Size       int
	Weight     string
	Slant      string
	Underline  bool
	Overstrike bool
}

func diffDetailFontSpec() []any {
	if runtime.GOOS == "linux" {
		return []any{FixedFont}
	}
	return []any{CourierFont(), 11}
}

func (a *Controller) applyUIFontSpec(spec []string, save bool) bool {
	selection, ok := fontSelectionFromSpec(spec)
	if !ok {
		return false
	}
	a.prefs.uiFontSpec = selection.preferenceSpec()
	a.ui.ApplyNamedFontOptions(uiNamedFonts, selection.fontOptions())
	a.ui.ApplyUIFontToStyles()
	a.ui.ApplyUIFontToWidgets()
	a.scheduleGraphCanvasDraw()
	if save {
		a.savePreferences(false)
	}
	return true
}

func (a *Controller) applyFixedFontSpec(spec []string, save bool) bool {
	selection, ok := fontSelectionFromSpec(spec)
	if !ok {
		return false
	}
	a.prefs.fixedFontSpec = selection.preferenceSpec()
	a.ui.ApplyNamedFontOptions([]string{FixedFont}, selection.fontOptions())
	a.ui.ApplyFixedFontToDiff()
	if save {
		a.savePreferences(false)
	}
	return true
}

func (a *Controller) applyStoredFontPreferences() {
	if len(a.prefs.uiFontSpec) > 0 {
		if !a.applyUIFontSpec(a.prefs.uiFontSpec, false) {
			slog.Debug("stored ui font invalid")
		}
	}
	if len(a.prefs.fixedFontSpec) > 0 {
		if !a.applyFixedFontSpec(a.prefs.fixedFontSpec, false) {
			slog.Debug("stored fixed font invalid")
		}
	}
}

func fontSelectionFromSpec(spec []string) (fontSelection, bool) {
	var selection fontSelection
	if len(spec) < 2 {
		return selection, false
	}
	family := strings.TrimSpace(spec[0])
	size := strings.TrimSpace(spec[1])
	if family == "" || size == "" {
		return selection, false
	}
	sizeValue, err := strconv.Atoi(size)
	if err != nil || sizeValue == 0 {
		return selection, false
	}
	selection = fontSelection{
		Family: family,
		Size:   sizeValue,
		Weight: NORMAL,
		Slant:  ROMAN,
	}
	for _, token := range spec[2:] {
		token = strings.ToLower(strings.TrimSpace(token))
		switch token {
		case BOLD:
			selection.Weight = BOLD
		case NORMAL:
			selection.Weight = NORMAL
		case ITALIC:
			selection.Slant = ITALIC
		case ROMAN:
			selection.Slant = ROMAN
		case UNDERLINE:
			selection.Underline = true
		case OVERSTRIKE:
			selection.Overstrike = true
		default:
			slog.Debug("unknown token", slog.String("token", token))
		}
	}
	return selection, true
}

func fontChooserSeed(seedFont string, seedSpec []string) Opt {
	selection, ok := fontSelectionFromSpec(seedSpec)
	if !ok {
		return Font(seedFont)
	}
	return selection.fontOpt()
}

func (selection fontSelection) fontOpt() Opt {
	args := []any{selection.Family, selection.Size}
	if selection.Weight == BOLD {
		args = append(args, BOLD)
	}
	if selection.Slant == ITALIC {
		args = append(args, ITALIC)
	}
	if selection.Underline {
		args = append(args, UNDERLINE)
	}
	if selection.Overstrike {
		args = append(args, OVERSTRIKE)
	}
	return Font(args...)
}

func (selection fontSelection) fontOptions() []any {
	return []any{
		Family(selection.Family),
		Size(selection.Size),
		Weight(selection.Weight),
		Slant(selection.Slant),
		Underline(selection.Underline),
		Overstrike(selection.Overstrike),
	}
}

func (selection fontSelection) preferenceSpec() []string {
	spec := []string{selection.Family, strconv.Itoa(selection.Size)}
	if selection.Weight == BOLD {
		spec = append(spec, BOLD)
	}
	if selection.Slant == ITALIC {
		spec = append(spec, ITALIC)
	}
	if selection.Underline {
		spec = append(spec, UNDERLINE)
	}
	if selection.Overstrike {
		spec = append(spec, OVERSTRIKE)
	}
	return spec
}
