package gui

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sgtdi/fswatcher"
	"github.com/thiagokokada/gitk-go/internal/debounce"
	. "modernc.org/tk9.0"
)

const autoReloadDebounceDelay = 350 * time.Millisecond

type autoReloadState struct {
	mu         sync.Mutex
	configured bool
	enabled    bool
	watcher    fswatcher.Watcher
	cancel     context.CancelFunc
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
	if !a.shouldEnableAutoReload() {
		return nil
	}
	slog.Debug("adding path to fswatcher", slog.String("path", a.repo.path))
	watcher, err := fswatcher.New(
		fswatcher.WithPath(a.repo.path, fswatcher.WithDepth(fswatcher.WatchNested)),
	)
	if err != nil {
		return fmt.Errorf("watch %s: %w", a.repo.path, err)
	}
	ctx, cancel := context.WithCancel(context.Background())

	if !a.attachAutoReloadWatcher(watcher, cancel) {
		cancel()
		watcher.Close()
		return nil
	}
	go a.watchLoop(watcher.Events())
	go func() {
		if err := watcher.Watch(ctx); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("auto reload watch failed", slog.Any("error", err))
		}
	}()
	return nil
}

func (a *Controller) shouldEnableAutoReload() bool {
	a.state.watch.mu.Lock()
	defer a.state.watch.mu.Unlock()
	return a.state.watch.configured && !a.state.watch.enabled
}

func (a *Controller) attachAutoReloadWatcher(watcher fswatcher.Watcher, cancel context.CancelFunc) bool {
	a.state.watch.mu.Lock()
	defer a.state.watch.mu.Unlock()
	if !a.state.watch.configured || a.state.watch.enabled {
		return false
	}
	if a.state.watch.debounce == nil {
		a.state.watch.debounce = debounce.New(autoReloadDebounceDelay, func() {
			PostEvent(func() {
				a.reloadCommitsAsync()
			}, false)
		})
	}
	a.state.watch.watcher = watcher
	a.state.watch.cancel = cancel
	a.state.watch.enabled = true
	return true
}

func (a *Controller) disableAutoReload() {
	a.state.watch.mu.Lock()
	defer a.state.watch.mu.Unlock()
	if a.state.watch.debounce != nil {
		a.state.watch.debounce.Stop()
		a.state.watch.debounce = nil
	}
	if a.state.watch.cancel != nil {
		a.state.watch.cancel()
		a.state.watch.cancel = nil
	}
	if a.state.watch.watcher != nil {
		a.state.watch.watcher.Close()
		a.state.watch.watcher = nil
	}
	a.state.watch.enabled = false
}

func (a *Controller) shutdown() {
	a.disableAutoReload()
	a.stopThemeWatch()
}

func (a *Controller) watchLoop(w <-chan fswatcher.WatchEvent) {
	for ev := range w {
		if !eventTriggersReload(ev.Types) {
			continue
		}
		if shouldIgnoreWatchPath(ev.Path) {
			continue
		}
		slog.Debug(
			"fswatcher event",
			slog.String("event", eventTypesString(ev.Types)),
			slog.String("path", ev.Path),
		)
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

func eventTriggersReload(types []fswatcher.EventType) bool {
	for _, eventType := range types {
		if eventType != fswatcher.EventUnknown {
			return true
		}
	}
	return false
}

func eventTypesString(types []fswatcher.EventType) string {
	if len(types) == 0 {
		return "Unknown"
	}
	names := make([]string, 0, len(types))
	for _, eventType := range types {
		names = append(names, eventType.String())
	}
	return strings.Join(names, "|")
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
