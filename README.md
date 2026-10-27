# safe-past

Copy prod logs, press a hotkey, paste the clean version into any LLM. No app to open, no browser, 100% offline.

`safe-past` scrubs PII and secrets from your clipboard (or stdin/files) and puts the clean text back — same value always maps to the same `[TYPE_N]` token, so logs stay correlatable after redaction.

## How it works

```
copy logs → Ctrl+Shift+V → paste clean version
```

Under the hood the hotkey just runs:

```
safe-past --clipboard --copy --stats
```

## Install

**Windows (prebuilt):**

```powershell
# download safe-past-windows-amd64.exe from Releases, rename to safe-past.exe
mkdir C:\Tools\safe-past
# move safe-past.exe there, then:
[Environment]::SetEnvironmentVariable("Path", $env:Path + ";C:\Tools\safe-past", "User")
safe-past --version
```

**macOS / Linux (from source):**

```bash
go install github.com/idarciooliveira/safe-past/cmd/safe-past@latest
# or:
git clone https://github.com/idarciooliveira/safe-past && cd safe-past && make build
sudo cp bin/safe-past /usr/local/bin/
```

## Global hotkey

**Windows — option A, no extra software (recommended):**

1. Create `C:\Tools\safe-past\scrub.bat`:
   ```bat
   @"C:\Tools\safe-past\safe-past.exe" --clipboard --copy --stats
   ```
2. Right-click it → Create shortcut → shortcut Properties → Shortcut key → `Ctrl+Shift+V`.

**Windows — option B, AutoHotkey v2:**

Install AHK v2, double-click `windows/safe-past-hotkey.ahk` (`Ctrl+Shift+V`).

**macOS:** Raycast → import `raycast/scrub-clipboard.js` as a Script Command, bind a hotkey.

**Linux (GNOME):** Settings → Keyboard → Custom Shortcut → `safe-past --clipboard --copy --stats`.

## Usage

```bash
safe-past --clipboard --copy --stats          # scrub clipboard in place
safe-past --list-rules                         # show all detectors
safe-past --pattern 'ACME-\d+' < big.log > clean.log --stats
safe-past --config extra-patterns.txt --clipboard --copy
echo "db 192.168.1.10 user bob@example.com" | safe-past --stats
```

`--config` is one regex per line, `#` = comment. See `examples/extra-patterns.txt`.

## Output modes (Presidio-style operators)

| Mode | Flag | `192.168.1.10` becomes | Notes |
|---|---|---|---|
| `replace` (default) | — | `[IP_1]` | same value → same token, per run |
| `mask` | `--mode mask` | `********1.10` | keeps last `--mask-keep` (4) chars with `--mask-char` (`*`); tails stay debuggable |
| `hash` | `--mode hash` | `4d2bbd81e6afb78a` | salted sha256, `--salt` (default `safe-past-v1`), `--hash-len` (16, `<=0` = full 64); stable across runs |
| `presidio` | `--mode presidio` | `<IP>` | `<TYPE>` tags, drop-in for Presidio pipelines |

Per-entity override (repeatable): `--entity-mode IBAN:hash --entity-mode PHONE_AO:mask`.

## What it redacts

IPv4/IPv6, MAC, emails, phones (US + Angola AO: `9XX XXX XXX`, `+244…`, `00244…`, `222` landlines), Angola IBAN (mod-97 checked), BI + NIF, AWS / GitHub / Slack / Stripe keys, JWT / Bearer, `password= / api_key:` values, `user:pass@host` URLs, private keys, credit cards (Luhn-checked), SSN — all offline regex, no network calls.

Same input → same token (`10.0.0.1` always becomes `[IP_1]`), so stack traces and joins still make sense.

## Desktop UI (Wails)

Two-pane Input → Output window in `ui/` (Go backend, HTML/CSS frontend):

```bash
cd ui && go mod tidy
go test ./...        # bridge tests, headless-safe
wails dev            # run (install wails CLI first)
wails build          # platform installer/binary
```

Features: live redact as you type, operator select (replace/mask/hash/presidio tags), per-detector toggles (`--disable` equivalent), paste-from-clipboard, one-click copy. Open `ui/frontend/index.html?demo=1` in any browser for a static look preview.

## Dev

```bash
go test ./...
make build        # bin/safe-past
make build-all    # windows + darwin-arm64 + linux in dist/
```

Layout: `internal/redact/` engine · `cmd/safe-past/` CLI · `windows/` AHK hotkey · `raycast/` script command · `examples/` custom patterns.

## Disclaimer

Regex redaction is best-effort. Always spot-check before pasting sensitive logs anywhere.
