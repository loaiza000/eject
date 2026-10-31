#!/usr/bin/env bash
set -euo pipefail

version="${1:?usage: scripts/build-release.sh vX.Y.Z}"
if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "expected a stable semantic version tag (vX.Y.Z)" >&2
  exit 2
fi
if [[ -n "$(git status --porcelain)" ]]; then
  echo "release builds require a clean checkout" >&2
  exit 2
fi
if [[ "$(git rev-parse HEAD)" != "$(git rev-parse "refs/tags/$version^{commit}")" ]]; then
  echo "HEAD must equal the approved release tag" >&2
  exit 2
fi
if [[ "$(go version)" != *"go1.27.3 "* ]]; then
  echo "release builds require Go 1.27.3" >&2
  exit 2
fi

outdir="dist/$version"
if [[ -e "$outdir" ]]; then
  echo "release output already exists: $outdir" >&2
  exit 2
fi
mkdir -p "$outdir"
go mod download
modcache="$(go env GOMODCACHE)"
cp LICENSE THIRD_PARTY_NOTICES.md "$outdir/"
cp "$modcache/github.com/spf13/cobra@v1.10.2/LICENSE.txt" "$outdir/LICENSE.cobra.txt"
cp "$modcache/github.com/spf13/pflag@v1.0.9/LICENSE" "$outdir/LICENSE.pflag.txt"
cp "$modcache/github.com/inconshreveable/mousetrap@v1.1.0/LICENSE" "$outdir/LICENSE.mousetrap.txt"
cp "$modcache/gopkg.in/yaml.v3@v3.0.1/LICENSE" "$outdir/LICENSE.yaml.txt"
cp "$modcache/gopkg.in/yaml.v3@v3.0.1/NOTICE" "$outdir/NOTICE.yaml.txt"
cp "$modcache/golang.org/x/sys@v0.49.0/LICENSE" "$outdir/LICENSE.x-sys.txt"
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
  os="${target%/*}"
  arch="${target#*/}"
  suffix=""
  if [[ "$os" == windows ]]; then suffix=".exe"; fi
  output="$outdir/eject_${os}_${arch}${suffix}"
  GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 go build -trimpath -buildvcs=false \
    -ldflags="-X main.version=$version" -o "$output" ./cmd/eject
done
(
  cd "$outdir"
  sha256sum eject_* LICENSE LICENSE.* THIRD_PARTY_NOTICES.md NOTICE.* > SHA256SUMS
  sha256sum -c SHA256SUMS
)
echo "Built $version assets in $outdir/; no release was published."
