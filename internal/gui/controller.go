package gui

import (
	"github.com/thiagokokada/gitk-go/internal/debounce"
	"github.com/thiagokokada/gitk-go/internal/git"
	"github.com/thiagokokada/gitk-go/internal/gui/model"
	"github.com/thiagokokada/gitk-go/internal/gui/view"
	"github.com/thiagokokada/gitk-go/internal/gui/widgets"
	"github.com/thiagokokada/gitk-go/internal/watch"
)

type Controller struct {
	svc *git.Service

	cfg   controllerConfig
	theme controllerTheme
	prefs controllerPreferences

	ui view.App

	model   model.App
	runtime controllerRuntime
}

type controllerConfig struct {
	batch               uint
	graphCanvas         bool
	autoReloadRequested bool
	syntaxHighlight     bool
	verbose             bool
}

type controllerTheme struct {
	pref     ThemePreference
	palette  colorPalette
	activate func(string) error
}

type controllerPreferences struct {
	uiFontSpec    []string
	fixedFontSpec []string
}

type controllerRuntime struct {
	actions     controllerActions
	watch       *watch.Watcher
	graphCanvas *widgets.GraphCanvas
}

type controllerActions struct {
	filterDebounce debounce.Action[string]
	diffDebounce   debounce.Action[model.DiffRequest]
}
