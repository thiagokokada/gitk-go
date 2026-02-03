package gui

import (
	"log/slog"
	"runtime"
	"strings"

	"github.com/thiagokokada/gitk-go/internal/gui/tkutil"

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

type fontDialogTarget struct {
	title      string
	seedFont   string
	names      []string
	afterApply func(selection fontSelection)
}

type fontSelection struct {
	Family     string
	Size       string
	Weight     string
	Slant      string
	Underline  bool
	Overstrike bool
}

type controllerFonts struct {
	ui *FontFace
}

func diffDetailFontSpec() []any {
	if runtime.GOOS == "linux" {
		return []any{FixedFont}
	}
	return []any{CourierFont(), 11}
}

func (a *Controller) showUIFontDialog() {
	a.showFontDialog(fontDialogTarget{
		title:      "Select UI Font",
		seedFont:   DefaultFont,
		names:      uiNamedFonts,
		afterApply: a.applyUIFontSelection,
	})
}

func (a *Controller) showFixedFontDialog() {
	a.showFontDialog(fontDialogTarget{
		title:    "Select Fixed Font",
		seedFont: FixedFont,
		names:    []string{FixedFont},
		afterApply: func(selection fontSelection) {
			a.applyFixedFontToDiff()
		},
	})
}

func (a *Controller) showFontDialog(target fontDialogTarget) {
	Fontchooser(
		Parent(App),
		Title(target.title),
		Font(target.seedFont),
		Command(func() {
			a.applyFontDialogSelection(target)
		}),
	)
	FontchooserShow()
}

func (*Controller) applyFontDialogSelection(target fontDialogTarget) {
	selection, ok := fontSelectionFromSpec(FontchooserFont())
	if !ok {
		slog.Debug("font selection missing or invalid")
		return
	}
	applyFontSelectionToNamedFonts(selection, target.names)
	if target.afterApply != nil {
		target.afterApply(selection)
	}
}

func (a *Controller) applyFixedFontToDiff() {
	if a.ui.diffDetail == nil {
		return
	}
	a.ui.diffDetail.Configure(Font(FixedFont))
}

func (a *Controller) applyUIFontSelection(selection fontSelection) {
	font := newFontFaceFromSelection(selection)
	if font == nil {
		return
	}
	a.replaceFontFace(&a.fonts.ui, font)
	applyUIFontToStyles(font)
	applyUIFontToOptions(font)
	a.applyUIFontToWidgets(font)
	a.scheduleGraphCanvasDraw()
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
	selection = fontSelection{
		Family: family,
		Size:   size,
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

func newFontFaceFromSelection(selection fontSelection) *FontFace {
	if selection.Family == "" || selection.Size == "" {
		return nil
	}
	opts := []Opt{
		Family(selection.Family),
		Size(selection.Size),
		Weight(selection.Weight),
		Slant(selection.Slant),
	}
	if selection.Underline {
		opts = append(opts, Underline(1))
	}
	if selection.Overstrike {
		opts = append(opts, Overstrike(1))
	}
	return NewFont(opts...)
}

func (*Controller) replaceFontFace(target **FontFace, next *FontFace) {
	if target == nil {
		return
	}
	if *target != nil {
		(*target).Delete()
	}
	*target = next
}

func applyFontSelectionToNamedFonts(selection fontSelection, names []string) {
	if len(names) == 0 {
		return
	}
	underline := boolToInt(selection.Underline)
	overstrike := boolToInt(selection.Overstrike)
	for _, name := range names {
		_, err := tkutil.Evalf(
			"font configure %s -family %s -size %s -weight %s -slant %s -underline %d -overstrike %d",
			tkutil.TclSafeString(name),
			tkutil.TclSafeString(selection.Family),
			tkutil.TclSafeString(selection.Size),
			tkutil.TclSafeString(selection.Weight),
			tkutil.TclSafeString(selection.Slant),
			underline,
			overstrike,
		)
		if err != nil {
			slog.Debug(
				"configure named font failed",
				slog.String("font", name),
				slog.String("family", selection.Family),
				slog.String("size", selection.Size),
				slog.Any("error", err),
			)
		}
	}
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func applyUIFontToStyles(font *FontFace) {
	styles := []string{
		".",
		"TLabel",
		"TEntry",
		"TButton",
		"Treeview",
		"Treeview.Heading",
	}
	for _, style := range styles {
		StyleConfigure(style, Font(font))
	}
}

func applyUIFontToOptions(font *FontFace) {
	if font == nil {
		return
	}
	fontName := tkutil.TclSafeString(font.String())
	if _, err := tkutil.Evalf("option add *font %s", fontName); err != nil {
		slog.Debug("option add font", slog.Any("error", err))
	}
	if _, err := tkutil.Evalf("option add *Menu.font %s", fontName); err != nil {
		slog.Debug("option add menu font", slog.Any("error", err))
	}
}

func (a *Controller) applyUIFontToWidgets(font *FontFace) {
	if a.ui.diffFileList != nil {
		a.ui.diffFileList.Configure(Font(font))
	}
	if a.ui.treeContextMenu != nil {
		a.ui.treeContextMenu.Configure(Font(font))
	}
	if a.ui.diffContextMenu != nil {
		a.ui.diffContextMenu.Configure(Font(font))
	}
	if a.ui.menubar != nil {
		a.ui.menubar.Configure(Font(font))
	}
	if a.ui.fileMenu != nil {
		a.ui.fileMenu.Configure(Font(font))
	}
	if a.ui.viewMenu != nil {
		a.ui.viewMenu.Configure(Font(font))
	}
	if a.ui.helpMenu != nil {
		a.ui.helpMenu.Configure(Font(font))
	}
}
