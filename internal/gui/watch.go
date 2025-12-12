package gui

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	. "modernc.org/tk9.0"
)

const autoReloadDebounceDelay = 350 * time.Millisecond

type autoReloadState struct {
	mu         sync.Mutex
	configured bool
	enabled    bool
	watcher    *fsnotify.Watcher
	debounce   *debouncer
	button     *TButtonWidget
}

func (a *Controller) initAutoReload(requested bool) {
	a.watch.mu.Lock()
	a.watch.configured = requested
	a.watch.mu.Unlock()
	if requested {
		if err := a.enableAutoReload(); err != nil {
			log.Printf("auto reload disabled: %v", err)
			a.watch.mu.Lock()
			a.watch.configured = false
			a.watch.mu.Unlock()
		}
	}
	a.updateReloadButtonLabel()
}

func (a *Controller) enableAutoReload() error {
	a.watch.mu.Lock()
	defer a.watch.mu.Unlock()
	if !a.watch.configured {
		return nil
	}
	if a.watch.enabled {
		return nil
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("fsnotify: %w", err)
	}
	for _, path := range watchPaths(a.repoPath) {
		if err := watcher.Add(path); err != nil {
			watcher.Close()
			return fmt.Errorf("watch %s: %w", path, err)
		}
	}
	if a.watch.debounce == nil {
		a.watch.debounce = newDebouncer(autoReloadDebounceDelay, func() {
			PostEvent(func() {
				a.reloadCommitsAsync()
			}, false)
		})
	}
	a.watch.watcher = watcher
	a.watch.enabled = true
	go a.watchLoop(watcher)
	return nil
}

func (a *Controller) disableAutoReload() {
	a.watch.mu.Lock()
	defer a.watch.mu.Unlock()
	if a.watch.debounce != nil {
		a.watch.debounce.Stop()
		a.watch.debounce = nil
	}
	if a.watch.watcher != nil {
		a.watch.watcher.Close()
		a.watch.watcher = nil
	}
	a.watch.enabled = false
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
			a.debugf("fsnotify: %s %s", ev.Op.String(), ev.Name)
			a.scheduleAutoReload()
		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			log.Printf("fsnotify error: %v", err)
		}
	}
}

func (a *Controller) scheduleAutoReload() {
	a.watch.mu.Lock()
	defer a.watch.mu.Unlock()
	if !a.watch.enabled || a.watch.debounce == nil {
		return
	}
	a.debugf("auto reload: scheduling diff refresh")
	a.watch.debounce.Trigger()
}

func watchPaths(root string) []string {
	if root == "" {
		return nil
	}
	var paths []string
	appendUnique := func(p string) {
		for _, existing := range paths {
			if existing == p {
				return
			}
		}
		paths = append(paths, p)
	}
	gitDir := filepath.Join(root, ".git")
	if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
		appendUnique(gitDir)
		return paths
	}
	appendUnique(root)
	return paths
}

func shouldIgnoreWatchPath(name string) bool {
	if name == "" {
		return false
	}
	base := strings.ToLower(filepath.Base(name))
	if strings.HasSuffix(base, ".lock") || strings.HasSuffix(base, ".ipc") {
		return true
	}
	return false
}

func (a *Controller) updateReloadButtonLabel() {
	if a.watch.button == nil {
		return
	}
	label := "Reload"
	a.watch.mu.Lock()
	configured := a.watch.configured
	enabled := a.watch.enabled
	a.watch.mu.Unlock()
	if configured {
		state := "Off"
		if enabled {
			state = "On"
		}
		label = fmt.Sprintf("Reload (Auto %s)", state)
	}
	a.watch.button.Configure(Txt(label))
}

func (a *Controller) onReloadButton() {
	a.watch.mu.Lock()
	configured := a.watch.configured
	enabled := a.watch.enabled
	a.watch.mu.Unlock()
	if !configured {
		a.reloadCommitsAsync()
		return
	}
	if enabled {
		a.disableAutoReload()
	} else {
		if err := a.enableAutoReload(); err != nil {
			log.Printf("auto reload enable failed: %v", err)
		}
	}
	a.updateReloadButtonLabel()
	a.reloadCommitsAsync()
}
