package gui

import (
	"github.com/thiagokokada/gitk-go/internal/debounce"
	"github.com/thiagokokada/gitk-go/internal/git"
	"github.com/thiagokokada/gitk-go/internal/gui/selection"
)

type Controller struct {
	svc *git.Service

	cfg   controllerConfig
	theme controllerTheme
	fonts controllerFonts
	prefs controllerPreferences

	ui appWidgets

	model   appModel
	runtime controllerRuntime
}

type controllerConfig struct {
	batch               uint
	graphCanvas         bool
	autoReloadRequested bool
	syntaxHighlight     bool
	verbose             bool
}

type controllerRepo struct {
	path    string
	headRef string
}

type controllerTheme struct {
	pref     ThemePreference
	palette  colorPalette
	activate func(string) error
	watch    themeWatchState
}

type controllerPreferences struct {
	uiFontSpec    []string
	fixedFontSpec []string
}

type appModel struct {
	repo  controllerRepo
	data  controllerData
	state controllerState
}

type controllerData struct {
	commits []*git.Entry
	visible []*git.Entry
}

type controllerState struct {
	tree      treeState
	diff      diffState
	filter    filterState
	localDiff localDiffCache
	scroll    scrollState
	selection selection.State
}

type controllerRuntime struct {
	actions controllerActions
	watch   autoReloadState
}

type controllerActions struct {
	filterDebounce debounce.Action[string]
	diffDebounce   debounce.Action[diffRequest]
}
