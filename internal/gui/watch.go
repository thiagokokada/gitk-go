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
	a.state.watch.mu.Lock()
	defer a.state.watch.mu.Unlock()
	if !a.state.watch.configured {
		return nil
	}
	if a.state.watch.enabled {
		return nil
	}
	options := []fswatcher.WatcherOpt{
		fswatcher.WithCooldown(100 * time.Millisecond),
	}
	slog.Debug("adding path to FS watcher", slog.String("path", a.repo.path))
	options = append(options, fswatcher.WithPath(a.repo.path))
	watcher, err := fswatcher.New(options...)
	if err != nil {
		return fmt.Errorf("fswatcher: %w", err)
	}
	if a.state.watch.debounce == nil {
		a.state.watch.debounce = debounce.New(autoReloadDebounceDelay, func() {
			PostEvent(func() {
				a.reloadCommitsAsync()
			}, false)
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.state.watch.watcher = watcher
	a.state.watch.cancel = cancel
	a.state.watch.enabled = true
	go a.watchLoop(ctx, watcher)
	return nil
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
}

func (a *Controller) watchLoop(ctx context.Context, w fswatcher.Watcher) {
	errCh := make(chan error, 1)
	go func() {
		errCh <- w.Watch(ctx)
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errCh:
			if err != nil && !errors.Is(err, context.Canceled) {
				slog.Error("fswatcher error", slog.Any("error", err))
			}
			return
		case ev, ok := <-w.Events():
			if !ok {
				return
			}
			if !eventTriggersReload(ev) {
				continue
			}
			if shouldIgnoreWatchPath(ev.Path) {
				continue
			}
			slog.Debug("fswatcher event",
				slog.Any("types", eventTypeNames(ev.Types)),
				slog.String("path", ev.Path),
			)
			a.scheduleAutoReload()
		}
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

func eventTriggersReload(event fswatcher.WatchEvent) bool {
	if len(event.Types) == 0 {
		return true
	}
	for _, eventType := range event.Types {
		switch eventType {
		case fswatcher.EventCreate, fswatcher.EventRemove, fswatcher.EventMod,
			fswatcher.EventRename, fswatcher.EventChmod, fswatcher.EventUnknown:
			return true
		default:
		}
	}
	return false
}

func eventTypeNames(types []fswatcher.EventType) []string {
	if len(types) == 0 {
		return nil
	}
	names := make([]string, 0, len(types))
	for _, eventType := range types {
		names = append(names, eventType.String())
	}
	return names
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
