# Lynx

Lynx keeps a folder the same on every computer that holds it.

You share a folder and send a friend a code. Your friend runs one command.
From that moment, every change on one computer reaches the other computer in
about 1 second. There is no server, no account and no upload. The bytes go
straight from one computer to the other.

```
you                                    your friend
──────────────                         ──────────────
$ lynx share
lynx-3f9k2m...  ──── send the code ───▶ $ lynx join lynx-3f9k2m...

  notes.md      ◀───── 1 second ──────▶   notes.md
  photos/       ◀───── 1 second ──────▶   photos/
```

## Install

Lynx is one file. Download the file for your system from the
[releases page](https://github.com/thyfriendlyfox/lynx/releases) and put it on
your PATH.

Build it yourself with Go 1.25 or later:

```bash
go install github.com/thyfriendlyfox/lynx/cmd/lynx@latest
```

## Share a folder

```bash
cd ~/my-folder
lynx share
```

Lynx prints a share code. Send the code to your friend.

## Join a folder

```bash
mkdir ~/from-my-friend
cd ~/from-my-friend
lynx join lynx-3f9k2m...
```

The files arrive. Both folders stay the same from then on.

## Use the right click menu

```bash
lynx menu install
```

Right click any folder and choose **Share with Lynx**. Lynx shares the folder,
starts syncing and copies the share code to your clipboard. This works in
Finder on macOS, in Explorer on Windows and in Nautilus or Dolphin on Linux.

Remove it again with `lynx menu remove`.

## Ask about the folder

```bash
cd ~/my-folder
lynx chat
```

Ask a question and Lynx answers about the files in that folder. Ask one
question without the conversation:

```bash
lynx chat "how much is the hotel in the budget?"
```

With no API key, answers come from a word search over the same files. For
full answers, store a key once:

```bash
lynx chat --set-key sk-ant-...
```

The key lives in your own config file, never in the shared folder. Lynx sends
the file list and the text files to the model; it never sends a file that
`.lynxignore` hides. Read [the chat document](agent-kit/docs/CHAT.md) for what
the model receives.

## Other commands

| Command | Use |
|---|---|
| `lynx chat` | Ask about the files in this folder. |
| `lynx status` | Show the peers, the file count and the last event. |
| `lynx log` | Show what happened in this folder. |
| `lynx code` | Print the share code again. |
| `lynx start` / `lynx stop` | Start or stop syncing this folder. |
| `lynx run` | Sync in this window and print every event. |

## Keep files out of the folder

Write `.lynxignore` in the folder:

```
*.log
/build/
secret/
```

Lynx always ignores `.lynx/`, `.git/`, `node_modules/` and the files your
operating system writes by itself.

## How it works

Lynx runs a small program per shared folder. The program watches the files,
finds the other copies of the folder and moves the changes.

- **Finding.** On a local network, Lynx uses multicast DNS. On the internet,
  Lynx publishes a record in the public libp2p distributed hash table under a
  topic that is a one way hash of the folder key.
- **Proving.** Two peers prove that they hold the same folder key with an HMAC
  challenge. A peer that fails gets no data.
- **Moving.** The connection is encrypted by libp2p. Lynx fetches a file to a
  temporary file, checks the hash and renames it into place.
- **Deciding.** Every file carries a version number. The higher version wins.
  When both sides write the same version, the larger hash wins and the other
  copy is kept as `<name> (conflict from <peer> <date>)`.
- **Keeping.** Every file that Lynx replaces or deletes goes to `.lynx/trash/`.

Read [the architecture](agent-kit/docs/ARCHITECTURE.md) for the full map.

## What this version does not do

- It does not connect 2 computers that both sit behind a network which blocks
  direct connections. The relay work is roadmap item 2.
- It does not push to git yet. The bridge is roadmap item 4.
- It does not sync `.git/`, empty folders, symbolic links or files above 2 GB.

## Security

Anyone who holds the share code holds the folder. Send it the way you would
send a house key. Report a vulnerability as described in
[SECURITY.md](agent-kit/SECURITY.md); never in a public issue.

## Contributing

Read [CONTRIBUTING.md](agent-kit/CONTRIBUTING.md). The work that is planned
lives in [ROADMAP.md](agent-kit/ROADMAP.md), and nothing gets built that is
not on it. AI agents that work on this repository start at
[agent-kit/ROUTING.md](agent-kit/ROUTING.md).

## License

MIT. See [LICENSE](LICENSE).
