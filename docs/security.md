# Security

`inspect` reads local files capped at 1 MiB and 64 JSON nesting levels each. It opens files relative to an `os.Root` directory handle and rejects traversal and symlinked input paths. The manifest parser rejects unknown fields and versions. AWS response parsers read only selected fields; ECS environment values and secret references are discarded. Output writes use restrictive permissions and refuse existing files unless `--force` is provided. EJECT does not execute Docker or AWS commands and performs no network access or telemetry. `doctor` only searches `PATH` for a Docker executable.

Input complexity is bounded by file size, nesting depth, and manifest entry count. New output uses a same-directory temporary file and a hard link that fails if a destination appears concurrently; a filesystem that cannot create hard links returns an error. With `--force`, EJECT syncs the temporary file and renames it over an existing regular file. This avoids deleting the old file first. Go documents that rename is not guaranteed atomic on non-Unix platforms, including Windows; filesystem and power-loss behavior also vary. Runtime verification remains unimplemented.

Do not put credentials in issue reports or example snapshots.
