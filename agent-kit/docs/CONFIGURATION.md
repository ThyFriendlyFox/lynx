# Configuration

Lynx reads one file per shared folder: `<folder>/.lynx/config.json`. Open it
in any text editor. Lynx writes the file when the folder is shared or joined.

| Platform | Path |
|---|---|
| macOS | `<folder>/.lynx/config.json` |
| Windows | `<folder>\.lynx\config.json` |
| Linux | `<folder>/.lynx/config.json` |

A folder with no config file is not shared. A damaged config file stops that
one folder with a message; it never stops another folder and never deletes
data. Restart the folder after an edit: `lynx stop` then `lynx start`.

## Fields

| Field | Type | Default | Use |
|---|---|---|---|
| `version` | number | `1` | The format of this file. |
| `name` | text | the folder name | The name of the folder. |
| `key` | text | a new random key | The folder key, in base64. Anyone with this key reads and writes the folder. |
| `peer_name` | text | the computer name | The name other people see for this computer. |
| `created` | number | the time of sharing | When the folder was shared, in milliseconds. |
| `port` | number | `0` | The network port. `0` means a free port. |
| `discovery` | text | `both` | How the folder finds peers: `both`, `mdns` for the local network only, or `dht` for the internet only. |

## Other state in `.lynx/`

| File | Use |
|---|---|
| `index.json` | The known path, hash and version of every file. |
| `peer.key` | The network identity of this computer for this folder. |
| `daemon.json` | The state of the running process. `lynx status` reads it. |
| `lynx.log` | What happened in this folder. |
| `trash/` | Files that Lynx replaced or deleted. Nothing is lost. |

## Ignore file

Write `.lynxignore` in the folder root to keep files local. One pattern per
line. `#` starts a comment. `!` keeps a file that an earlier line hides.

```
*.log
/build/
secret/
!keep.log
```

Lynx always ignores `.lynx/`, `.git/`, `.hg/`, `.svn/`, `node_modules/`,
`.DS_Store`, `Thumbs.db`, `desktop.ini`, `*.swp` and `*~`.
