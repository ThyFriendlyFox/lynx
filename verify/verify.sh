#!/usr/bin/env bash
# The health gate. CI runs this same script. No CI-only logic lives here.
set -euo pipefail
cd "$(dirname "$0")/.."

fail=0
step() { printf '\n== %s ==\n' "$1"; }

step "format"
unformatted=$(gofmt -l . | grep -v '^agent-kit/' || true)
if [ -n "$unformatted" ]; then
  echo "these files are not formatted:"
  echo "$unformatted"
  echo "run: gofmt -w ."
  fail=1
fi

step "vet"
go vet ./... || fail=1

step "build"
go build ./... || fail=1

step "cross build"
for target in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64; do
  GOOS=${target%/*} GOARCH=${target#*/} go build -o /dev/null ./cmd/lynx || { echo "build for $target failed"; fail=1; }
done

step "test"
go test ./... || fail=1

if [ "$fail" -ne 0 ]; then
  printf '\nverify: FAILED\n'
  exit 1
fi
printf '\nverify: passed\n'
