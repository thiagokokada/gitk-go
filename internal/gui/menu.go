package gui

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/thiagokokada/gitk-go/internal/buildinfo"
	"github.com/thiagokokada/gitk-go/internal/git"
	"github.com/thiagokokada/gitk-go/internal/gui/view"
	tk "modernc.org/tk9.0"
)

func (a *Controller) initMenubar() {
	a.ui.InitMenubar(view.MenuHandlers{
		OpenRepository: a.promptRepositorySwitch,
		SwitchBranch:   a.promptBranchSwitch,
		UIFont: func() {
			view.ShowFontDialog(
				"Select UI Font",
				view.FontChooserSeed(tk.DefaultFont, a.prefs.uiFontSpec),
				a.applyUIFontSpec,
			)
		},
		FixedFont: func() {
			view.ShowFontDialog(
				"Select Fixed Font",
				view.FontChooserSeed(tk.FixedFont, a.prefs.fixedFontSpec),
				a.applyFixedFontSpec,
			)
		},
		Shortcuts: func() {
			a.ui.ShowShortcutsDialog(formatShortcutsHelpText(a.shortcutBindings()))
		},
		About: showAboutDialog,
	})
}

func (a *Controller) promptRepositorySwitch() {
	dir := strings.TrimSpace(view.ChooseRepositoryDirectory("Select Git repository", a.model.Repo.Path))
	if dir == "" || dir == a.model.Repo.Path {
		return
	}
	a.switchRepository(dir)
}

func showAboutDialog() {
	message := fmt.Sprintf("gitk-go %s", buildinfo.VersionWithTags())
	if gitVer, err := git.GitVersion(); gitVer != "" {
		message += "\n" + gitVer
		if err != nil {
			message += fmt.Sprintf(" (warning: %v)", err)
		}
	} else if err != nil {
		message += fmt.Sprintf("\ngit: %v", err)
	}
	view.ShowMessage("About gitk-go", "info", message)
}

func (a *Controller) switchRepository(path string) {
	newSvc, err := git.Open(path)
	if err != nil {
		view.ShowMessage("Open Repository", "error", fmt.Sprintf("Unable to open repository:\n\n%v", err))
		return
	}

	a.runtime.watch.mu.Lock()
	wasConfigured := a.runtime.watch.configured
	wasEnabled := a.runtime.watch.enabled
	a.runtime.watch.mu.Unlock()

	a.disableAutoReload()
	a.cancelPendingDiffLoad()

	a.svc = newSvc
	a.clearTreeRows()
	a.runtime.actions.filterDebounce.Stop()
	a.model.ResetRepository(newSvc.RepoPath())
	a.ui.ClearFilterText()
	a.setLocalRowVisibility(false, false)
	a.setLocalRowVisibility(true, false)
	a.updateRepoLabel()
	a.clearDetailText("Select a commit to view its details.")
	a.showInitialLoadingRow()
	a.setStatus("Loading commits...")

	if wasConfigured && wasEnabled {
		if err := a.enableAutoReload(); err != nil {
			slog.Error("auto reload enable failed", slog.Any("error", err))
		}
	}
	a.updateReloadButtonLabel()
	a.refreshLocalChangesAsync(true)
	a.reloadCommitsAsync()
}
