# STATUS.md — where the project actually stands

The single source of truth for project state. Claims require evidence: a
passing gate, a linked run, a tag. Updated in the same commit as the
behavior change. The weekly cycle (WEEKLY.md step 5) refreshes it.

| Area | State | Evidence |
|---|---|---|
| Live sync between 2 folders | ✅ | 2 folders on 1 Mac synced an add, an edit, a delete, a rename and a 20 MB file in about 1 second each. |
| Conflict handling | ✅ | Both peers kept the same winner and the same conflict copy after an offline edit on both sides. |
| Share code and join | ✅ | `lynx share --code-only` then `lynx join <code>` created a synced folder. |
| Right click menu, macOS | ✅ | `lynx menu install`, then `pbs -dump_pboard` lists "Share with Lynx"; running the Quick Action shared a folder and copied the code. |
| Right click menu, Windows | 🚧 | The code writes the registry keys and cross builds. No run on a real Windows computer yet. |
| Right click menu, Linux | 🚧 | The code writes a Nautilus script and a KDE service menu. No run on a real Linux computer yet. |
| Peer lookup over the public DHT | ✅ | 2 folders with `discovery: dht` and no local network lookup found each other through the public libp2p table in 90 seconds and synced a file. |
| Sync between 2 different networks | 🚧 | Both peers in the test above sat on 1 network. NAT traversal between 2 networks is roadmap item 2. |
| Talk to the folder, local provider | ✅ | `verify/chat-gate.sh` green: `lynx chat` names `notes/budget.md` for "how much is the hotel?" and says "No file" for a question the folder cannot answer. |
| Talk to the folder, Claude provider | 🚧 | The provider compiles, streams and caches the folder description. No run against the real API: this machine holds no API key. A human with a key proves it. |
| Read the folder with git | ❌ | Roadmap item 4. |
| Discovery choice (`both`, `mdns`, `dht`) | ✅ | The DHT only run above used the `discovery` field. |
| Several shared folders at once | ✅ | 2 folders with 2 different keys, each joined by a second copy: 4 processes ran at once, each pair synced its own file, and no file crossed between the pairs. |
| Public repository and CI | ✅ | https://github.com/ThyFriendlyFox/lynx — main pushed, CI green, Release workflow published v0.1.0 and v0.2.0. |
| Published binaries | ✅ | 5 binaries for v0.2.0 with `SHA256SUMS`; every checksum verified and `lynx-v0.2.0-darwin-arm64 version` printed `lynx v0.2.0`. |
| Health gate | ✅ | `./verify/verify.sh` green: format, vet, build, 5 cross builds, 4 test packages. |

States: ✅ done (gated) · 🚧 in progress · ❌ not started · 🧊 frozen/won't do.

## Current week

- **Shipping:** between cycles. The next item is "Connect when the network is hard".
- **Last release:** v0.2.0 — 2026-09-16, published at
  https://github.com/ThyFriendlyFox/lynx/releases/tag/v0.2.0 with 5 binaries
  and checksums. `go install github.com/thyfriendlyfox/lynx/cmd/lynx@v0.2.0`
  works.
- **Known red:** none. The untested paths are Windows, Linux, the connection
  between 2 networks and the Claude chat provider; they are marked 🚧 above,
  not ✅.
