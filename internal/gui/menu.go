package gui

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/thiagokokada/gitk-go/internal/buildinfo"
	"github.com/thiagokokada/gitk-go/internal/git"
	. "modernc.org/tk9.0"
)

func (a *Controller) initMenubar() {
	menubar := Menu(Tearoff(false))

	fileMenu := menubar.Menu(Tearoff(false))
	fileMenu.AddCommand(Lbl("Open Repository..."), Command(a.promptRepositorySwitch))
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
		Initialdir(a.repoPath),
		Mustexist(true),
	))
	if dir == "" || dir == a.repoPath {
		return
	}
	a.switchRepository(dir)
}

func (a *Controller) showAboutDialog() {
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

	a.watch.mu.Lock()
	wasConfigured := a.watch.configured
	wasEnabled := a.watch.enabled
	a.watch.mu.Unlock()

	a.disableAutoReload()
	a.cancelPendingDiffLoad()

	a.svc = newSvc
	a.repoPath = newSvc.RepoPath()
	a.headRef = ""
	a.commits = nil
	a.visible = nil
	a.tree.branchLabels = nil
	a.tree.hasMore = false
	a.tree.contextTargetID = ""
	a.tree.loadingBatch = false
	a.selection.Set("")
	a.localDiffs = localDiffCache{}
	a.filter.value = ""
	a.stopFilterDebounce()
	if a.filter.entry != nil {
		a.filter.entry.Configure(Textvariable(""))
	}
	if a.diff.fileList != nil {
		a.diff.fileList.Delete(0, END)
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
