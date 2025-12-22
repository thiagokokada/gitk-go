## Automation Notes

This project occasionally uses AI assistance. To keep contributions consistent,
agents should follow these rules:

1. Prefer pure-Go solutions. Do not shell out to external git commands or other tools unless explicitly approved.
2. Keep UI logic testable. Add helpers in `internal/gui` when possible and cover them with unit tests.
3. Treat `cmd` and `internal/gui` as UI layers (minimal logic). Core behavior belongs in `internal/git`.
4. Use `go test ./...`, `go fmt`, and `go vet ./...` before sending changes whenever the environment allows it.
5. README and AGENTS should stay ASCII-only and concise.
6. Describe any sandbox or permission issues observed while running commands so maintainers can reproduce locally.
7. Keep GUI assets (icons, images, etc.) inside `internal/gui/assets` and reference them there when using `go:embed`.
8. Group related controller/state fields into dedicated structs instead of leaving long flat structs.
9. When working with mutexes, prefer `defer mu.Unlock()` immediately after locking unless there is a strong reason not to.
10. Prefer `slog/log` instead of `log` for logging.
11. Prefer tests co-located with the implementation (e.g. `internal/gui/filter_test.go` for `filter.go`) instead of creating per-feature test filenames.
12. Always run tests with `GOCACHE` set to avoid sandbox issues: `env GOCACHE="$PWD/.gocache" go test ./...`.
