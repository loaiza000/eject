# EJECT v0.1.0 (experimental preview)

First public release of EJECT, a free, local, offline-first CLI that reads exported AWS configuration, assesses what can run on Docker Compose, generates a reviewed local proposal, and verifies it.

## What's included

- `inspect`: offline parsing of AWS CLI JSON for ECS task definitions and services, RDS instances, and S3 versioning and encryption, producing a versioned, provider-neutral inventory. Only dependencies proven by the exports are recorded.
- `plan`: explicit, evidence-based Compose compatibility rules and a deterministic, versioned migration plan with blockers and manual actions.
- `generate`: Docker Compose proposals for eligible ECS containers and **empty** PostgreSQL 16 services, with `limitations.json`, `.env.example`, the embedded plan, and a hashed completion marker. Output files are private (Unix `0600`/`0700`, protected Windows ACLs), and existing directories are never overwritten.
- `verify`: detection of edited, missing, extra, or incomplete files and unsafe permissions, plus Compose syntax validation. Opt-in runtime checks (`--runtime --allow-start`) start an isolated Compose project and check health, PostgreSQL `pg_isready` and `SELECT 1`, and HTTP endpoints, without pulling images unless `--allow-pull` is given and without deleting volumes.
- A synthetic HTTP + PostgreSQL demo that runs end to end.

## Limitations (please read)

- No data migration. Generated PostgreSQL services start empty.
- No functional equivalence with AWS. ECS scheduling, scaling, `awsvpc`, load balancing, IAM, and KMS are not reproduced, and network and IAM policies need manual review.
- Partial ECS coverage: health checks, CPU and memory, logging, volumes, and sidecar semantics are not captured.
- No S3 backend mapping.
- Real workloads usually produce `PARTIAL` output. `limitations.json` explains why.

## Platform verification

| Platform | Evidence |
| --- | --- |
| Windows 11 amd64 | Native build, tests, ACL tests, CLI scenarios, Docker runtime demo, extracted release binary executed |
| Linux amd64 (WSL2 Ubuntu) | Native build, tests, race detector, fuzzing, govulncheck, Unix permission checks, extracted release binary executed |
| Linux arm64, Windows arm64, macOS amd64/arm64 | Cross-compiled and packaged only. **Not executed natively.** |

GitHub Actions CI is configured, but it could not run for this release because of an account-level billing lock. All evidence above comes from local runs, not CI. Reports from macOS and arm64 users are welcome.

## Install

Download the archive for your platform and `SHA256SUMS`, verify the checksum, extract, and run `eject version`. Step-by-step commands for each OS are in [docs/installation.md](https://github.com/loaiza000/eject/blob/v0.1.0/docs/installation.md).

The binaries are not code-signed or notarized.
