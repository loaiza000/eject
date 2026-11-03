# Compose compatibility assessment

`eject plan --input infrastructure.json --target compose --report compatibility` emits a JSON report with `schema_version: "1"`, `target: "compose"` and sorted `results`. Each result has a stable rule `code`, source `resource`, narrow `capability`, `status`, `severity`, explanation, evidence, recommendation and target. A `SUPPORTED` result applies to that capability only; it never declares a whole workload equivalent.

Statuses are `SUPPORTED`, `PARTIALLY_SUPPORTED`, `MANUAL_ACTION_REQUIRED`, `UNSUPPORTED` and `UNKNOWN`. Severity is `info`, `warning` or `blocker`; blockers appear in the plan. `UNKNOWN` preserves missing evidence instead of guessing. Rule codes are stable within schema v1. A future incompatible reinterpretation requires a schema version change.

| Source evidence | Rule behavior |
| --- | --- |
| ECS container image | A reference is partially supported until pull access and runtime behavior are checked. A private ECR reference requires manual registry access or copying. Missing image is a blocker. |
| ECS container port | A valid TCP/UDP container port has a Compose representation. The plan does not publish it publicly. Invalid ports block mapping. |
| ECS environment and secret names | Their values or source secret references are absent from the IR. Supplying values securely is a manual action. |
| ECS task and service | Container structure can inform a Compose proposal. Scheduling, scaling, roles, logs and `awsvpc` behavior require review. Missing confirmed task/container relationships block reliable mapping. |
| RDS PostgreSQL | The engine family can map to a PostgreSQL service, but exact version, extensions and configuration need review. The generator currently renders only PostgreSQL major version 16 with the Alpine image layout; other versions are blocked. Database contents require a separate migration. Other engines are unsupported. |
| S3 bucket | No S3-compatible Compose backend is implemented. Objects, history, IAM, policies and encryption need separate assessment. No S3 target step is produced. |
| Unknown resource or capability | Reported as `UNKNOWN` with a recommendation to assess it. |
| Missing dependency target | Reported as `MANUAL_ACTION_REQUIRED` and a blocker. An unconfirmed relation remains `UNKNOWN`. |

The rules use only the versioned IR and do not contact AWS, Docker or registries. They cannot prove image availability, application behavior or data equivalence. The synthetic example's reviewed [compatibility report](../tests/golden/compatibility.json) shows the actual JSON shape.
