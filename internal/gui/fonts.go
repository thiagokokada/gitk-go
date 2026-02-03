package gui

import (
	"log/slog"
	"runtime"
	"strings"

	"github.com/thiagokokada/gitk-go/internal/gui/tkutil"

	. "modernc.org/tk9.0"
)

var (
	linuxSansFontCandidates = []string{
		"DejaVu Sans",
		"Noto Sans",
		"Ubuntu",
		"Liberation Sans",
		"Cantarell",
		"Arial",
		"Sans",
	}
	linuxMonoFontCandidates = []string{
		"DejaVu Sans Mono",
		"Noto Sans Mono",
		"Ubuntu Mono",
		"Liberation Mono",
		"Monospace",
		"Courier New",
		"Courier",
	}
	linuxSansNamedFonts = []string{
		DefaultFont,
		TextFont,
		MenuFont,
		HeadingFont,
		CaptionFont,
		SmallCaptionFont,
		IconFont,
		TooltipFont,
	}
)

func diffDetailFontSpec() []any {
	if runtime.GOOS == "linux" {
		return []any{FixedFont}
	}
	return []any{CourierFont(), 11}
}

func (a *Controller) configureFonts() {
	if runtime.GOOS != "linux" {
		return
	}
	families, err := tkFontFamilies()
	if err != nil {
		slog.Debug("font families lookup failed", slog.Any("error", err))
		return
	}
	sans := pickFontFamily(families, linuxSansFontCandidates)
	mono := pickFontFamily(families, linuxMonoFontCandidates)
	if sans == "" && mono == "" {
		slog.Debug("no matching font families found")
		return
	}
	if sans != "" {
		applyNamedFontFamily(sans, linuxSansNamedFonts)
	}
	if mono != "" {
		applyNamedFontFamily(mono, []string{FixedFont})
	}
	slog.Debug("font families configured", slog.String("sans", sans), slog.String("mono", mono))
}

func tkFontFamilies() ([]string, error) {
	out, err := tkutil.Evalf("join [font families] \\n")
	if err != nil {
		return nil, err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

func pickFontFamily(available []string, candidates []string) string {
	if len(available) == 0 || len(candidates) == 0 {
		return ""
	}
	availableByLower := make(map[string]string, len(available))
	for _, name := range available {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		availableByLower[strings.ToLower(name)] = name
	}
	for _, candidate := range candidates {
		if match, ok := availableByLower[strings.ToLower(candidate)]; ok {
			return match
		}
	}
	return ""
}

func applyNamedFontFamily(family string, names []string) {
	family = strings.TrimSpace(family)
	if family == "" {
		return
	}
	for _, name := range names {
		_, err := tkutil.Evalf(
			"font configure %s -family %s",
			tkutil.TclSafeString(name),
			tkutil.TclSafeString(family),
		)
		if err != nil {
			slog.Debug(
				"configure font family",
				slog.String("font", name),
				slog.String("family", family),
				slog.Any("error", err),
			)
		}
	}
}
