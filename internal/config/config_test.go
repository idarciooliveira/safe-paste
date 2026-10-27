package config

import (
	"os"
	"path/filepath"
	"testing"
)

func withTempConfig(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.json")
	t.Setenv(EnvFile, p)
	return p
}

func TestLoadMissingReturnsDefault(t *testing.T) {
	p := withTempConfig(t)
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Hotkey == "" || c.Mode != "replace" || c.MaskKeep != 4 || c.MaskChar != "*" {
		t.Fatalf("bad defaults: %+v", c)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("Load must not create the file")
	}
}

func TestSaveRoundTrip(t *testing.T) {
	withTempConfig(t)
	want := Default()
	want.Mode = "mask"
	want.Disabled = []string{"email", "IP", "email"}
	if err := want.Save(); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != "mask" || len(got.Disabled) != 2 || got.Disabled[0] != "EMAIL" {
		t.Fatalf("round trip failed: %+v", got)
	}
}

func TestLoadRejectsBadMode(t *testing.T) {
	p := withTempConfig(t)
	os.WriteFile(p, []byte(`{"hotkey":"Ctrl+Shift+V","mode":"nope"}`), 0o600)
	if _, err := Load(); err == nil {
		t.Fatal("expected error for bad mode")
	}
}

func TestLoadRejectsGarbage(t *testing.T) {
	p := withTempConfig(t)
	os.WriteFile(p, []byte(`not json`), 0o600)
	if _, err := Load(); err == nil {
		t.Fatal("expected error for garbage")
	}
}

func TestValidateNormalizes(t *testing.T) {
	c := Config{Hotkey: "Ctrl+Shift+V", Mode: "", Disabled: []string{" ip ", "", "IP"}}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if c.Mode != "replace" || len(c.Disabled) != 1 || c.Disabled[0] != "IP" {
		t.Fatalf("not normalized: %+v", c)
	}
}
