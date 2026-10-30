# Development

Use Go 1.27.3. Run `go mod download`, `go build ./...`, `go test ./...`, and `go vet ./...`. Check formatting with `go fmt ./...` (it should print nothing). Run `go test -race ./...` where CGO and a C compiler are available; CI runs it on Linux. Run `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...` for a vulnerability scan. That scan queries the public Go vulnerability database; the EJECT CLI itself needs no network or AWS account.

The sole direct runtime dependency is Cobra v1.10.2 (Apache-2.0) for command parsing and help. Its transitive modules are locked in `go.sum`. The Go standard library handles JSON, file parsing, environment checks, signals, and errors.
