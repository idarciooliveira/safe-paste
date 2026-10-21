# prompt-parse

Clipboard-first PII scrubber for pasting prod logs into LLMs.

**Flow:** copy logs → press hotkey → paste clean version. Nothing to open, no browser, works offline.

## Quick start (Windows, your current OS)

```powershell
go build -o prompt-parse.exe ./cmd/prompt-parse
# copy it to C:\Tools\prompt-parse\prompt-parse.exe, add that dir to PATH
.\prompt-parse.exe --clipboard --copy --stats   # test once in terminal

# Global hotkey (pick one, no code changes needed):
# A) AutoHotkey: double-click windows\prompt-parse-hotkey.ahk (Ctrl+Shift+V)
# B) PowerToys Run / Keyboard Manager: bind a key to the same command
```

## macOS / Linux

```bash
go build -o prompt-parse ./cmd/prompt-parse && sudo mv prompt-parse /usr/local/bin/
prompt-parse --clipboard --copy --stats
# mac: Raycast → import raycast/scrub-clipboard.js as Script Command, bind hotkey
# linux (GNOME): Settings → Keyboard → Custom Shortcut → same command
```

## What it redacts (regex-only, offline)

IPv4/IPv6, emails, phones, AWS/G GitHub/Slack/Stripe keys, JWT/Bearer,
`password= / api_key:` values, `user:pass@host` URLs, private keys,
credit cards (Luhn-checked), SSN. Same value → same `[TYPE_N]` token so
logs stay correlatable. Custom: `--pattern 'COMPANY-\d+'`.

## Big files

Pipes straight through, no clipboard needed:
`prompt-parse < 200mb.log > clean.log --stats`
~GB/s-class throughput (pure regex, streams in memory per file arg).

## Layout

- `internal/redact/` — engine + tests (`go test ./...`)
- `cmd/prompt-parse/` — CLI (`--clipboard --copy --stats --pattern --max`)
- `windows/` — AHK hotkey, `raycast/` — Raycast script command
