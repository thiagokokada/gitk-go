package gui

import (
	"github.com/thiagokokada/gitk-go/internal/git"
)

type Controller struct {
	svc                 *git.Service
	repoPath            string
	batch               int
	graphCanvas         bool
	themePref           ThemePreference
	palette             colorPalette
	autoReloadRequested bool
	syntaxHighlight     bool
	verbose             bool

	headRef string

	commits []*git.Entry
	visible []*git.Entry

	ui appWidgets

	tree   treeState
	diff   diffState
	filter filterState

	localDiffs localDiffCache
	scroll     scrollState
	selection  selectionState
	watch      autoReloadState
}
