## gitk-go

`gitk-go` is a lightweight Git history explorer written in Go. It recreates
much of the `gitk` experience using the
[`modernc.org/tk9.0`](https://pkg.go.dev/modernc.org/tk9.0) bindings so it runs
without Tcl scripts or external runtimes.

### Features

- Tk-based UI with commit list, diff viewer, and status bar
- Uses `go-git`(https://github.com/go-git/go-git) to read repositories without
  shelling out to git
- Real-time filtering across hashes, authors, emails, and messages
- Batching logic keeps memory usage stable while browsing large repos
- Explicit reload and load-more controls when you want to refresh history

### Running

```bash
go run . -repo /path/to/repo -limit 200
```

Flags:

- `-repo` (default `.`): repository root or `.git` directory
- `-limit` (default `200`): number of commits per batch
