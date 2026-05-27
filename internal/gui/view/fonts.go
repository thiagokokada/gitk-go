package view

import (
	"log/slog"
	"strconv"
	"strings"

	tk "modernc.org/tk9.0"
)

var UINamedFonts = []string{
	tk.DefaultFont,
	tk.TextFont,
	tk.MenuFont,
	tk.HeadingFont,
	tk.CaptionFont,
	tk.SmallCaptionFont,
	tk.IconFont,
	tk.TooltipFont,
}

type FontSelection struct {
	Family     string
	Size       int
	Weight     string
	Slant      string
	Underline  bool
	Overstrike bool
}

func ShowFontDialog(title string, seed tk.Opt, apply func([]string, bool) bool) {
	tk.Fontchooser(
		tk.Parent(tk.App),
		tk.Title(title),
		seed,
		tk.Command(func() {
			if apply == nil || !apply(tk.FontchooserFont(), true) {
				slog.Debug("font selection missing or invalid", slog.String("title", title))
			}
		}),
	)
	tk.FontchooserShow()
}

func FontSelectionFromSpec(spec []string) (FontSelection, bool) {
	var selection FontSelection
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
	selection = FontSelection{
		Family: family,
		Size:   sizeValue,
		Weight: tk.NORMAL,
		Slant:  tk.ROMAN,
	}
	for _, token := range spec[2:] {
		token = strings.ToLower(strings.TrimSpace(token))
		switch token {
		case tk.BOLD:
			selection.Weight = tk.BOLD
		case tk.NORMAL:
			selection.Weight = tk.NORMAL
		case tk.ITALIC:
			selection.Slant = tk.ITALIC
		case tk.ROMAN:
			selection.Slant = tk.ROMAN
		case tk.UNDERLINE:
			selection.Underline = true
		case tk.OVERSTRIKE:
			selection.Overstrike = true
		default:
			slog.Debug("unknown token", slog.String("token", token))
		}
	}
	return selection, true
}

func FontChooserSeed(seedFont string, seedSpec []string) tk.Opt {
	selection, ok := FontSelectionFromSpec(seedSpec)
	if !ok {
		return tk.Font(seedFont)
	}
	return selection.FontOpt()
}

func (selection FontSelection) FontOpt() tk.Opt {
	args := []any{selection.Family, selection.Size}
	if selection.Weight == tk.BOLD {
		args = append(args, tk.BOLD)
	}
	if selection.Slant == tk.ITALIC {
		args = append(args, tk.ITALIC)
	}
	if selection.Underline {
		args = append(args, tk.UNDERLINE)
	}
	if selection.Overstrike {
		args = append(args, tk.OVERSTRIKE)
	}
	return tk.Font(args...)
}

func (selection FontSelection) FontOptions() []any {
	return []any{
		tk.Family(selection.Family),
		tk.Size(selection.Size),
		tk.Weight(selection.Weight),
		tk.Slant(selection.Slant),
		tk.Underline(selection.Underline),
		tk.Overstrike(selection.Overstrike),
	}
}

func (selection FontSelection) PreferenceSpec() []string {
	spec := []string{selection.Family, strconv.Itoa(selection.Size)}
	if selection.Weight == tk.BOLD {
		spec = append(spec, tk.BOLD)
	}
	if selection.Slant == tk.ITALIC {
		spec = append(spec, tk.ITALIC)
	}
	if selection.Underline {
		spec = append(spec, tk.UNDERLINE)
	}
	if selection.Overstrike {
		spec = append(spec, tk.OVERSTRIKE)
	}
	return spec
}

func ApplyNamedFontOptions(names []string, options []any) {
	for _, name := range names {
		tk.FontConfigure(name, options...)
	}
}

func ApplyUIFontToStyles() {
	styles := []string{
		".",
		"TLabel",
		"TEntry",
		"TButton",
		"Treeview",
		"Treeview.Heading",
	}
	for _, style := range styles {
		tk.StyleConfigure(style, tk.Font(tk.DefaultFont))
	}
}

func (a *App) ApplyUIFontToWidgets() {
	if a.DiffFileList != nil {
		a.DiffFileList.Configure(tk.Font(tk.DefaultFont))
	}
	if a.TreeContextMenu != nil {
		a.TreeContextMenu.Configure(tk.Font(tk.DefaultFont))
	}
	if a.DiffContextMenu != nil {
		a.DiffContextMenu.Configure(tk.Font(tk.DefaultFont))
	}
	if a.Menubar != nil {
		a.Menubar.Configure(tk.Font(tk.DefaultFont))
	}
	if a.FileMenu != nil {
		a.FileMenu.Configure(tk.Font(tk.DefaultFont))
	}
	if a.ViewMenu != nil {
		a.ViewMenu.Configure(tk.Font(tk.DefaultFont))
	}
	if a.HelpMenu != nil {
		a.HelpMenu.Configure(tk.Font(tk.DefaultFont))
	}
}

func (a *App) ApplyFixedFontToDiff() {
	if a.DiffDetail == nil {
		return
	}
	a.DiffDetail.Configure(tk.Font(tk.FixedFont))
}
