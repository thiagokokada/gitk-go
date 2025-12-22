package gui

import (
	"github.com/thiagokokada/gitk-go/internal/git"
)

type Controller struct {
	svc *git.Service

	cfg   controllerConfig
	repo  controllerRepo
	theme controllerTheme
	data  controllerData

	ui appWidgets

	state controllerState
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
	pref    ThemePreference
	palette colorPalette
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
	selection selectionState
	watch     autoReloadState
}
