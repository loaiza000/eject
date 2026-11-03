# Generated Compose proposal

This directory is a local configuration proposal from EJECT. Read `limitations.json` before running anything. A PARTIAL or BLOCKED status means source behavior is not fully represented.

Copy `.env.example` to a private `.env` file and supply required values. Do not commit `.env` or put credentials in `compose.yaml`. EJECT has not copied data, images or secrets, and has not changed AWS resources.

`compose.yaml` contains only services with supported configuration evidence. Ports are internal to the local bridge network. The PostgreSQL service, when present, starts with an empty named volume and a local health check; it does not contain RDS data. Verify image access, architecture, application behavior, networking and version compatibility independently.
