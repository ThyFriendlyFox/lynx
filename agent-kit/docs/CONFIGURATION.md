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

## Chat settings

The chat settings live outside every shared folder, because a shared folder
travels to other people. An API key never enters a shared folder.

| Platform | Path |
|---|---|
| macOS | `~/Library/Application Support/lynx/config.json` |
| Windows | `%AppData%\lynx\config.json` |
| Linux | `~/.config/lynx/config.json` |

| Field | Type | Default | Use |
|---|---|---|---|
| `provider` | text | `anthropic` with a key, else `local` | The chat provider. See `docs/CHAT.md`. |
| `model` | text | `claude-opus-5` | The model the anthropic provider asks. |
| `api_key` | text | empty | The API key. `ANTHROPIC_API_KEY` fills it when the field is empty. |
| `effort` | text | `low` | How much the model thinks: `low`, `medium`, `high`, `xhigh`, `max`. |
| `max_tokens` | number | `8000` | The longest answer. |

Write the key with `lynx chat --set-key <key>`. The environment variables
`LYNX_CHAT_PROVIDER` and `LYNX_CHAT_MODEL` override the file for one run.

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
