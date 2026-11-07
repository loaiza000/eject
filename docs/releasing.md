# Release procedure

This document is a procedure, not an authorization. Merging, tagging, and publishing each require an explicit maintainer decision. Nothing in the repository publishes automatically.

## Versioning

Tags follow Semantic Versioning: `vMAJOR.MINOR.PATCH`, optionally with a release-candidate suffix such as `v0.1.0-rc.1`. While the major version is `0`, any release may change behavior. Mark releases as **pre-release** on GitHub until the platform matrix includes native evidence for every published binary. JSON schema versions (`schema_version` in inventories, plans, and reports) are independent of the CLI version.

## Evidence policy

GitHub Actions jobs for this repository currently do not start, because the account is locked over a billing issue ("The job was not started because your account is locked due to a billing issue"). That is an administrative block, not a test failure. Do not pay for runners, switch accounts, or edit the workflow to make it appear green. Until CI runs:

- Record local evidence (commands and results) in the pull request, and label it as local, never as CI.
- State which platforms were executed natively and which were only cross-compiled.
- Keep `.github/workflows/ci.yml` unchanged so CI runs as soon as the account is unblocked.

## Gate before tagging

Run on a clean checkout of the commit to be released.

Windows (Git Bash) and Linux (WSL2 or native):

```sh
go build ./...
go vet ./...
go mod verify
test -z "$(gofmt -l $(git ls-files '*.go'))"
go test -count=1 ./...
```

Linux only:

```sh
CGO_ENABLED=1 go test -race -count=1 ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
for f in FuzzManifest FuzzTaskResponse; do go test -run '^$' -fuzz "^$f\$" -fuzztime 30s ./internal/discovery/aws; done
go test -run '^$' -fuzz '^FuzzGeneratePlan$' -fuzztime 30s ./internal/cli
for f in FuzzParseStates FuzzHTTPCheck; do go test -run '^$' -fuzz "^$f\$" -fuzztime 30s ./internal/verification; done
```

Docker integration (local images only: build `eject-demo-api:local` as described in [the demo](../examples/demo/README.md), and have `postgres:16-alpine` present):

```sh
EJECT_RUNTIME_TEST=1 go test -run '^TestAuthorizedDemoRuntime$' -count=1 ./tests/integration
```

Each successful run leaves one named volume `eject-verify-<id>_..._data`. Remove these after review with `docker volume rm <name>`; never use a broad prune.

Packaging smoke test. This requires GNU tar, so run it on Linux or WSL from a clean checkout; it writes `dist/smoke-<version>/`, which is ignored by Git and **not publishable**:

```sh
bash scripts/build-release.sh v0.1.0 --smoke
(cd dist/smoke-v0.1.0 && sha256sum -c SHA256SUMS)
```

Check the archive members (`tar -tvzf`), the executable modes (`-rwxr-xr-x` for Unix binaries), and the `version` output of the binaries you can run natively.

## Release steps (maintainer)

1. **Review and merge** the release pull request into `main` on GitHub, after reading the diff and the local evidence in its description.

2. **Update the local `main`** and confirm a clean tree:

   ```sh
   git switch main
   git pull --ff-only origin main
   git status --short          # must print nothing
   ```

3. **Create the annotated tag** on the merged commit (local only for now):

   ```sh
   git tag -a v0.1.0 -m "EJECT v0.1.0"
   ```

4. **Build the release assets** on Linux or WSL with Go 1.27.3. The script refuses to run unless `HEAD` equals the annotated tag and the tree is clean:

   ```sh
   bash scripts/build-release.sh v0.1.0
   (cd dist/v0.1.0 && sha256sum -c SHA256SUMS)
   ```

5. **Push the tag**. This is the first public, outward-facing step:

   ```sh
   git push origin v0.1.0
   ```

6. **Create a draft pre-release** with the assets. A draft is not public:

   ```sh
   gh release create v0.1.0 dist/v0.1.0/* --draft --prerelease --verify-tag \
     --title "EJECT v0.1.0" --notes-file docs/release-notes-v0.1.0.md
   ```

7. **Inspect the draft** on GitHub. Download one archive and `SHA256SUMS`, then follow [installation](installation.md) to confirm that the checksum and `eject version` match.

8. **Publish**:

   ```sh
   gh release edit v0.1.0 --draft=false
   ```

If a defect is found after step 5, do not move or delete a pushed tag. Fix it and release `v0.1.1`.

## Optional repository settings

- Enable **Private vulnerability reporting** (Settings → Code security) so the process in `SECURITY.md` works.
- Add topics such as `cli`, `docker-compose`, `aws`, `cloud-migration`, `golang`, and `offline-first`.
- Documentation can later be published free of charge with GitHub Pages from the `docs/` folder (Settings → Pages → Deploy from branch → `main` / `docs`). A custom domain such as `eject.dloaiza.dev` needs a DNS `CNAME` record pointing to `loaiza000.github.io` and the domain entered in the Pages settings. That is a separate decision and is not configured here.
