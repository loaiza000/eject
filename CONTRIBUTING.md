# Contributing

Install Go 1.27.3 and clone the repository. Run `go mod download`, `go build ./...`, `go test ./...`, `go test -race ./...`, and `go vet ./...`. Format Go code with `gofmt`.

The small entry point is in `cmd/eject`; commands and their tests are in `internal/cli`; AWS snapshot parsing is in `internal/discovery/aws`; the neutral inventory is in `internal/domain`. As new features become real, keep provider parsing separate from the neutral domain and destination generation. Add tests beside each Go package. Put end-to-end fixtures under `tests/` only when needed.

Open an issue to discuss a significant new provider, target, or public format. Pull requests should explain behavior, tests, security impact, and documentation changes. Use Conventional Commits (`feat:`, `fix:`, `docs:`, etc.). Do not add real cloud credentials or customer snapshots.

Version public JSON formats. Incompatible changes require a documented migration path and an explicit version increment. Do not silently reinterpret existing fields.
