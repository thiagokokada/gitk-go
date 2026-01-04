package gui

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rjeczalik/notify"
	"github.com/thiagokokada/gitk-go/internal/debounce"
	. "modernc.org/tk9.0"
)

const autoReloadDebounceDelay = 350 * time.Millisecond

type autoReloadState struct {
	mu         sync.Mutex
	configured bool
	enabled    bool
	watcher    chan notify.EventInfo
	debounce   *debounce.Debouncer
}

func (a *Controller) initAutoReload(requested bool) {
	a.state.watch.mu.Lock()
	a.state.watch.configured = requested
	a.state.watch.mu.Unlock()
	if requested {
		if err := a.enableAutoReload(); err != nil {
			slog.Error("auto reload disabled", slog.Any("error", err))
			a.state.watch.mu.Lock()
			a.state.watch.configured = false
			a.state.watch.mu.Unlock()
		}
	}
	a.updateReloadButtonLabel()
}

func (a *Controller) enableAutoReload() error {
	a.state.watch.mu.Lock()
	defer a.state.watch.mu.Unlock()
	if !a.state.watch.configured {
		return nil
	}
	if a.state.watch.enabled {
		return nil
	}
	watcher := make(chan notify.EventInfo, 64)
	slog.Debug("adding path to notify watcher", slog.String("path", a.repo.path))
	if err := notify.Watch(a.repo.path, watcher, notify.All); err != nil {
		notify.Stop(watcher)
		return fmt.Errorf("watch %s: %w", a.repo.path, err)
	}
	if a.state.watch.debounce == nil {
		a.state.watch.debounce = debounce.New(autoReloadDebounceDelay, func() {
			PostEvent(func() {
				a.reloadCommitsAsync()
			}, false)
		})
	}
	a.state.watch.watcher = watcher
	a.state.watch.enabled = true
	go a.watchLoop(watcher)
	return nil
}

func (a *Controller) disableAutoReload() {
	a.state.watch.mu.Lock()
	defer a.state.watch.mu.Unlock()
	if a.state.watch.debounce != nil {
		a.state.watch.debounce.Stop()
		a.state.watch.debounce = nil
	}
	if a.state.watch.watcher != nil {
		notify.Stop(a.state.watch.watcher)
		close(a.state.watch.watcher)
		a.state.watch.watcher = nil
	}
	a.state.watch.enabled = false
}

func (a *Controller) shutdown() {
	a.disableAutoReload()
}

func (a *Controller) watchLoop(w <-chan notify.EventInfo) {
	for ev := range w {
		event := ev.Event()
		path := ev.Path()
		if !eventTriggersReload(event) {
			continue
		}
		if shouldIgnoreWatchPath(path) {
			continue
		}
		slog.Debug("notify event", slog.String("event", event.String()), slog.String("path", path))
		a.scheduleAutoReload()
	}
}

func (a *Controller) scheduleAutoReload() {
	a.state.watch.mu.Lock()
	defer a.state.watch.mu.Unlock()
	if !a.state.watch.enabled || a.state.watch.debounce == nil {
		return
	}
	slog.Debug("auto reload scheduled")
	a.state.watch.debounce.Trigger()
}

func shouldIgnoreWatchPath(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	if ext == ".lock" || ext == ".ipc" {
		return true
	}
	return false
}

func eventTriggersReload(event notify.Event) bool {
	return event&notify.All != 0
}

func (a *Controller) updateReloadButtonLabel() {
	label := "Reload"
	a.state.watch.mu.Lock()
	configured := a.state.watch.configured
	enabled := a.state.watch.enabled
	a.state.watch.mu.Unlock()
	if configured {
		state := "Off"
		if enabled {
			state = "On"
		}
		label = fmt.Sprintf("Reload (Auto %s)", state)
	}
	a.ui.reloadButton.Configure(Txt(label))
}

func (a *Controller) onReloadButton() {
	a.state.watch.mu.Lock()
	configured := a.state.watch.configured
	enabled := a.state.watch.enabled
	a.state.watch.mu.Unlock()
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
