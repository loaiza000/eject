# CLI reference

All commands run locally. `inspect`, `plan`, and static `verify` do not contact AWS. Docker Compose `config` may run locally during generation and static verification when available. Only `verify --runtime --allow-start` starts containers.

| Command | Required options | Other options | Output |
| --- | --- | --- | --- |
| `eject version` | None | `--json` | Version (`dev` for source builds). |
| `eject doctor` | None | `--json` | Go platform and Docker executable lookup; no daemon probe. |
| `eject inspect` | `--manifest FILE` | `--output FILE`, `--force` | Version 1 infrastructure JSON. |
| `eject plan` | `--input FILE` | `--target compose`, `--report plan\|compatibility`, `--output FILE`, `--force` | Version 2 plan or version 1 compatibility report. |
| `eject generate` | `--plan FILE`, `--output DIR` | None | Compose proposal, embedded plan, and completion marker. |
| `eject verify` | `--project DIR` | `--runtime`, `--allow-start`, `--allow-pull`, repeatable `--http-check SERVICE:PORT:/PATH` | Version 1 verification JSON on stdout. |

Exit status 0 means the command succeeded; verification may still include `SKIPPED` or `NOT_VERIFIED` checks. Status 1 is an unclassified command error, 2 is invalid input, 3 is an output write failure, and 4 means a verification check failed. Diagnostics go to stderr; JSON reports go to stdout. A static `PASSED` result establishes file and syntax consistency only. The generation status may still be `PARTIAL` or `BLOCKED`.

`generate` rejects historical version 1 plans because they lack source data. Rerun `plan` against the version 1 infrastructure JSON. Output directories must be new or byte identical to an existing generation; there is no generation `--force`. A failed generation may leave a directory without a completion marker. Inspect and remove it manually before retrying.

Runtime checks require explicit `--allow-start`; image downloads require the separate `--allow-pull` flag. Without configured HTTP checks, the HTTP check is `SKIPPED`. See [verification](verification.md), [generation](generation.md), and the [demo](../examples/demo/README.md).
