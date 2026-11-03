# Changelog

All notable changes to this project will be documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Initial CLI foundation with `version` and `doctor`.
- Offline `inspect` command for selected AWS CLI JSON responses.
- Compose compatibility assessment and versioned `plan` output, without migration execution.
- Version 2 plans with embedded source inventory and `generate` for local Compose proposals, YAML validation, and limitations reports.
- Versioned generation marker and `verify` with offline integrity checks, opt-in isolated Docker runtime checks, and a synthetic HTTP/PostgreSQL demo.

### Fixed

- Reject symlinked parent directories for JSON output and symlinked destinations with `--force`.
- Preserve executable modes for Unix binaries inside release archives.
- Reject permissive Unix generated artifacts and refuse writes when a mounted filesystem cannot enforce private modes.
