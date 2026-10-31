# EJECT

**Your infrastructure. Your rules. Eject anywhere.**

EJECT is a free, open-source, local cloud portability CLI. It inspects selected AWS CLI JSON snapshots, assesses a proposed Docker Compose target, writes a versioned migration plan, generates a local Compose proposal, and verifies its files. Optional runtime checks can start an isolated local Docker Compose project with explicit authorization. EJECT does **not** migrate data, modify cloud infrastructure, or prove application equivalence.

## Install and run from source

Requires Go 1.27.3. The module path is `github.com/loaiza000/eject`.

```sh
go build -o eject ./cmd/eject
./eject --help
./eject version --json
./eject doctor --json
./eject inspect --manifest examples/aws-basic/manifest.json --output .eject/infrastructure.json
./eject plan --input .eject/infrastructure.json --target compose --output .eject/plan.json
./eject plan --input .eject/infrastructure.json --target compose --report compatibility --output .eject/compatibility.json
./eject generate --plan .eject/plan.json --output ./output
./eject verify --project ./output
```

On Windows, build with `go build -o eject.exe ./cmd/eject` and run `./eject.exe`.

Generated files use restricted Unix modes or protected Windows ACLs. Windows verification rejects existing projects and private `.env` files that grant access beyond the current user, LocalSystem and Administrators; see [security](docs/security.md).

`doctor` reports the Go runtime, platform, and whether a Docker executable is on `PATH`. It does not contact Docker, AWS, or the network. Finding Docker does not prove Docker Compose works.

## Available commands

| Command | Behavior |
| --- | --- |
| `eject version` | Print the build version. `--json` is available. |
| `eject doctor` | Report local environment capabilities. `--json` is available. |
| `eject inspect` | Parse a versioned manifest and selected AWS CLI JSON responses. JSON goes to stdout by default; `--output` writes a file and `--force` allows replacement. |
| `eject plan` | Assess Compose compatibility and produce a JSON migration proposal. `--report compatibility` prints the standalone assessment; `--output` and `--force` work as with `inspect`. |
| `eject generate` | Recheck a version 2 plan and write a local Compose proposal. Existing output is accepted only when byte identical. |
| `eject verify` | Compare generated files with their embedded plan and completion marker. `--runtime --allow-start` opts into local Docker checks; `--allow-pull` separately permits missing image downloads. |

`--help` and `--version` are available. The default version is `dev` until a release build injects a version. Exit code 0 means success, 1 means a command error, 2 means invalid input, 3 means output write failure, and 4 means verification failure. Diagnostics go to stderr. JSON command output goes to stdout.

The synthetic example produces five resources (ECS service, task, container, RDS PostgreSQL instance, and S3 bucket) and two confirmed dependencies (service to task and task to container). The exported data does not establish a connection between the container and RDS, so EJECT does not claim one. The generated project is **PARTIAL**: it contains an empty local PostgreSQL service and excludes the ECS container with an unresolved secret and the S3 bucket. See [configuration](docs/configuration.md) and [generation](docs/generation.md).

For example, `eject version --json` prints `{"version":"dev"}` in a source build. The synthetic example's plan contains 13 rule results, three proposed steps, four blockers and seven manual actions. These are proposal records, not executed work. See the [CLI reference](docs/cli.md), [compatibility rules](docs/compatibility.md), and [plan format](docs/planning.md).

## Architecture and limits

The executable entry point handles signals and exit status. `internal/cli` defines commands, `internal/discovery/aws` parses exported AWS CLI responses, `internal/domain` holds versioned models, `internal/compatibility` evaluates rules, `internal/planner` builds a proposal, `internal/generator` renders supported configuration, and `internal/verification` checks generated files and optional local runtime behavior. See [verification](docs/verification.md), the [synthetic HTTP demo](examples/demo/README.md), [architecture](docs/architecture.md), [security](docs/security.md), [release procedure](docs/releasing.md), and [roadmap](ROADMAP.md).

EJECT does not move data or modify cloud resources. There is no account, telemetry, hosted service, or AI API.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). The project is licensed under [Apache License 2.0](LICENSE).
