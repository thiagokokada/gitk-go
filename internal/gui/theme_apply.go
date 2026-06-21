package gui

import (
	"log/slog"

	"github.com/thiagokokada/gitk-go/internal/gui/view"
)

func (a *Controller) setThemePalette(palette colorPalette) bool {
	if a.theme.palette == palette {
		return false
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
	return true
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
	if !a.cfg.syntaxHighlight || len(a.model.State.Diff.FileSections) == 0 {
		return
	}
	content := a.ui.CurrentDiffText()
	if content == "" {
		return
	}
	a.maybeStartSyntaxHighlight(content, true)
}
