# Architecture

The entry point `cmd/eject` translates process signals and errors into exit status. `internal/cli` constructs Cobra commands and writes user output. `internal/discovery/aws` validates a versioned manifest and parses selected exported AWS CLI responses. `internal/domain` defines the neutral, versioned inventory. Command construction is isolated to make it testable without global mutable state.

The future offline flow is manifest to provider parser to versioned neutral infrastructure model to dependency and compatibility analysis to plan to target generator to verification. These packages will appear only as they gain real behavior. AWS is intended to be an input adapter and Compose an output adapter. Neither is the domain model.

`inspect` records only dependencies directly present in snapshots. It deliberately does not infer connections from environment variable names or endpoint similarity. No network or cloud account is needed for current commands. `doctor` checks executable lookup only.
