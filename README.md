## gitk-go

![Icon](internal/gui/assets/appicon.svg)

`gitk-go` is a lightweight Git history explorer written in Go. It recreates
much of `gitk` using
[`modernc.org/tk9.0`](https://pkg.go.dev/modernc.org/tk9.0) and the system
`git` binary.

![Screenshot](screenshot.png)

### Features

- Three-column commit list with branch graph, author, and date columns
- Background batching keeps the UI responsive and automatically loads more
  commits
- Diff viewer highlights additions, removals, headers, and supports per-file
  navigation plus optional syntax highlighting
- Built-in file list to jump to specific file diffs
- Keyboard shortcuts mirroring common `gitk` bindings (navigation, paging,
  reload). Press `F1` to see all shortcuts
- Automatic reload watcher (with UI toggle) to keep history fresh as the
  repository changes
- Auto-detects OS dark mode with optional manual override

### Usage

```bash
$ gitk-go -h
Usage of gitk-go:
  -graph-cols uint
    	max number of graph columns to render (lower uses less CPU/memory) (default 200)
  -limit uint
    	number of commits to load per batch (larger uses more CPU/memory) (default 1000)
  -mode string
    	color mode: auto, light, or dark (default "auto")
  -nosyntax
    	disable syntax highlighting in the diff viewer
  -nowatch
    	disable automatic reload when repository changes
  -text-graph
    	render commit graph as text (disables canvas graph)
  -verbose
    	enable verbose logging
  -version
    	print version information and exit
```

### Known issues

- Automatic reload doesn't work well with `core.fsmonitor` option from `git`
  enabled
- Automatic reload doesn't detect untracked files since [fsmonitor doesn't
  support recursive watcher
  yet](https://github.com/fsnotify/fsnotify/issues/18)
- Light theme seems to be much slower than dark theme (not noticeable unless
  you're using a slow device)

### Development

```bash
go test ./...
go build ./...
```

See `AGENTS.md` for guidelines followed by the automation helping maintain this
project.
