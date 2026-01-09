## Automation Notes

This project occasionally uses AI assistance. To keep contributions consistent,
agents should follow these rules:

1. Keep UI logic testable. Add helpers in `internal/gui` when possible and cover them with unit tests.
2. Treat `cmd` and `internal/gui` as UI layers (minimal logic). Core behavior belongs in `internal/git`.
3. Use `go test ./...`, `go fmt`, and `go vet ./...` before sending changes whenever the environment allows it.
4. README and AGENTS should stay ASCII-only and concise.
5. Keep lines under 120 characters (enforced by golangci-lint).
6. If a function returns multiple values of the same type, use named return values.
7. Dot imports are only allowed for `modernc.org/tk9.0`.
8. Describe any sandbox or permission issues observed while running commands so maintainers can reproduce locally, but ignore `nice(5) failed: operation not permitted`.
9. Keep GUI assets (icons, images, etc.) inside `internal/gui/assets` and reference them there when using `go:embed`.
10. Group related controller/state fields into dedicated structs instead of leaving long flat structs.
11. When working with mutexes, prefer `defer mu.Unlock()` immediately after locking unless there is a strong reason not to.
12. Prefer `slog/log` instead of `log` for logging.
13. Prefer tests co-located with the implementation (e.g. `internal/gui/filter_test.go` for `filter.go`) instead of creating per-feature test filenames.
14. Always run tests with `GOCACHE` set to avoid sandbox issues: `env GOCACHE="$PWD/.gocache" go test ./...`.
