package autostart

import (
	"strings"
	"testing"
)

func TestQuoteDesktopArg(t *testing.T) {
	if got := quoteDesktopArg("/usr/bin/safe-past-daemon"); got != "/usr/bin/safe-past-daemon" {
		t.Fatalf("bare path should pass through: %q", got)
	}
	got := quoteDesktopArg("/home/me/My Apps/safe-past-daemon")
	if got != `"/home/me/My Apps/safe-past-daemon"` {
		t.Fatalf("spaced path should be quoted: %q", got)
	}
	if !strings.HasPrefix(got, `"`) || !strings.HasSuffix(got, `"`) {
		t.Fatalf("not quoted: %q", got)
	}
	// Embedded quote must be escaped, not terminate the token.
	got = quoteDesktopArg(`/home/me/We"ird/app`)
	if !strings.HasPrefix(got, `"`) || !strings.Contains(got, `\"`) {
		t.Fatalf("quote not escaped: %q", got)
	}
}

func TestXMLEscape(t *testing.T) {
	in := "/Users/me/Tools & Utilities/safe-past-daemon"
	got := xmlEscape(in)
	if strings.Contains(got, "& ") && !strings.Contains(got, "&amp;") {
		t.Fatalf("ampersand not escaped: %q", got)
	}
	if strings.Contains(got, "Tools & Utilities") {
		t.Fatalf("raw & leaked: %q", got)
	}
	for _, tc := range []struct{ in, want string }{
		{"a<b", "a&lt;b"},
		{`a"b`, "a&#34;b"},
		{"a'b", "a&#39;b"},
	} {
		if got := xmlEscape(tc.in); got != tc.want {
			t.Errorf("xmlEscape(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDaemonArgsPreservesCustomConfig(t *testing.T) {
	t.Setenv(EnvConfigFile, "")
	if args := DaemonArgs(); len(args) != 0 {
		t.Fatalf("default config should add no args: %v", args)
	}
	t.Setenv(EnvConfigFile, "/tmp/custom config.json")
	args := DaemonArgs()
	if len(args) != 2 || args[0] != "--config" || args[1] != "/tmp/custom config.json" {
		t.Fatalf("custom config not preserved: %v", args)
	}
}

func TestQuoteWindowsArg(t *testing.T) {
	if got := quoteWindowsArg(`C:\Tools\app.exe`); got != `C:\Tools\app.exe` {
		t.Fatalf("bare path should pass through: %q", got)
	}
	if got := quoteWindowsArg(`C:\My Tools\app.exe`); got != `"C:\My Tools\app.exe"` {
		t.Fatalf("spaced path should be quoted: %q", got)
	}
}
