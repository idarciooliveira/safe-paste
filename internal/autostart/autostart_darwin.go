//go:build darwin

package autostart

import (
	"fmt"
	"os"
	"path/filepath"
)

// Label is the launchd job label.
const Label = "com.safepast.daemon"

func unitPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", Label+".plist"), nil
}

// Path reports where the login entry lives (for Settings UI display).
func Path() string {
	p, err := unitPath()
	if err != nil {
		return filepath.Join("~/Library/LaunchAgents", Label+".plist")
	}
	return p
}

func plistFile(exe string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>` + "\n" +
		`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n" +
		`<plist version="1.0"><dict>` + "\n" +
		"  <key>Label</key><string>" + Label + "</string>\n" +
		"  <key>ProgramArguments</key><array><string>" + exe + "</string></array>\n" +
		"  <key>RunAtLoad</key><true/>\n" +
		"  <key>KeepAlive</key><false/>\n" +
		"</dict></plist>\n"
}

// Enabled reports whether the LaunchAgent exists.
func Enabled() (bool, error) {
	p, err := unitPath()
	if err != nil {
		return false, err
	}
	_, err = os.Stat(p)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// SetEnabled creates or removes the LaunchAgent plist.
func SetEnabled(enable bool) error {
	p, err := unitPath()
	if err != nil {
		return err
	}
	if !enable {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", p, err)
		}
		return nil
	}
	exe, err := ExePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(p, []byte(plistFile(exe)), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", p, err)
	}
	return nil
}
