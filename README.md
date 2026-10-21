# safe-past

Clipboard-first PII scrubber for pasting prod logs into LLMs.

**Flow:** copy logs → press hotkey → paste clean version. Nothing to open, no browser, works offline.

## Quick start (Windows)

```powershell
make build
# copy dist binary to C:\Tools\safe-past\safe-past.exe, add that dir to PATH
safe-past --clipboard --copy --stats   # test once in terminal

# Global hotkey (pick one, no code changes needed):
# A) AutoHotkey: double-click windows\safe-past-hotkey.ahk (Ctrl+Shift+V)
# B) PowerToys Keyboard Manager: bind a key to the same command
```

## macOS / Linux

```bash
make build && sudo cp bin/safe-past /usr/local/bin/
safe-past --clipboard --copy --stats
# mac: Raycast → import raycast/scrub-clipboard.js as Script Command, bind hotkey
# linux (GNOME): Settings → Keyboard → Custom Shortcut → same command
```

## CLI

```
safe-past --clipboard --copy --stats
safe-past --list-rules
safe-past --pattern 'ACME-\d+' < big.log > clean.log --stats
safe-past --config extra-patterns.txt --clipboard --copy
```

`--config` = one regex per line, `#` = comment. Example in `examples/extra-patterns.txt`.

## What it redacts (regex-only, offline)

IPv4/IPv6, emails, phones, AWS/GitHub/Slack/Stripe keys, JWT/Bearer,
`password= / api_key:` values, `user:pass@host` URLs, private keys,
credit cards (Luhn-checked), SSN. Same value → same `[TYPE_N]` token so
logs stay correlatable.

## Big files

Pipes straight through, no clipboard needed.
1.4MB / 60k secrets scrubbed in ~0.7s locally.

## Layout

- `internal/redact/` — engine + tests (`go test ./...`)
- `cmd/safe-past/` — CLI
- `windows/` — AHK hotkey, `raycast/` — Raycast script command
