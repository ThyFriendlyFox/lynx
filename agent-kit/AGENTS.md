# AGENTS.md — the binding contract

In effect whenever code in this repo is touched, by agent or human.
When this conflicts with intuition, this wins.

## Commands

```sh
go build ./...      # build
go test ./...       # tests
gofmt -l . && go vet ./...       # lint / format check
./verify/verify.sh     # full health gate — must pass before any push
```

Requires Go 1.25.

## Invariants — never regress these

1. **No server.** Lynx never requires a service that the project runs. Peers
   find each other with mDNS and the public DHT. A feature that needs a
   central server is a different product.
2. **The engine owns the folder.** Only `internal/engine` writes inside a
   shared folder. It fetches to a temporary file, verifies the hash, and
   renames. A partly written file never appears under a real name.
3. **Nothing is lost.** Before Lynx replaces or deletes a file, the old bytes
   go to `.lynx/trash/`. Two people who write the same version get a conflict
   copy, and both computers keep both copies.
4. **A path from a peer is hostile until checked.** `safeRel` refuses `..`,
   absolute paths, backslashes and `.lynx/`. Never write a peer path without
   it.
5. **The folder key never leaves the computer.** The share code carries the
   key to a person. On the network, peers prove the key with an HMAC
   challenge, and the DHT topic is a one way hash of it.
6. **A remote write must not look like a local change.** After a file lands,
   the engine sets the file time from the entry. Break this and two peers
   bounce the same file forever.
7. **One command, three systems.** Every change builds for macOS, Windows and
   Linux. `verify.sh` cross builds all 5 targets.
8. **Version, then hash.** Entry order is the Lamport version first, the
   larger hash second. Both peers reach the same answer with no coordination.
   Changing this rule changes what "conflict" means.

## Landmine map

<!-- Places where the obvious change is the wrong change. -->

| Area | Why it bites |
|---|---|
| `internal/engine` `scanLocked` | Same hash with a new file time means "record the time, tell nobody". Send it and peers loop. |
| `internal/p2p` `runSession` | Both peers dial each other. The first session wins; replacing it makes peers flap. |
| `internal/proto` `Conn` | One message is one line, because raw file bytes follow a header on the same stream. A buffered JSON decoder eats those bytes. |
| `internal/index` `Entry.Newer` | Ordering must be total and identical on both sides. A rule that reads local state breaks convergence. |
| `internal/watch` | Editors write a file in 3 steps. The 400 ms settle and the 30 second full scan are both load bearing. |
| `internal/shell` darwin | `document.wflow` is a plist that Automator wrote. Hand edits that drop a key make the menu entry vanish with no error. |
| `.lynx/` | Never sync it. It holds the key, the index and the trash. |

## House style

- Match the surrounding code's idiom, naming, and comment density.
- No demo scaffolding, no leftover diagnostics, no dead flags.
- Comments state constraints the code can't show — never narration.
- User-facing copy states the thing plainly; no reassurance microcopy.

## Process rules

- Branch from `main`; never commit to it directly.
- `./verify/verify.sh` green before every push. Flaky gate → fix or
  quarantine in the same PR; never route around it.
- After adding/removing/renaming source files, run the stack's
  regeneration step (project gen, lockfile, tidy) and commit the result.
- Commit at boundaries; message says what changed and cites evidence.
- Docs move with behavior — same commit or PR.
- Report outcomes faithfully; failing is failing, with output.
