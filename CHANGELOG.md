# Changelog

All notable changes to this project will be documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - Unreleased

First public preview. The date is set when the release is published.

### Added

- CLI foundation with `version` and `doctor`.
- Offline `inspect` command for selected AWS CLI JSON responses (ECS, RDS, S3 metadata).
- Compose compatibility assessment and versioned `plan` output, without migration execution.
- Version 2 plans with embedded source inventory, and `generate` for local Compose proposals, YAML validation, and limitations reports.
- Versioned generation marker and `verify` with offline integrity checks, opt-in isolated Docker runtime checks, and a synthetic HTTP/PostgreSQL demo.
- Release packaging for Linux, macOS, and Windows on amd64 and arm64, with license texts and `SHA256SUMS`.

### Fixed

- Reject symlinked parent directories for JSON output and symlinked destinations with `--force`.
- Preserve executable modes for Unix binaries inside release archives.
- Reject permissive Unix generated artifacts, and refuse writes when a mounted filesystem cannot enforce private modes.
- Runtime verification reads the single-array `docker compose ps --format json` output of Compose releases before 2.21, in addition to the newline-delimited format.
- Docker commands receive `DOCKER_HOST`, `DOCKER_CONTEXT`, `DOCKER_TLS_VERIFY`, and `DOCKER_CERT_PATH`, so rootless Docker and non-default contexts work. Interpolation-sensitive variables are still withheld.
- Runtime verification resolves the Compose project before starting containers. A missing `.env` value now reports `RUNTIME_CONFIG` instead of a misleading cleanup failure.
- `inspect` records ECS port mappings without `protocol` as `tcp` (the ECS default) instead of producing an inventory that `plan` rejects. Unknown protocols are rejected with a clear error.

### Changed

- Release archives contain the executable as `eject` (`eject.exe` on Windows), so it runs without renaming.

[Unreleased]: https://github.com/loaiza000/eject/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/loaiza000/eject/releases/tag/v0.1.0
