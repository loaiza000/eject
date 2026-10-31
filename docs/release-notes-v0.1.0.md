# EJECT v0.1.0

Initial open-source preview of the local EJECT CLI.

- Inspect selected AWS CLI JSON snapshots for ECS, RDS PostgreSQL, and S3 metadata.
- Assess explicit Compose compatibility rules and produce versioned migration plans.
- Generate guarded local Compose proposals for eligible ECS containers and empty PostgreSQL 16 services.
- Verify generated file integrity and Compose syntax; optionally start an isolated Docker project with separate start and pull authorization.
- Run a synthetic HTTP/PostgreSQL demo with health, SQL, and HTTP checks.

EJECT does **not** migrate real data, copy secrets, modify AWS resources, map S3 to a verified local backend, or guarantee functional equivalence with AWS. Generated projects may be `PARTIAL` or `BLOCKED`. Runtime verification is opt-in and retains persistent volumes. Read `limitations.json` and the documentation before running generated services.

Supported binaries and SHA-256 checksums will be attached only after the release checklist and maintainer approval are complete.
