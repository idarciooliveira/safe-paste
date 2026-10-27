//go:build linux

package autostart

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetEnabledRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(EnvExe, "/usr/bin/safe-past-daemon")
	t.Setenv(EnvConfigFile, "")

	if on, err := Enabled(); err != nil || on {
		t.Fatalf("fresh home should be disabled: %v %v", on, err)
	}
	if err := SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	if on, err := Enabled(); err != nil || !on {
		t.Fatalf("should be enabled: %v %v", on, err)
	}
	p := Path()
	if !strings.HasSuffix(p, "autostart/"+Label) {
		t.Fatalf("bad path %q", p)
	}
	b, _ := os.ReadFile(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "autostart", Label))
	if !strings.Contains(string(b), "Exec=/usr/bin/safe-past-daemon") {
		t.Fatalf("bad desktop file:\n%s", b)
	}
	if cur, err := IsCurrent(); err != nil || !cur {
		t.Fatalf("fresh entry should be current: %v %v", cur, err)
	}
	if err := SetEnabled(false); err != nil {
		t.Fatal(err)
	}
	if on, err := Enabled(); err != nil || on {
		t.Fatalf("should be disabled: %v %v", on, err)
	}
	// Idempotent disable.
	if err := SetEnabled(false); err != nil {
		t.Fatal(err)
	}
}

func TestDesktopFileQuotesSpaces(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(EnvExe, "/home/me/My Apps/safe-past-daemon")
	t.Setenv(EnvConfigFile, "")

	if err := SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "autostart", Label))
	execLine := ""
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "Exec=") {
			execLine = line
		}
	}
	// The spaced path must be one quoted token, not split on words.
	if execLine != `Exec="/home/me/My Apps/safe-past-daemon"` {
		t.Fatalf("Exec line not quoted as one token: %q\nfull file:\n%s", execLine, b)
	}
	if cur, err := IsCurrent(); err != nil || !cur {
		t.Fatalf("quoted entry should be current: %v %v", cur, err)
	}
}

func TestSetEnabledPreservesCustomConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(EnvExe, "/usr/bin/safe-past-daemon")
	t.Setenv(EnvConfigFile, "/home/me/custom config.json")

	if err := SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "autostart", Label))
	if !strings.Contains(string(b), "--config") || !strings.Contains(string(b), "custom config.json") {
		t.Fatalf("login entry must preserve --config:\n%s", b)
	}
	if cur, err := IsCurrent(); err != nil || !cur {
		t.Fatalf("custom-config entry should be current: %v %v", cur, err)
	}
}

func TestIsCurrentDetectsMovedExecutable(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(EnvConfigFile, "")
	t.Setenv(EnvExe, "/opt/v1/safe-past-daemon")
	if err := SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	// Simulate an update that moved the binary.
	t.Setenv(EnvExe, "/opt/v2/safe-past-daemon")
	if cur, err := IsCurrent(); err != nil || cur {
		t.Fatalf("moved binary should not be current: %v %v", cur, err)
	}
}
