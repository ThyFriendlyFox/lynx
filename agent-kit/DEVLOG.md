# DEVLOG.md — the live devlog

The plain history of this project. A person who knows nothing about the
code reads this file and knows what happened, in order, with no jargon.
Append only. Never rewrite an old entry — a wrong entry gets a
correction entry, not an edit.

Write every entry in the voice of `TONE.md`.

## Entry shape

Every entry has the same shape:

```
## YYYY-MM-DD — <one line: what happened>

<What we did. What worked. What broke. What we learned.
3–10 short sentences. Plain words. Past tense for what happened,
present tense for how things now stand.>

Evidence: <commit / tag / gate run / screenshot>
```

## When to write

- Every WEEKLY.md cycle writes one entry at step 5 (before merge).
- A failed or abandoned attempt gets an entry too. The devlog records
  what happened, not what succeeded. A week with no shipped feature
  still gets its entry.
- Out-of-band work (security patch, gate repair, big triage) gets one.
- SETUP.md writes the first entry: "Installed the agent kit."

## What does not go here

- Code detail that belongs in commit messages.
- Promises about the future — that is ROADMAP.md.
- State claims — that is STATUS.md. The devlog is the story; STATUS is
  the snapshot.

---

<!-- Entries below, newest first. -->

## 2026-09-15 — Installed the agent kit and built the first working Lynx

I started with an empty folder and this kit. The goal from the human: send a
friend a folder, and both folders stay the same, live, with no server and no
git account.

I asked 3 questions before any code. The human chose Go, peer to peer only,
and no AI chat in the first version. Go gives 1 binary per system with no
runtime to install. Peer to peer only means the project runs no servers.

I built 11 packages: the folder state, the share code, the index, the ignore
rules, the watcher, the sync engine, the wire protocol, the libp2p network,
the daemon, the right click menu and the command line. Then I ran 2 folders
on this Mac. The second folder received both files 2 seconds after it joined.
An edit crossed in 1 second. A delete, a rename and a 20 MB file all crossed
in 1 second.

Two things broke. First, `lynx join CODE --dir X` created a folder named
`--dir`, because Go's flag package stops reading flags at the first word. I
now parse flags before and after the words. Second, both peers dialled each
other at the same time, so each connection replaced the other and the peers
flapped once per second. The first session now wins and the second one
closes. The log went from 1 connect and 1 disconnect every second to 1
connect and 0 disconnects in 12 seconds.

I also wrote the macOS Quick Action. `pbs -dump_pboard` lists "Share with
Lynx", and running the action shared a real folder and put the code on the
clipboard.

What I did not prove: the internet path, Windows and Linux. The code cross
builds for all 5 targets and the registry and desktop files are written, but
no second machine ran them. STATUS.md marks those rows 🚧, not ✅.

Evidence: `./verify/verify.sh` green — format, vet, build, 5 cross builds,
4 test packages.

## 2026-09-16 — The folder answers questions

This week I shipped roadmap item 1: talk to the folder. `lynx chat` in a
shared folder answers a question about the files in it.

The chat builds a description of the folder: every file name with its size,
then the text of the files, newest first, up to 200 KB. It sends the
description with the question and writes the answer.

There are 2 providers. The `anthropic` provider streams an answer from
Claude. The `local` provider answers with a word search over the same
description and needs no key, no account and no network. The provider is the
second seam in the project, next to discovery.

The `local` provider started as a way to test the chat without a key. It
turned into a feature. A person with no API key runs `lynx chat "where is the
budget"` and gets the file and the matching lines. The gate uses it too, so
the health check runs offline and proves the real thing: the folder reaches
the provider and the answer names real files.

I did not prove the Claude path. This computer holds no API key, so no
request left it. The code compiles, streams, marks the folder description for
caching, and falls back to a second model when the first one declines. None
of that is proven until a person with a key runs it. STATUS.md marks that row
🚧 and the roadmap holds it as item 4.

One thing I changed from the plan: the item promised a `--once` flag. The
command takes the question as plain words instead, because one way to ask is
enough. ROADMAP.md records the change.

Evidence: `./verify/verify.sh` green, including the new `verify/chat-gate.sh`
with 4 cases.
