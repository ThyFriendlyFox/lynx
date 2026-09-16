# Chat

The chat answers questions about the files in one folder. `lynx chat` builds
a description of the folder, sends it with the question, and writes the
answer. Lynx has 2 chat providers.

A provider is the second seam in Lynx, next to `docs/DISCOVERY.md`. Select
one with the `provider` field in the chat settings.

## local

The local provider answers with a word search over the folder description.
It needs no API key, no account and no network. It names the files whose
name or text holds the words of the question, with the matching lines.

This provider is the default when no API key exists. It is also the provider
that the gate uses, so the health check runs offline.

```json
{ "provider": "local" }
```

## anthropic

The anthropic provider sends the question and the folder description to
Claude and streams the answer back. It is the default when an API key
exists. Set the key with `lynx chat --set-key <key>` or in the environment
variable `ANTHROPIC_API_KEY`.

```json
{ "provider": "anthropic", "model": "claude-opus-5", "effort": "low" }
```

The folder description is marked for caching, so the second question in a
session costs much less than the first one. A request that the model
declines gets a server side fallback to a second model; a request that still
fails reports the reason in one sentence.

## What the model receives

| Part | Content |
|---|---|
| Rules | Answer from the folder. Name the files. Say "I do not know". Never invent a file name. |
| File list | Every file with its size, up to 500 names. |
| File content | The text files, newest first, up to 32 KB per file and 200 KB in total. |
| Question | The question, with the earlier turns of this session. |

Lynx never sends a file that is not text, and never sends a file that
`.lynxignore` hides.

## Writing a new provider

A provider implements 2 methods in `internal/chat`:

```go
Name() string
Answer(ctx context.Context, req Request, w io.Writer) (string, error)
```

`Answer` writes the answer to `w` while it arrives and returns the whole
answer. A pull request with a new provider carries 4 things:

1. The implementation in `internal/chat`.
2. Its section in this file.
3. Its fields in `docs/CONFIGURATION.md`.
4. A case in `verify/chat-gate.sh`.
