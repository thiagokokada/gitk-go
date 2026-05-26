package gui

import (
	"fmt"
	"log/slog"
	"runtime"
	"strings"

	"github.com/thiagokokada/gitk-go/internal/buildinfo"
	"github.com/thiagokokada/gitk-go/internal/git"
	. "modernc.org/tk9.0"
)

func (a *Controller) initMenubar() {
	menubar := Menu(Tearoff(false))
	a.ui.Menubar = menubar

	openAccel := "Ctrl+O"
	branchAccel := "Ctrl+B"
	if runtime.GOOS == "darwin" {
		openAccel = "Cmd+O"
		branchAccel = "Cmd+B"
	}

	fileMenu := menubar.Menu(Tearoff(false))
	a.ui.FileMenu = fileMenu
	fileMenu.AddCommand(Lbl("Open Repository..."), Accelerator(openAccel), Command(a.promptRepositorySwitch))
	fileMenu.AddCommand(Lbl("Switch Branch..."), Accelerator(branchAccel), Command(a.promptBranchSwitch))
	fileMenu.AddSeparator()
	fileMenu.AddCommand(Lbl("Quit"), Command(func() { Destroy(App) }))
	menubar.AddCascade(Lbl("File"), Mnu(fileMenu))

	viewMenu := menubar.Menu(Tearoff(false))
	a.ui.ViewMenu = viewMenu
	viewMenu.AddCommand(Lbl("UI Font..."), Command(a.showUIFontDialog))
	viewMenu.AddCommand(Lbl("Fixed Font..."), Command(a.showFixedFontDialog))
	menubar.AddCascade(Lbl("View"), Mnu(viewMenu))

	helpMenu := menubar.Menu(Tearoff(false))
	a.ui.HelpMenu = helpMenu
	helpMenu.AddCommand(Lbl("Keyboard Shortcuts"), Command(a.showShortcutsDialog))
	helpMenu.AddCommand(Lbl("About gitk-go"), Command(a.showAboutDialog))
	menubar.AddCascade(Lbl("Help"), Mnu(helpMenu))

	App.Configure(Mnu(menubar))
}

func (a *Controller) promptRepositorySwitch() {
	dir := strings.TrimSpace(ChooseDirectory(
		Parent(App),
		Title("Select Git repository"),
		Initialdir(a.model.Repo.Path),
		Mustexist(true),
	))
	if dir == "" || dir == a.model.Repo.Path {
		return
	}
	a.switchRepository(dir)
}

func (*Controller) showAboutDialog() {
	message := fmt.Sprintf("gitk-go %s", buildinfo.VersionWithTags())
	if gitVer, err := git.GitVersion(); gitVer != "" {
		message += "\n" + gitVer
		if err != nil {
			message += fmt.Sprintf(" (warning: %v)", err)
		}
	} else if err != nil {
		message += fmt.Sprintf("\ngit: %v", err)
	}
	MessageBox(
		Parent(App),
		Title("About gitk-go"),
		Icon("info"),
		Msg(message),
		Type("ok"),
	)
}

func (a *Controller) switchRepository(path string) {
	newSvc, err := git.Open(path)
	if err != nil {
		MessageBox(
			Parent(App),
			Title("Open Repository"),
			Icon("error"),
			Msg(fmt.Sprintf("Unable to open repository:\n\n%v", err)),
			Type("ok"),
		)
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
