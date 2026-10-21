#!/usr/bin/env node
// Raycast Script Command: Scrub Clipboard (PII)
// @raycast.title Scrub Clipboard PII
// @raycast.mode silent
// @raycast.packageName Safe Past
// Requires: safe-past binary on PATH (make build / brew / gh release).
const { execFileSync } = require("child_process");
try {
  const out = execFileSync("safe-past", ["--clipboard", "--copy", "--stats"], { encoding: "utf8", timeout: 15000 });
  console.log("Clipboard scrubbed ✓ " + (out || "").trim());
} catch (e) {
  console.log("Scrub failed: " + (e.message || e));
}
