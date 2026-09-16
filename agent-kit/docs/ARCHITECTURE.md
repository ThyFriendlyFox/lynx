# Architecture

Lynx is a command line program in Go. It runs on macOS, Windows and Linux.
The code has 10 parts.

| Part | Folder | Task |
|---|---|---|
| Command line | `internal/cli` | Reads the command a person typed and calls one part. |
| Folder state | `internal/folder` | Owns `.lynx/`, the config file and the folder key. |
| Share code | `internal/sharecode` | Turns the folder key into the code a person sends, and back. |
| Index | `internal/index` | Knows the path, size, time, hash and version of every file. |
| Ignore | `internal/ignore` | Decides which files stay local. |
| Watcher | `internal/watch` | Reports that something in the folder changed. |
| Engine | `internal/engine` | Records local changes and applies remote changes. |
| Protocol | `internal/proto` | The messages and the greeting between two peers. |
| Network | `internal/p2p` | Finds the peers and moves the bytes. |
| Daemon | `internal/daemon` | Runs the folder in the background and publishes its state. |
| Menu | `internal/shell` | Adds "Share with Lynx" to the right click menu. |

## Data flow

A person edits a file. The other computer has the new file 1 second later.

1. The person saves `notes.md` in the shared folder.
2. `internal/watch` sees the file event and waits 400 ms for more events.
3. `internal/engine` walks the folder, hashes `notes.md`, and sees a new hash.
4. The engine writes a new entry: version 4, origin "laptop", the new hash.
5. `internal/p2p` sends the entry to every connected peer as an update.
6. The peer compares the entry with its own entry for `notes.md`.
7. The peer opens a second stream and asks for the bytes of version 4.
8. This computer answers with a header and the bytes of the file.
9. The peer checks the hash, moves any file it would overwrite, and renames
   the new file into place.
10. The peer sets the file time to the time in the entry, so the next walk on
    that computer reports no change.

## Network

Lynx has no server. Two computers find each other in two ways:

1. **Local network.** Multicast DNS finds a peer in the same house or office.
2. **Internet.** The public libp2p distributed hash table holds a record under
   the topic `lynx/1/<folder id>`. The folder id is a hash of the folder key,
   so the topic never exposes the key.

After a connection exists, each side proves that it holds the folder key with
an HMAC challenge. A peer that fails the challenge gets no data.

## Boundaries

| Boundary | Rule |
|---|---|
| `p2p` ↔ `engine` | The network hands the engine entries and bytes. The network never writes a file. |
| `engine` ↔ disk | Only the engine writes inside the folder. It writes to a temporary file and renames it. |
| `engine` ↔ `.lynx/` | The engine never syncs `.lynx/`. A path from a peer that points at `.lynx/` or outside the folder is refused. |
| `cli` ↔ `daemon` | The command line reads `.lynx/daemon.json`. It never reaches into the running process. |
| `shell` ↔ `cli` | The right click menu calls `lynx menu share <folder>`. It holds no sync logic. |
