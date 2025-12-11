## gitk-go

`gitk-go` is a lightweight Git history explorer written in Go. It recreates much
of `gitk` using [`modernc.org/tk9.0`](https://pkg.go.dev/modernc.org/tk9.0) and
[`go-git`](https://github.com/go-git/go-git) so it runs without Tcl scripts or
external git commands.

### Features

- Three-column commit list with branch graph, author, and date columns
- Background batching keeps the UI responsive and automatically loads more
- Diff viewer highlights additions, removals, headers, and supports per-file navigation
- Built-in file list to jump to specific file diffs
- Keyboard shortcuts mirroring common gitk bindings (navigation, paging, reload)
- Pure-Go git access with no shell commands or extra dependencies

### Usage

```bash
go run . [-limit N] /path/to/repo
```

Arguments:

- First positional argument (optional): repository root or `.git` directory (defaults to current directory)
- `-limit` (default `1000`): number of commits to load per batch

### Development

```bash
go test ./...
go build ./...
```

Key packages:

- `cmd`: CLI parsing and entry point
- `internal/git`: repository access, commit scanning, graph building
- `internal/gui`: Tk UI and controller logic

See `AGENTS.md` for guidelines followed by the automation helping maintain this project.
