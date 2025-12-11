## gitk-go

![Icon](internal/gui/assets/appicon.svg)

`gitk-go` is a lightweight Git history explorer written in Go. It recreates much
of `gitk` using [`modernc.org/tk9.0`](https://pkg.go.dev/modernc.org/tk9.0) and
[`go-git`](https://github.com/go-git/go-git), and it can optionally leverage the
system `git` binary for faster local-change handling.

![Screenshot](screenshot.png)

### Features

- Three-column commit list with branch graph, author, and date columns
- Background batching keeps the UI responsive and automatically loads more
- Diff viewer highlights additions, removals, headers, and supports per-file navigation
- Built-in file list to jump to specific file diffs
- Keyboard shortcuts mirroring common gitk bindings (navigation, paging, reload)
- Optional acceleration using the system `git` CLI (see below)
- Auto-detects OS dark mode with optional manual override

### Usage

```bash
go run . [-limit N] [-mode auto|light|dark] /path/to/repo
```

Arguments:

- First positional argument (optional): repository root or `.git` directory (defaults to current directory)
- `-limit` (default `1000`): number of commits to load per batch
- `-mode` (default `auto`): choose light, dark, or auto-detected theme
- `-version`: print the build version (plus active build tags when available) and exit. Version info comes from Go's build metadata (`go build -buildvcs` captures git revision automatically).

#### git CLI acceleration

By default, `gitk-go` uses pure Go code for repository access. For large
repositories you can opt-in to faster local-change detection and diff rendering
by building with the `gitcli` build tag:

```bash
go build -tags gitcli
```

This requires the `git` binary to be available in `$PATH`. If the binary is not
available the build will still succeed, but the accelerated paths will fail at
runtime, so only enable the tag when `git` is installed.

### Development

```bash
go test ./...
go test -tags gitcli ./...
go build ./...
go build -tags gitcli ./...
```

Key packages:

- `cmd`: CLI parsing and entry point
- `internal/git`: repository access, commit scanning, graph building
- `internal/gui`: Tk UI and controller logic

See `AGENTS.md` for guidelines followed by the automation helping maintain this project.
