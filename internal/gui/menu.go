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

	openAccel := "Ctrl+O"
	branchAccel := "Ctrl+B"
	if runtime.GOOS == "darwin" {
		openAccel = "Cmd+O"
		branchAccel = "Cmd+B"
	}

	fileMenu := menubar.Menu(Tearoff(false))
	fileMenu.AddCommand(Lbl("Open Repository..."), Accelerator(openAccel), Command(a.promptRepositorySwitch))
	fileMenu.AddCommand(Lbl("Switch Branch..."), Accelerator(branchAccel), Command(a.promptBranchSwitch))
	fileMenu.AddSeparator()
	fileMenu.AddCommand(Lbl("Quit"), Command(func() { Destroy(App) }))
	menubar.AddCascade(Lbl("File"), Mnu(fileMenu))

	helpMenu := menubar.Menu(Tearoff(false))
	helpMenu.AddCommand(Lbl("Keyboard Shortcuts"), Command(a.showShortcutsDialog))
	helpMenu.AddCommand(Lbl("About gitk-go"), Command(a.showAboutDialog))
	menubar.AddCascade(Lbl("Help"), Mnu(helpMenu))

	App.Configure(Mnu(menubar))
}

func (a *Controller) promptRepositorySwitch() {
	dir := strings.TrimSpace(ChooseDirectory(
		Parent(App),
		Title("Select Git repository"),
		Initialdir(a.repo.path),
		Mustexist(true),
	))
	if dir == "" || dir == a.repo.path {
		return
	}
	a.switchRepository(dir)
}

func (*Controller) showAboutDialog() {
	message := fmt.Sprintf("gitk-go %s", buildinfo.VersionWithTags())
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

	a.state.watch.mu.Lock()
	wasConfigured := a.state.watch.configured
	wasEnabled := a.state.watch.enabled
	a.state.watch.mu.Unlock()

	a.disableAutoReload()
	a.cancelPendingDiffLoad()

	a.svc = newSvc
	a.repo.path = newSvc.RepoPath()
	a.repo.headRef = ""
	a.data.commits = nil
	a.data.visible = nil
	a.state.tree = treeState{}
	a.state.localDiff = localDiffCache{}
	a.state.filter = filterState{}
	a.state.selection = selectionState{}
	a.stopFilterDebounce()
	if a.ui.filterEntry != nil {
		a.ui.filterEntry.Configure(Textvariable(""))
	}
	if a.ui.diffFileList != nil {
		a.ui.diffFileList.Delete(0, END)
	}

	a.setLocalRowVisibility(false, false)
	a.setLocalRowVisibility(true, false)

	a.clearTreeRows()
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
