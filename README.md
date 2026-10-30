# EJECT

**Your infrastructure. Your rules. Eject anywhere.**

EJECT is a free, open-source, local cloud portability CLI. It inspects selected AWS CLI JSON snapshots and produces a versioned, provider-neutral JSON inventory with evidenced dependencies. It does **not** yet assess compatibility, plan migrations, generate Compose files, or verify deployments.

## Install and run from source

Requires Go 1.27.3. The module path is `github.com/loaiza000/eject`.

```sh
go build -o eject ./cmd/eject
./eject --help
./eject version --json
./eject doctor --json
./eject inspect --manifest examples/aws-basic/manifest.json --output .eject/infrastructure.json
```

On Windows, build with `go build -o eject.exe ./cmd/eject` and run `./eject.exe`.

`doctor` reports the Go runtime, platform, and whether a Docker executable is on `PATH`. It does not contact Docker, AWS, or the network. Finding Docker does not prove Docker Compose works.

## Available commands

| Command | Behavior |
| --- | --- |
| `eject version` | Print the build version. `--json` is available. |
| `eject doctor` | Report local environment capabilities. `--json` is available. |
| `eject inspect` | Parse a versioned manifest and selected AWS CLI JSON responses. JSON goes to stdout by default; `--output` writes a file and `--force` allows replacement. |

`--help` and `--version` are available. The default version is `dev` until a release build injects a version. Exit code 0 means success, 1 means a command error, 2 means invalid inspection input, and 3 means output write failure. Diagnostics go to stderr. JSON command output goes to stdout.

The synthetic example produces five resources (ECS service, task, container, RDS PostgreSQL instance, and S3 bucket) and two confirmed dependencies (service to task and task to container). The exported data does not establish a connection between the container and RDS, so EJECT does not claim one. The S3 bucket is inventoried, not portable deployment output. See [configuration](docs/configuration.md).

For example, `eject version --json` prints `{"version":"dev"}` in a source build. `inspect` writes the full versioned inventory to `.eject/infrastructure.json` in the command above.

## Architecture and limits

The executable entry point handles signals and exit status. `internal/cli` defines commands, `internal/discovery/aws` parses exported AWS CLI responses, and `internal/domain` holds the neutral inventory. Compatibility, planning, generation, and verification packages will be added when their behavior exists. See [architecture](docs/architecture.md), [security](docs/security.md), and [roadmap](ROADMAP.md).

EJECT does not move data or modify cloud resources. Future optional cloud operations may incur provider charges; EJECT itself is free. There is no account, telemetry, hosted service, or AI API.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). The project is licensed under [Apache License 2.0](LICENSE).
