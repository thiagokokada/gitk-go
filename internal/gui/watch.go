package gui

import (
	"fmt"
	"log/slog"

	. "modernc.org/tk9.0"
)

func (a *Controller) initAutoReload(requested bool) {
	a.runtime.watch.Init(requested)
	if requested {
		if err := a.enableAutoReload(); err != nil {
			slog.Error("auto reload disabled", slog.Any("error", err))
			a.runtime.watch.Init(false)
		}
	}
	a.updateReloadButtonLabel()
}

func (a *Controller) enableAutoReload() error {
	return a.runtime.watch.Start(a.model.Repo.Path, func() {
		PostEvent(func() {
			a.reloadCommitsAsync()
		}, false)
	})
}

func (a *Controller) disableAutoReload() {
	a.runtime.watch.Stop()
}

func (a *Controller) shutdown() {
	a.disableAutoReload()
}

func (a *Controller) updateReloadButtonLabel() {
	label := "Reload"
	configured := a.runtime.watch.IsConfigured()
	enabled := a.runtime.watch.IsEnabled()
	if configured {
		state := "Off"
		if enabled {
			state = "On"
		}
		label = fmt.Sprintf("Reload (Auto %s)", state)
	}
	a.ui.SetReloadButtonLabel(label)
}

func (a *Controller) onReloadButton() {
	configured := a.runtime.watch.IsConfigured()
	enabled := a.runtime.watch.IsEnabled()
	if !configured {
		a.reloadCommitsAsync()
		return
	}
	if enabled {
		a.disableAutoReload()
	} else {
		if err := a.enableAutoReload(); err != nil {
			slog.Error("auto reload enable failed", slog.Any("error", err))
		}
	}
	a.updateReloadButtonLabel()
	a.reloadCommitsAsync()
}
