# Migration proposal

`eject plan --input infrastructure.json --target compose` emits a plan JSON with `schema_version: "2"`. `--output path` writes it to a file; `--force` permits replacing an existing regular file. `--report compatibility` emits the standalone version 1 assessment instead. Both outputs are deterministic for the same input and engine version. The CLI writes JSON to stdout and diagnostics to stderr.

The plan embeds the version 1 source inventory, lists source resource IDs, embeds the versioned compatibility assessment, and records proposed `steps`, `blockers`, `manual_actions`, `required_variables`, `limitations` and `recommended_checks`. Each step has a stable ID, source resources, proposed target resource, description, prerequisite step IDs, preconditions and expected status. The status describes a proposal or review, not execution. The embedded inventory contains image references and metadata but no ECS environment values or secret references from `inspect`.

For the synthetic example, EJECT proposes a Compose service definition for the container image, an **empty** PostgreSQL service definition, and a review step for the confirmed ECS service → task → container relationship. The review step depends on the container proposal. It does not infer a connection from the container to RDS merely because an environment variable is named `DATABASE_HOST`. It emits no S3 target step.

Blockers include the missing S3 backend, absent S3 objects and permissions, missing ECS secret mapping, and unimplemented database data transfer. The [reviewed example plan](../tests/golden/plan.json) is a reproducible output, not an executable Compose project.

The planner writes reports only. The separate `generate` command renders supported configuration from a version 2 plan after recalculating every rule and step. Version 1 plans remain historical reports and must be regenerated with `plan` before use by `generate`. Neither command pulls images, starts containers, moves data, changes AWS resources, performs rollback or verifies a deployment.
