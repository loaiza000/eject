# Release procedure

This is a preparation checklist, not authorization to publish. GitHub Actions jobs currently stop before runner assignment because of an account billing lock. Do not describe those jobs as passing or require payment as a release workaround. The maintainer may approve a v0.1.0 preview after reviewing the manual gate below and explicitly accepting the remaining platform limits. Tagging and publication still require separate authorization.

## Release gate

- [ ] Review the full diff, untracked files, licenses, `.gitignore`, dependency list, and secret scan.
- [ ] Review the compatibility matrix and examples; confirm no claim of AWS functional or data equivalence.
- [ ] Record the actual GitHub Actions status. If runners remain unavailable, attach local evidence to the pull request: native Windows and Linux build/tests/vet/module verification, Linux race detector, bounded fuzzing, vulnerability scan, and authorized Docker integration. Never label this evidence as successful CI.
- [ ] Confirm Windows ACL tests: new outputs and temporary files exclude inherited readers, while permissive existing generations fail verification. Review on an independent Windows host when available; if unavailable, record that limit before release.
- [ ] Confirm Unix mode tests: generated projects and files exclude group and other access, and generation fails before content is written when a mounted filesystem cannot enforce private modes.
- [ ] Confirm the six cross builds and unpublishable packaging smoke test on a clean commit. Native macOS and arm64 execution remain unverified until a matching host is available; disclose this limitation if those binaries are distributed.
- [ ] Obtain maintainer review of the diff, golden files, licenses, dependency list, secret scan, local results, and residual platform risk before merging or authorizing a release.
- [ ] Update `CHANGELOG.md` from `[Unreleased]` to `[0.1.0]` with the approved date, and update `SECURITY.md` support wording.
- [ ] Confirm the checkout is clean and the release tag points to the approved commit.
- [ ] Build binaries, per-platform archives, license texts and `SHA256SUMS` with Go 1.27.3 using `bash scripts/build-release.sh v0.1.0` on a clean Linux checkout; review names, versions, archive contents, executable modes, notices and hashes.
- [ ] Obtain separate approval for pushing the tag, creating a draft GitHub Release, and publishing it.

## Review and commit proposal

The first commit `6d16d16` contains Phase 0–1 and was published unchanged to `origin/main` before the review branch was created. Four phase and hardening commits establish the review branch, followed by focused corrective commits; no published history was rewritten:

1. `feat: add compatibility assessment and versioned migration plans` — Phase 2 rules, plan v2, tests, fixtures, and docs.
2. `feat: generate guarded Docker Compose proposals` — Phase 3 generator, marker, golden output, tests, and docs.
3. `feat: verify generated projects and runtime behavior` — Phase 4 static/runtime verification, demo, fault tests, and verification documentation.
4. `fix: protect Windows artifacts and prepare v0.1.0 release` — protected ACLs, permission tests, CI, security review, and release documentation.

The follow-up commits harden symlinked output paths, expand HTTP runtime checks, add a nonpublishable packaging smoke test, and preserve executable modes in Unix archives. Review them with the four foundation commits.

Review each commit's diff, especially shared documentation and `go.mod`. Draft pull request #1 is open from `codex/v0.1.0` into `main` for code and security review. The local draft body is `.eject/pr-v0.1.0-draft.md` and is ignored by Git. Treat CI results and the reviewed diff as the release evidence. Do not label a local test run as a successful GitHub Actions run.

## Assets and publication

Use semantic version tags `vMAJOR.MINOR.PATCH`. `v0.1.0` is the first public preview; JSON schema versions are independent of the CLI version. The release script produces six raw binaries and six `.tar.gz` archives (Linux, macOS, Windows on amd64/arm64), license and notice files, and `SHA256SUMS` under `dist/v0.1.0/` from a clean, tagged checkout. Each archive includes the matching binary, Apache license, and third-party notices and license texts. The script does not push or publish. Verify downloaded assets with `sha256sum -c SHA256SUMS` in the asset directory. Windows users can extract `.tar.gz` with `tar -xzf` from PowerShell.

Before a release tag is authorized, `bash scripts/build-release.sh v0.1.0 --smoke` exercises the same packaging from a clean, immutable HEAD and writes `dist/smoke-v0.1.0/`. These assets are explicitly unpublishable: they do not prove that an approved tag or CI run exists. Review their checksums, archive members, injected `version` output, and native binary execution where the corresponding OS is available. Cross-compilation alone cannot establish native behavior on another platform.

After explicit approval, push the annotated tag, create a **draft** release with assets and reviewed notes, inspect its downloads and checksums, then obtain final publication approval. An example command for that authorized stage is:

```sh
gh release create v0.1.0 dist/v0.1.0/* --draft --verify-tag --title "EJECT v0.1.0" --notes-file docs/release-notes-v0.1.0.md
```

The draft command does not publish the release. Publishing later is a separate action. Do not create the tag, draft, or release as part of routine development.
