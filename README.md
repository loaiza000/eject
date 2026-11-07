# EJECT

**Your infrastructure. Your rules. Eject anywhere.**

EJECT is a free, open-source command-line tool that reads exported AWS configuration, tells you honestly what could run locally, and generates a Docker Compose proposal for the parts that can, with a report of everything it could not represent.

It runs entirely on your machine. There is no account, server, telemetry, subscription, or AI service. EJECT never contacts AWS. You export the data yourself, and EJECT works from those files.

> **Status: v0.1.0 preview.** The core flow works and is tested, but the AWS coverage is intentionally narrow (see [What it does not do](#what-it-does-not-do)). Treat generated output as a reviewed starting point, not a finished migration.

## Why

Teams that want to run a cloud workload locally, for development, testing, cost control, or to reduce lock-in, usually have to reverse-engineer it by hand. Generic converters tend to guess, and the guesses look authoritative. EJECT does the opposite. Every conclusion is tied to evidence in your exported files. Anything it cannot establish is reported as unknown, blocked, or a manual action rather than silently papered over.

## What it does

```
AWS CLI JSON exports ──► inspect ──► plan ──► generate ──► verify
                          │           │          │            │
                     neutral      compat-    Compose      integrity, syntax,
                     inventory    ibility    proposal     optional local
                     (JSON)       + plan     + limits     runtime checks
```

| Command | What it does |
| --- | --- |
| `eject inspect` | Reads a versioned manifest of AWS CLI JSON responses and builds a provider-neutral inventory. Records only dependencies that the exports prove. |
| `eject plan` | Applies explicit compatibility rules for a Docker Compose target and writes a versioned, unexecuted plan with blockers and manual actions. |
| `eject generate` | Re-validates the plan and writes `compose.yaml`, `.env.example`, `limitations.json`, a README, the embedded plan, and a hashed completion marker. Never overwrites a different existing directory. |
| `eject verify` | Detects modified, missing, extra, or incomplete files and unsafe permissions. With explicit `--runtime --allow-start`, it starts an isolated local Compose project and checks health, PostgreSQL (`pg_isready`, `SELECT 1`), and optional HTTP endpoints. |
| `eject doctor` | Reports version, platform, and whether a Docker executable is on `PATH`. |
| `eject version` | Prints the version. |

Supported input today:

| AWS source | EJECT result |
| --- | --- |
| `ecs describe-task-definition`, `ecs describe-services` | Compose services for container images and internal ports; environment variable *names* become required local placeholders. Values and secret references are discarded at inspection. |
| `rds describe-db-instances` (PostgreSQL 16) | An **empty** local `postgres:16-alpine` service with a named volume and health check. Other engines and versions are reported as blocked. |
| `s3api get-bucket-versioning`, `get-bucket-encryption` | Assessed and reported. No local S3 backend is generated. |

## What it does not do

- It does **not** migrate data. A generated PostgreSQL service starts empty.
- It does **not** claim functional equivalence with AWS. ECS scheduling, scaling, `awsvpc` networking, load balancing, IAM, and KMS are not reproduced.
- It does **not** map S3 to a local backend.
- It does **not** copy secrets or environment values. You supply them in a private `.env`.
- It does **not** pull images or start containers unless you pass explicit flags.
- It does **not** modify any cloud resource, and it never calls AWS.

Generated projects are therefore usually `PARTIAL`. That is the honest answer for real ECS/RDS workloads, and `limitations.json` lists exactly why.

## Install

### Prebuilt binaries (GitHub Releases)

Each release provides `eject_<version>_<os>_<arch>.tar.gz` archives for Linux, macOS, and Windows on `amd64` and `arm64`, plus a `SHA256SUMS` file. Each archive contains the `eject` executable (`eject.exe` on Windows), the Apache-2.0 license, and third-party notices.

**Linux / macOS** (replace `linux_amd64` with `linux_arm64`, `darwin_amd64` or `darwin_arm64`):

```sh
VERSION=v0.1.0
ASSET=eject_${VERSION}_linux_amd64.tar.gz
curl -fLO https://github.com/loaiza000/eject/releases/download/$VERSION/$ASSET
curl -fLO https://github.com/loaiza000/eject/releases/download/$VERSION/SHA256SUMS
grep " $ASSET\$" SHA256SUMS | sha256sum -c -   # macOS: use `shasum -a 256 -c -`
tar -xzf "$ASSET" eject
./eject version
```

Optionally move `eject` into a directory on your `PATH`, such as `~/.local/bin`.

**Windows (PowerShell)** (use `windows_arm64` on ARM devices):

```powershell
$Version = "v0.1.0"
$Asset = "eject_${Version}_windows_amd64.tar.gz"
Invoke-WebRequest "https://github.com/loaiza000/eject/releases/download/$Version/$Asset" -OutFile $Asset
Invoke-WebRequest "https://github.com/loaiza000/eject/releases/download/$Version/SHA256SUMS" -OutFile SHA256SUMS
$Expected = ((Select-String -Path SHA256SUMS -Pattern " $([regex]::Escape($Asset))$").Line -split '\s+')[0]
if ((Get-FileHash $Asset -Algorithm SHA256).Hash -ne $Expected.ToUpper()) { throw "checksum mismatch" }
tar -xzf $Asset eject.exe
.\eject.exe version
```

See the [installation guide](docs/installation.md) for platform notes (macOS Gatekeeper, Windows SmartScreen, WSL) and troubleshooting.

### From source

Requires Go 1.27.3.

```sh
git clone https://github.com/loaiza000/eject.git
cd eject
go build -o eject ./cmd/eject        # Windows: go build -o eject.exe ./cmd/eject
./eject version                      # prints "eject dev" for source builds
```

## Quick start (offline, about one minute)

The repository includes a synthetic AWS export with an ECS service, task, and container, an RDS PostgreSQL 16 instance, and an S3 bucket. Run it from the repository root:

```sh
./eject inspect  --manifest examples/aws-basic/manifest.json --output out/infrastructure.json
./eject plan     --input out/infrastructure.json --output out/plan.json
./eject generate --plan out/plan.json --output out/project
./eject verify   --project out/project
```

What you should see:

- `inspect` finds 5 resources and 2 confirmed dependencies (service → task → container). It does **not** invent a container → database link just because a variable is named `DATABASE_HOST`.
- `plan` records 13 rule results, 3 proposed steps, 4 blockers, and 7 manual actions.
- `generate` prints `PARTIAL: out/project`. The project contains an empty PostgreSQL service. The ECS container is excluded because its secret has no safe local mapping, and the S3 bucket is excluded because no backend is implemented.
- `verify` prints a JSON report with `"outcome": "PASSED"`, meaning files match the plan and permissions are private. It does not mean the application works.

Open `out/project/limitations.json` to see every blocker and manual action.

### Runtime demo with Docker (optional)

[`examples/demo`](examples/demo/README.md) is a synthetic HTTP service plus PostgreSQL 16 that runs end to end. It builds a small local image from `scratch` (no base image download) and needs `postgres:16-alpine` available locally, or an explicit `--allow-pull`. Then run:

```sh
./eject verify --project examples/demo/output/project --runtime --allow-start \
  --http-check backend-0c64dac5bad49d23:8080:/health
```

EJECT starts a uniquely named Compose project, waits for health checks, runs `pg_isready` and `SELECT 1`, probes the HTTP endpoint through a temporary loopback-only port, and stops only its own project. Named volumes are kept, never deleted.

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Success. A verification report may still contain `SKIPPED` or `NOT_VERIFIED` checks. |
| 1 | Unclassified command error (for example, an unknown command). |
| 2 | Invalid input: missing flag, malformed or unsupported file, tampered plan. |
| 3 | Output could not be written safely (existing file without `--force`, symlink, unsafe permissions). |
| 4 | Verification found a failed check. See the JSON report on stdout. |

Diagnostics go to stderr. JSON goes to stdout.

## Security model

- **Inputs are bounded.** Input files are capped (1 MiB per AWS export, 8 MiB inventory, 32 MiB plan) and limited to 64 JSON nesting levels. Parsing rejects unknown fields, trailing data, path traversal, and symlinked paths.
- **Outputs are private.** On Unix, generated files are `0600` in a `0700` directory, and EJECT refuses to write where the filesystem cannot enforce that (for example, Windows drives mounted in WSL). On Windows, outputs get a protected ACL limited to the current user, SYSTEM, and Administrators. `verify` fails on existing projects or `.env` files that grant broader access.
- **Nothing is overwritten by surprise.** JSON outputs require `--force` to replace a file. `generate` only accepts a new directory or a byte-identical previous generation.
- **Tampering is detected.** `generate` and `verify` recompute the plan from its embedded source. The completion marker detects interrupted or edited generations. These hashes are unkeyed consistency checks, not signatures.
- **Docker is opt-in and constrained.** EJECT runs fixed `docker compose` subcommands with an allow-listed environment, never `down -v`, uses `--pull never` unless `--allow-pull` is given, and bounds runtime checks with timeouts. Starting a container runs that image's code, so only verify images you trust.

Read [docs/security.md](docs/security.md) for the details and known limits. Report vulnerabilities as described in [SECURITY.md](SECURITY.md).

## Platform support

| Platform | Binary | Verification for v0.1.0 |
| --- | --- | --- |
| Windows amd64 | ✓ | Native tests, ACL tests, CLI and Docker runtime demo on Windows 11 |
| Linux amd64 | ✓ | Native tests and race detector on WSL2 Ubuntu, Unix permission checks, extracted binary executed |
| Linux arm64 | ✓ | Cross-compiled and packaged; not executed natively |
| Windows arm64 | ✓ | Cross-compiled and packaged; not executed natively |
| macOS amd64 / arm64 | ✓ | Cross-compiled and packaged; **not tested on macOS** |

GitHub Actions CI (Linux, Windows, and macOS matrix, fuzzing, Docker integration) is configured but has not run for this release. Verification so far is local. If you run EJECT on an untested platform, a report in an issue is very welcome.

## Documentation

- [Installation](docs/installation.md) · [CLI reference](docs/cli.md) · [Input manifest](docs/configuration.md)
- [Architecture](docs/architecture.md) · [Compatibility rules](docs/compatibility.md) · [Plan format](docs/planning.md) · [Generation](docs/generation.md)
- [Verification](docs/verification.md) · [Security](docs/security.md) · [Development](docs/development.md) · [Release procedure](docs/releasing.md)

## Architecture

```
cmd/eject                  entry point: signals and exit codes
internal/cli               Cobra commands, safe file input and output
internal/discovery/aws     manifest and AWS CLI JSON parsers (input adapter)
internal/domain            versioned, provider-neutral models
internal/compatibility     explicit Compose rules with evidence
internal/planner           deterministic, unexecuted plans
internal/generator         Compose rendering and completion marker
internal/verification      static integrity and opt-in runtime checks
internal/securefs          private files: Unix modes, Windows ACLs
```

AWS is an input adapter and Compose is a target evaluated by rules. Neither is the domain model. All outputs are deterministic for the same input. There are three direct dependencies: Cobra, `gopkg.in/yaml.v3`, and `golang.org/x/sys`.

## Roadmap

v0.1.0 completes the planned foundation (inspect, plan, generate, verify). Future work, such as broader AWS coverage, more targets, or data migration, requires separate design and validation. See [ROADMAP.md](ROADMAP.md).

## Contributing

Contributions are welcome. Read [CONTRIBUTING.md](CONTRIBUTING.md) and the [Code of Conduct](CODE_OF_CONDUCT.md). Please never include real credentials or customer exports in issues or pull requests.

## License

[Apache License 2.0](LICENSE). Third-party licenses are listed in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
