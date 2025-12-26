package gui

import (
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/thiagokokada/gitk-go/internal/debounce"
	. "modernc.org/tk9.0"
)

const autoReloadDebounceDelay = 350 * time.Millisecond

type autoReloadState struct {
	mu         sync.Mutex
	configured bool
	enabled    bool
	watcher    *fsnotify.Watcher
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
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("fsnotify: %w", err)
	}
	for path := range watchPaths(a.repo.path) {
		slog.Debug("adding path to FS watcher", slog.String("path", path))
		if err := watcher.Add(path); err != nil {
			err := errors.Join(err, watcher.Close())
			return fmt.Errorf("watch %s: %w", path, err)
		}
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
		err := a.state.watch.watcher.Close()
		if err != nil {
			slog.Error("watcher close", slog.Any("error", err))
		}
		a.state.watch.watcher = nil
	}
	a.state.watch.enabled = false
}

func (a *Controller) shutdown() {
	a.disableAutoReload()
}

func (a *Controller) watchLoop(w *fsnotify.Watcher) {
	for {
		select {
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}
			if shouldIgnoreWatchPath(ev.Name) {
				continue
			}
			slog.Debug("fsnotify event",
				slog.String("op", ev.Op.String()),
				slog.String("path", ev.Name),
			)
			a.scheduleAutoReload()
		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			slog.Error("fsnotify error", slog.Any("error", err))
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

func watchPaths(root string) iter.Seq[string] {
	if root == "" {
		return nil
	}
	uniquePaths := map[string]struct{}{}
	appendUnique := func(p string) { uniquePaths[p] = struct{}{} }
	gitDir := filepath.Join(root, ".git")
	if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
		appendUnique(gitDir)
		return maps.Keys(uniquePaths)
	}
	appendUnique(root)
	return maps.Keys(uniquePaths)
}

func shouldIgnoreWatchPath(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	if ext == ".lock" || ext == ".ipc" {
		return true
	}
	return false
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
