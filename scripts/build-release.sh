#!/usr/bin/env bash
set -euo pipefail

version="${1:?usage: scripts/build-release.sh vX.Y.Z[-rc.N] [--smoke]}"
mode="${2:-}"
if [[ -n "$mode" && "$mode" != "--smoke" ]] || (( $# > 2 )); then
  echo "usage: scripts/build-release.sh vX.Y.Z[-rc.N] [--smoke]" >&2
  exit 2
fi
if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-rc\.[0-9]+)?$ ]]; then
  echo "expected a semantic version tag (vX.Y.Z or vX.Y.Z-rc.N)" >&2
  exit 2
fi
if [[ -n "$(git status --porcelain)" ]]; then
  echo "release builds require a clean checkout" >&2
  exit 2
fi
if [[ "$mode" != "--smoke" ]]; then
  if [[ "$(git cat-file -t "refs/tags/$version" 2>/dev/null || true)" != tag ]]; then
    echo "release builds require an approved annotated tag: $version" >&2
    exit 2
  fi
  approved_commit="$(git rev-parse --verify --quiet "refs/tags/$version^{commit}")"
  if [[ "$(git rev-parse HEAD)" != "$approved_commit" ]]; then
    echo "HEAD must equal the approved release tag" >&2
    exit 2
  fi
fi
if [[ "$(go version)" != *"go1.27.3 "* ]]; then
  echo "release builds require Go 1.27.3" >&2
  exit 2
fi

outdir="dist/$version"
if [[ "$mode" == "--smoke" ]]; then outdir="dist/smoke-$version"; fi
if [[ -e "$outdir" ]]; then
  echo "release output already exists: $outdir" >&2
  exit 2
fi
mkdir -p "$outdir"
go mod download
go mod verify
modcache="$(go env GOMODCACHE)"
if command -v cygpath >/dev/null 2>&1; then modcache="$(cygpath -u "$modcache")"; fi
cp LICENSE THIRD_PARTY_NOTICES.md "$outdir/"
cp "$modcache/github.com/spf13/cobra@v1.10.2/LICENSE.txt" "$outdir/LICENSE.cobra.txt"
cp "$modcache/github.com/spf13/pflag@v1.0.9/LICENSE" "$outdir/LICENSE.pflag.txt"
cp "$modcache/github.com/inconshreveable/mousetrap@v1.1.0/LICENSE" "$outdir/LICENSE.mousetrap.txt"
cp "$modcache/gopkg.in/yaml.v3@v3.0.1/LICENSE" "$outdir/LICENSE.yaml.txt"
cp "$modcache/gopkg.in/yaml.v3@v3.0.1/NOTICE" "$outdir/NOTICE.yaml.txt"
cp "$modcache/golang.org/x/sys@v0.49.0/LICENSE" "$outdir/LICENSE.x-sys.txt"
legal=(LICENSE THIRD_PARTY_NOTICES.md LICENSE.cobra.txt LICENSE.pflag.txt LICENSE.mousetrap.txt LICENSE.yaml.txt NOTICE.yaml.txt LICENSE.x-sys.txt)
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
  os="${target%/*}"
  arch="${target#*/}"
  suffix=""
  if [[ "$os" == windows ]]; then suffix=".exe"; fi
  binary="eject_${os}_${arch}${suffix}"
  output="$outdir/$binary"
  GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 go build -trimpath -buildvcs=false \
    -ldflags="-X main.version=$version" -o "$output" ./cmd/eject
  # Inside each archive the executable is plainly `eject` (or `eject.exe`), so
  # users can extract and run it without renaming. Raw binaries keep the
  # platform suffix because they share one release directory.
  package="$outdir/.package"
  mkdir "$package"
  member="eject${suffix}"
  cp "$output" "$package/$member"
  for file in "${legal[@]}"; do cp "$outdir/$file" "$package/$file"; done
  archive_tar="$(pwd)/$outdir/.package.tar"
  binary_mode=755
  if [[ "$os" == windows ]]; then binary_mode=644; fi
  (
    cd "$package"
    tar --sort=name --mtime='@0' --owner=0 --group=0 --numeric-owner --mode="$binary_mode" -cf "$archive_tar" "$member"
    tar --sort=name --mtime='@0' --owner=0 --group=0 --numeric-owner --mode=644 -rf "$archive_tar" "${legal[@]}"
  )
  gzip -n < "$archive_tar" > "$outdir/eject_${version}_${os}_${arch}.tar.gz"
  rm "$archive_tar"
  rm "$package/$member"
  for file in "${legal[@]}"; do rm "$package/$file"; done
  rmdir "$package"
done
(
  cd "$outdir"
  sha256sum eject_* "${legal[@]}" > SHA256SUMS
  sha256sum -c SHA256SUMS
)
if [[ "$mode" == "--smoke" ]]; then
  echo "Built unpublishable smoke assets from $(git rev-parse HEAD) in $outdir/."
else
  echo "Built $version assets in $outdir/; no release was published."
fi
