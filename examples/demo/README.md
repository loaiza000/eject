# Synthetic HTTP + PostgreSQL demo

This example is entirely fictitious. The AWS CLI JSON files describe an ECS container and RDS PostgreSQL 16 instance; they do not contain application data or credentials. The local backend serves `/` and `/health`. Its health endpoint checks a TCP connection to PostgreSQL; the verifier independently runs `pg_isready` and `SELECT 1`.

From the repository root, build the backend binary for your Docker host architecture and build the local image. This Dockerfile starts from `scratch` and does not download a base image:

```sh
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o examples/demo/bin/backend ./examples/demo/backend
docker build --pull=false -t eject-demo-api:local examples/demo
```

On an ARM64 Docker host, use `GOARCH=arm64`. Building the image is a separate local step; EJECT does not build or pull it.

Reproduce the EJECT flow in an ignored output directory:

```sh
eject inspect --manifest examples/demo/manifest.json --output examples/demo/output/infrastructure.json
eject plan --input examples/demo/output/infrastructure.json --output examples/demo/output/plan.json
eject generate --plan examples/demo/output/plan.json --output examples/demo/output/project
eject verify --project examples/demo/output/project
```

Before runtime verification, create `examples/demo/output/project/.env` privately with these **fictitious local values**:

```dotenv
DATABASE_HOST=eject-demo-db-2a98991fc0368e39
POSTGRES_PASSWORD=synthetic-local-only
```

The PostgreSQL image `postgres:16-alpine` must be present locally for the default no-pull runtime mode. You may pull it separately, or explicitly pass `--allow-pull` to EJECT. Starting containers requires `--allow-start`:

```sh
eject verify --project examples/demo/output/project --runtime --allow-start --http-check backend-0c64dac5bad49d23:8080:/health
```

The output remains `PARTIAL` as a portability assessment. ECS scheduling and networking, RDS data migration, and application equivalence are outside this demo. Runtime success proves only that these local containers started, their health checks passed, PostgreSQL answered a local query, and the configured HTTP endpoint responded. Named volumes are retained after verification and require manual cleanup if no longer needed.
