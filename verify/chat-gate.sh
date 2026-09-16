#!/usr/bin/env bash
# Proves the promise of the chat: "lynx chat" answers a question about the
# files and the answer names a real file from the folder.
set -euo pipefail
cd "$(dirname "$0")/.."

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

go build -o "$work/lynx" ./cmd/lynx

folder="$work/folder"
mkdir -p "$folder/notes"
printf '# Weekend trip\nWe drive to the coast on Friday.\n' > "$folder/README.md"
printf 'Budget for the trip\n- hotel: 240 euro\n- fuel: 60 euro\n' > "$folder/notes/budget.md"
"$work/lynx" share --dir "$folder" --no-start --code-only > /dev/null

fail=0
check() { # check <name> <expected text> <output>
  if printf '%s' "$3" | grep -qF "$2"; then
    echo "ok   $1"
  else
    echo "FAIL $1: the answer does not hold '$2'"
    printf '%s\n' "$3" | sed 's/^/     /'
    fail=1
  fi
}

# The provider is the local word search, so the gate needs no API key and no
# network. It proves the folder reaches the provider and the answer names
# real files.
export LYNX_CHAT_PROVIDER=local

check "lists the folder" "notes/budget.md" \
  "$("$work/lynx" chat --dir "$folder" "what is in this folder?")"

check "finds the file that answers" "notes/budget.md" \
  "$("$work/lynx" chat --dir "$folder" "how much is the hotel?")"

check "does not invent a file" "No file" \
  "$("$work/lynx" chat --dir "$folder" "where is the submarine?")"

# With the Claude provider and no key, the chat refuses with a sentence that
# says what to do. It never falls back silently.
out=$(LYNX_CHAT_PROVIDER=anthropic ANTHROPIC_API_KEY= "$work/lynx" chat --dir "$folder" "hello" 2>&1 || true)
check "asks for a key" "API key" "$out"

[ "$fail" -eq 0 ] || exit 1
echo "chat gate: passed"
