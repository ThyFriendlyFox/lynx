#!/usr/bin/env bash
# Builds one lynx binary per platform into dist/.
set -euo pipefail
cd "$(dirname "$0")/.."

version="${1:-dev}"
rm -rf dist && mkdir -p dist

for target in darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64; do
  os=${target%/*}
  arch=${target#*/}
  name="lynx-${version}-${os}-${arch}"
  out="dist/${name}"
  [ "$os" = "windows" ] && out="${out}.exe"
  GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 go build \
    -trimpath -ldflags "-s -w -X main.version=${version}" \
    -o "$out" ./cmd/lynx
  echo "built $out"
done

( cd dist && shasum -a 256 * > SHA256SUMS 2>/dev/null || sha256sum * > SHA256SUMS )
echo "checksums written to dist/SHA256SUMS"
