## Automation Notes

This project occasionally uses AI assistance. To keep contributions consistent,
agents should follow these rules:

1. Prefer pure-Go solutions. Do not shell out to external git commands or other tools unless explicitly approved.
2. Keep UI logic testable. Add helpers in `internal/gui` when possible and cover them with unit tests.
3. Treat `cmd` and `internal/gui` as UI layers (minimal logic). Core behavior belongs in `internal/git`.
4. Use `go test ./...` and `go fmt` before sending changes whenever the environment allows it.
5. README and AGENTS should stay ASCII-only and concise.
6. Describe any sandbox or permission issues observed while running commands so maintainers can reproduce locally.
