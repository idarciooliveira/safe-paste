//go:build darwin

package autostart

import (
	"strings"
	"testing"
)

func TestPlistEscapesSpecialChars(t *testing.T) {
	exe := "/Users/me/Tools & Utilities/safe-past-daemon"
	out := plistFile(exe)
	if strings.Contains(out, "Tools & Utilities") {
		t.Fatalf("raw & leaked into plist:\n%s", out)
	}
	if !strings.Contains(out, "Tools &amp; Utilities") {
		t.Fatalf("escaped path missing:\n%s", out)
	}
	if got := xmlEscape(exe); !strings.Contains(out, got) {
		t.Fatalf("escaped exe %q not in plist:\n%s", got, out)
	}
}

func TestPlistPreservesCustomConfig(t *testing.T) {
	out := plistFile("/Applications/safe-past-daemon", "--config", "/Users/me/my config.json")
	if !strings.Contains(out, "--config") || !strings.Contains(out, "my config.json") {
		t.Fatalf("plist must preserve --config:\n%s", out)
	}
}
