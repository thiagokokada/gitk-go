package gui

import (
	"context"
	"log/slog"
	"sync"

	"github.com/thiagokokada/gitk-go/internal/gui/view"
	. "modernc.org/tk9.0"
)

type themeWatchState struct {
	mu      sync.Mutex
	cancel  context.CancelFunc
	running bool
}

func (a *Controller) startThemeWatch() {
	if a.theme.pref != ThemeAuto || watchDarkMode == nil {
		return
	}
	a.theme.watch.mu.Lock()
	if a.theme.watch.running {
		a.theme.watch.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.theme.watch.cancel = cancel
	a.theme.watch.running = true
	a.theme.watch.mu.Unlock()

	events, errs, err := watchDarkMode(ctx)
	if err != nil {
		slog.Error("watch dark-mode", slog.Any("error", err))
		a.stopThemeWatch()
		return
	}
	go a.themeWatchLoop(events, errs)
}

func (a *Controller) stopThemeWatch() {
	a.theme.watch.mu.Lock()
	defer a.theme.watch.mu.Unlock()
	if a.theme.watch.cancel != nil {
		a.theme.watch.cancel()
		a.theme.watch.cancel = nil
	}
	a.theme.watch.running = false
}

func (a *Controller) themeWatchLoop(events <-chan bool, errs <-chan error) {
	defer a.stopThemeWatch()
	for {
		select {
		case dark, ok := <-events:
			if !ok {
				return
			}
			PostEvent(func() {
				a.onSystemThemeChanged(dark)
			}, false)
		case err, ok := <-errs:
			if ok && err != nil {
				slog.Error("dark-mode watch", slog.Any("error", err))
			}
			return
		}
	}
}

func (a *Controller) onSystemThemeChanged(dark bool) {
	next, changed := paletteForThemeChange(a.theme.pref, a.theme.palette, dark)
	if !changed {
		return
	}
	a.applyThemePalette(next)
}

func (a *Controller) applyThemePalette(palette colorPalette) {
	if a.theme.palette == palette {
		return
	}
	a.theme.palette = palette
	if palette.ThemeName != "" {
		if err := a.activateTheme(palette.ThemeName); err != nil {
			slog.Error(
				"activate theme",
				slog.String("theme", palette.ThemeName),
				slog.Any("error", err),
			)
		}
	}
	a.refreshThemeStyles()
}

func (a *Controller) activateTheme(name string) error {
	if a.theme.activate == nil {
		return nil
	}
	return a.theme.activate(name)
}

func (a *Controller) refreshThemeStyles() {
	a.applyTreeRowStyles()
	a.applyDiffTagStyles()
	a.applyDiffFileListStyles()
	a.refreshSyntaxHighlight()
	a.scheduleGraphCanvasDraw()
}

func (a *Controller) applyTreeRowStyles() {
	unstagedColor := a.theme.palette.LocalUnstagedRow
	if unstagedColor == "" {
		unstagedColor = lightPalette.LocalUnstagedRow
	}
	stagedColor := a.theme.palette.LocalStagedRow
	if stagedColor == "" {
		stagedColor = lightPalette.LocalStagedRow
	}
	a.ui.ApplyTreeRowStyles(view.TreeRowColors{
		LocalUnstaged: unstagedColor,
		LocalStaged:   stagedColor,
	})
}

func (a *Controller) applyDiffTagStyles() {
	addColor := a.theme.palette.DiffAdd
	if addColor == "" {
		addColor = lightPalette.DiffAdd
	}
	delColor := a.theme.palette.DiffDel
	if delColor == "" {
		delColor = lightPalette.DiffDel
	}
	headerColor := a.theme.palette.DiffHeader
	if headerColor == "" {
		headerColor = lightPalette.DiffHeader
	}
	a.ui.ApplyDiffTagStyles(view.DiffColors{
		Add:    addColor,
		Delete: delColor,
		Header: headerColor,
	})
}

func (a *Controller) applyDiffFileListStyles() {
	addColor := a.theme.palette.DiffAddText
	if addColor == "" {
		addColor = lightPalette.DiffAddText
	}
	delColor := a.theme.palette.DiffDelText
	if delColor == "" {
		delColor = lightPalette.DiffDelText
	}
	a.ui.ApplyDiffFileListStyles(view.DiffFileListColors{
		AddText:    addColor,
		DeleteText: delColor,
	})
}

func (a *Controller) refreshSyntaxHighlight() {
	if !a.cfg.syntaxHighlight || !a.shouldHighlightDiff() {
		return
	}
	content := a.currentDiffText()
	if content == "" {
		return
	}
	a.maybeStartSyntaxHighlight(content, true)
}

func (a *Controller) shouldHighlightDiff() bool {
	return len(a.model.State.Diff.FileSections) > 0
}

func (a *Controller) currentDiffText() string {
	return a.ui.CurrentDiffText()
}
