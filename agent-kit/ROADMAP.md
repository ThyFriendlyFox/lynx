# ROADMAP.md — the source of all work

**This file is not optional.** Every feature the agent builds flows down
from here. If it isn't on this roadmap, it doesn't get built; if it needs
building, it gets added here first. One item ships per weekly cycle
(see `WEEKLY.md`).

## North star

Lynx is becoming the way two people share a live folder with no account, no
server and no upload. A person sends a code; a folder appears on the other
computer and stays the same on both. The folder then becomes something a
person can talk to and, when the project wants it, something git can read.
A person who has never used git contributes to a project by dropping a file
in a folder.

## Feature Queue — ordered; top unblocked item ships next

### 1. Connect when the network is hard
- **Promise:** Two computers on different networks, both behind a NAT that
  blocks direct connections, sync a file in under 60 seconds.
- **Evidence:** A run on 2 real machines on 2 networks, with the log lines
  and `lynx status` output pasted in the PR. `lynx status` gains a line that
  names the connection type: local, direct or relayed.
- **Use case:** docs/USE-CASES.md, "Send a folder to a friend".
- **Scope guard:** Use libp2p circuit relay and hole punching. Lynx runs no
  relay server of its own in this item.
- **Status:** ready — needs a human with a second machine for the evidence.

### 2. Install without a terminal
- **Promise:** A person installs Lynx on macOS, Windows or Linux with one
  download, and the right click menu works without a terminal command.
- **Evidence:** Release artifacts for 5 platforms with checksums, and a
  screenshot of the menu entry on macOS and on Windows.
- **Use case:** docs/USE-CASES.md, "Send a folder to a friend".
- **Scope guard:** No code signing certificate and no notarization in this
  item; document the warning each system shows instead.
- **Status:** ready

### 3. Read the folder with git
- **Promise:** `lynx github link <repo>` pushes the current folder to a
  branch, and every later change lands as a commit on that branch within 60
  seconds.
- **Evidence:** A public test repository with commits made by the bridge, and
  a `lynx status` line that names the last push.
- **Use case:** docs/USE-CASES.md, "Contribute without git".
- **Scope guard:** One direction only: folder to git. No pull, no merge, no
  branch selection UI.
- **Status:** ready

### 4. Prove the Claude chat against the real API
- **Promise:** `lynx chat` answers a question with the Claude provider and
  the answer names a real file from the folder.
- **Evidence:** A transcript of one session with the real API in the PR, and
  a second gate case that runs when `ANTHROPIC_API_KEY` exists and skips
  loudly when it does not.
- **Use case:** docs/USE-CASES.md, "Ask a question about the folder".
- **Scope guard:** No new provider. No tools for the model.
- **Status:** ready — needs a human with an API key.

## Later — candidates, not yet specced

- Start at login — a shared folder syncs again after a restart, with no
  command. Today a person runs `lynx start` again.
- Folder history — a person restores yesterday's version of a file.
- Selective sync — a person joins a folder but takes only one subfolder.
- Chat between peers — a message box next to the folder, not only an AI.
- A folder that is read only for the people who join it.
- Encryption of `.lynx/config.json` with a passphrase.
- A small window instead of the terminal, for people who never open one.
- Mobile: a phone holds a folder.

## Shipped

| Week | Feature | Release | Evidence |
|---|---|---|---|
| 2026-09-16 | Talk to the folder | v0.2.0 | `verify/chat-gate.sh` green: `lynx chat "how much is the hotel?"` names `notes/budget.md`, and an unanswerable question returns "No file". |
| 2026-09-15 | Live shared folders, share codes, right click menu | v0.1.0 | Two folders synced a file in 1 second; `./verify/verify.sh` green; macOS Quick Action registered and run. |

## Explicitly not doing

- A Lynx server that holds people's files. The project has no servers on
  purpose.
- An account system. The share code is the only identity.
- Syncing `.git/`. Two computers that write a git index at the same time
  destroy it. The git bridge in item 4 is the supported path.
- A full text CRDT for live typing in the same file. The conflict copy is the
  answer in this version.

## Queue changes

- 2026-09-16: Shipped "Talk to the folder". The item promised a `--once`
  flag; the command takes the question as plain words instead, because one
  way to ask is enough. The gate runs the local provider, so it needs no key
  and no network. The unproven Claude path became new item 4.
- 2026-09-15: Seeded the queue after the first build. The human ranked chat
  first, network hardening second. Mark: provisional order, the human
  re-orders at any time.
