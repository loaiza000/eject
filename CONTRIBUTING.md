# Contributing

Install Go 1.27.3 and clone the repository. Run `go mod download`, `go build ./...`, `go test ./...`, `go test -race ./...`, and `go vet ./...`. Format Go code with `gofmt`.

The small entry point is in `cmd/eject`; commands and their tests are in `internal/cli`; AWS snapshot parsing is in `internal/discovery/aws`; versioned models are in `internal/domain`; target rules and planning live in `internal/compatibility` and `internal/planner`; Compose rendering is in `internal/generator`; verification is in `internal/verification`. Keep provider parsing separate from the neutral domain and destination generation. Add behavior tests beside each Go package. The end-to-end fixtures and reviewed golden output live under `tests/`.

Open an issue to discuss a significant new provider, target, or public format. Pull requests should explain behavior, tests, security impact, and documentation changes. Use Conventional Commits (`feat:`, `fix:`, `docs:`, etc.). Do not add real cloud credentials or customer snapshots.

Version public JSON formats. Incompatible changes require a documented migration path and an explicit version increment. Do not silently reinterpret existing fields.

To add compatibility, define a stable rule code, capability, status, severity, evidence and recommendation. Add tests for supported and missing evidence, then review changes to golden reports. A Compose representation of one capability must not be described as full workload equivalence.
