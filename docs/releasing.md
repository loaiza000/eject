# Release procedure

This is a preparation checklist, not authorization to publish. EJECT v0.1.0 must not be tagged or released until the maintainer approves the final reviewed commit and all required CI jobs pass.

## Release gate

- [ ] Review the full diff, untracked files, licenses, `.gitignore`, dependency list, and secret scan.
- [ ] Review the compatibility matrix and examples; confirm no claim of AWS functional or data equivalence.
- [ ] Merge a reviewed pull request only after Linux, Windows, macOS, fuzz, Docker integration, and Linux race jobs pass on GitHub Actions. Local cross builds do not substitute for runner results.
- [ ] Confirm Windows ACL tests and a clean-host audit: new outputs and temporary files exclude inherited readers; legacy permissive projects fail static verification. Inspect any platform or filesystem where the DACL API is unavailable.
- [ ] Confirm `go build ./...`, `go test ./...`, `go vet ./...`, `go mod verify`, `govulncheck`, and the authorized demo integration on the release commit.
- [ ] Update `CHANGELOG.md` from `[Unreleased]` to `[0.1.0]` with the approved date, and update `SECURITY.md` support wording.
- [ ] Confirm the checkout is clean and the release tag points to the approved commit.
- [ ] Build binaries, license texts and `SHA256SUMS` with Go 1.27.3 using `bash scripts/build-release.sh v0.1.0` on a clean Linux checkout; review names, versions, notices and hashes.
- [ ] Obtain separate approval for pushing the tag, creating a draft GitHub Release, and publishing it.

## Review and commit proposal

The first commit `6d16d16` contains Phase 0–1 and was published unchanged to `origin/main` before the review branch was created. The subsequent work is organized as four coherent commits in one pull request, without rewriting that history:

1. `feat: add compatibility assessment and versioned migration plans` — Phase 2 rules, plan v2, tests, fixtures, and docs.
2. `feat: generate guarded Docker Compose proposals` — Phase 3 generator, marker, golden output, tests, and docs.
3. `feat: verify generated projects and runtime behavior` — Phase 4 static/runtime verification, demo, fault tests, and verification documentation.
4. `fix: protect Windows artifacts and prepare v0.1.0 release` — protected ACLs, permission tests, CI, security review, and release documentation.

Review each commit's diff, especially shared documentation and `go.mod`. Open a **draft** pull request from `codex/v0.1.0` into `main`; request code and security review. The local draft body is `.eject/pr-v0.1.0-draft.md` and is ignored by Git. Treat CI results and the reviewed diff as the release evidence. Do not label a local test run as a successful GitHub Actions run.

## Assets and publication

Use semantic version tags `vMAJOR.MINOR.PATCH`. `v0.1.0` is the first public preview; JSON schema versions are independent of the CLI version. The release script produces six raw binaries (Linux, macOS, Windows on amd64/arm64), license and notice files, and `SHA256SUMS` under `dist/v0.1.0/` from a clean, tagged checkout. The script does not push or publish. Distribute the license and notice files with each binary. Verify downloaded assets with `sha256sum -c SHA256SUMS` in the asset directory.

After explicit approval, push the annotated tag, create a **draft** release with assets and reviewed notes, inspect its downloads and checksums, then obtain final publication approval. An example command for that authorized stage is:

```sh
gh release create v0.1.0 dist/v0.1.0/* --draft --verify-tag --title "EJECT v0.1.0" --notes-file docs/release-notes-v0.1.0.md
```

The draft command does not publish the release. Publishing later is a separate action. Do not create the tag, draft, or release as part of routine development.
