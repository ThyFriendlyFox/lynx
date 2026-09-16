# VERIFICATION.md — one command answers "is this repo healthy"

`./verify/verify.sh` runs, in order:

1. Format check — `gofmt -l .`
2. Vet — `go vet ./...`
3. Build — `go build ./...`, then `go test ./...`
4. Cross build — one `go build` per platform: darwin/amd64, darwin/arm64,
   linux/amd64, linux/arm64, windows/amd64. A change that breaks one system
   fails the gate on every system.

## Rules

- CI runs **the same command** as local. No CI-only logic.
- A gate that can't run in some environment **skips loudly**, never
  passes silently.
- New behavior lands with its gate in the same PR whenever feasible.
- A feature's completion promise (ROADMAP.md) should be backed by a gate
  here whenever it can be — evidence that keeps proving itself beats
  evidence produced once.
- Fixing a flaky or broken gate is always in scope, for any task.

## What the gate does not cover yet

State these as gaps, never as passes:

- No test starts 2 peers and syncs a file between them. The evidence today is
  a manual run, written in STATUS.md.
- No test runs the right click menu on Windows or Linux.
- No test connects 2 computers over the internet.

Each gap is roadmap work. A cycle that closes one moves a 🚧 row in STATUS.md
to ✅ and adds the gate here.
