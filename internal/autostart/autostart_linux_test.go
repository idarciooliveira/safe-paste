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
