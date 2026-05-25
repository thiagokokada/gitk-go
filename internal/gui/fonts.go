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

func (a *Controller) showUIFontDialog() {
	showFontDialog("Select UI Font", DefaultFont, a.prefs.uiFontSpec, a.applyUIFontSpec)
}

func (a *Controller) showFixedFontDialog() {
	showFontDialog("Select Fixed Font", FixedFont, a.prefs.fixedFontSpec, a.applyFixedFontSpec)
}

func showFontDialog(title string, seedFont string, seedSpec []string, apply func([]string, bool) bool) {
	Fontchooser(
		Parent(App),
		Title(title),
		fontChooserSeed(seedFont, seedSpec),
		Command(func() {
			if !apply(FontchooserFont(), true) {
				slog.Debug("font selection missing or invalid", slog.String("title", title))
			}
		}),
	)
	FontchooserShow()
}

func (a *Controller) applyUIFontSpec(spec []string, save bool) bool {
	selection, ok := fontSelectionFromSpec(spec)
	if !ok {
		return false
	}
	a.prefs.uiFontSpec = selection.preferenceSpec()
	selection.applyToNamedFonts(uiNamedFonts)
	applyUIFontToStyles()
	a.applyUIFontToWidgets()
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
	selection.applyToNamedFonts([]string{FixedFont})
	a.applyFixedFontToDiff()
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

func (a *Controller) applyFixedFontToDiff() {
	if a.ui.diffDetail == nil {
		return
	}
	a.ui.diffDetail.Configure(Font(FixedFont))
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

func (selection fontSelection) applyToNamedFonts(names []string) {
	options := selection.fontOptions()
	for _, name := range names {
		FontConfigure(name, options...)
	}
}

func applyUIFontToStyles() {
	styles := []string{
		".",
		"TLabel",
		"TEntry",
		"TButton",
		"Treeview",
		"Treeview.Heading",
	}
	for _, style := range styles {
		StyleConfigure(style, Font(DefaultFont))
	}
}

func (a *Controller) applyUIFontToWidgets() {
	if a.ui.diffFileList != nil {
		a.ui.diffFileList.Configure(Font(DefaultFont))
	}
	if a.ui.treeContextMenu != nil {
		a.ui.treeContextMenu.Configure(Font(DefaultFont))
	}
	if a.ui.diffContextMenu != nil {
		a.ui.diffContextMenu.Configure(Font(DefaultFont))
	}
	if a.ui.menubar != nil {
		a.ui.menubar.Configure(Font(DefaultFont))
	}
	if a.ui.fileMenu != nil {
		a.ui.fileMenu.Configure(Font(DefaultFont))
	}
	if a.ui.viewMenu != nil {
		a.ui.viewMenu.Configure(Font(DefaultFont))
	}
	if a.ui.helpMenu != nil {
		a.ui.helpMenu.Configure(Font(DefaultFont))
	}
}
