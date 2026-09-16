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
| Sync over the internet (DHT) | 🚧 | The peer lookup starts and advertises. No run between 2 different networks yet. |
| Talk to the folder (chat) | ❌ | Roadmap item 1. |
| Read the folder with git | ❌ | Roadmap item 4. |
| Health gate | ✅ | `./verify/verify.sh` green: format, vet, build, 5 cross builds, 4 test packages. |

States: ✅ done (gated) · 🚧 in progress · ❌ not started · 🧊 frozen/won't do.

## Current week

- **Shipping:** between cycles. The next item is "Talk to the folder".
- **Last release:** none yet. v0.1.0 is ready to tag.
- **Known red:** none. The untested paths are Windows, Linux and the
  internet connection; they are marked 🚧 above, not ✅.
