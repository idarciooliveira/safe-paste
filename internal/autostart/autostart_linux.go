//go:build linux

package autostart

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Label is the .desktop file name.
const Label = "safe-past-daemon.desktop"

// unitPath returns ~/.config/autostart/safe-past-daemon.desktop,
// honoring XDG_CONFIG_HOME via os.UserConfigDir.
func unitPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "autostart", Label), nil
}

// Path reports where the login entry lives (for Settings UI display).
func Path() string {
	p, err := unitPath()
	if err != nil {
		return filepath.Join("~/.config", "autostart", Label)
	}
	return p
}

func desktopFile(exe string, args ...string) string {
	parts := []string{quoteDesktopArg(exe)}
	for _, a := range args {
		parts = append(parts, quoteDesktopArg(a))
	}
	return "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=safe-past\n" +
		"Comment=Scrub PII from the clipboard with a global hotkey\n" +
		"Exec=" + strings.Join(parts, " ") + "\n" +
		"Terminal=false\n" +
		"X-GNOME-Autostart-enabled=true\n"
}

// Enabled reports whether the autostart entry exists.
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

// SetEnabled creates or removes the autostart entry.
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
	exe, args, err := LoginCommand()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(p, []byte(desktopFile(exe, args...)), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", p, err)
	}
	return nil
}

// IsCurrent reports whether the existing login entry matches the current
// executable + preserved args. A moved binary or a newly custom --config
// path makes it false so the caller rewrites the entry.
func IsCurrent() (bool, error) {
	p, err := unitPath()
	if err != nil {
		return false, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	exe, args, err := LoginCommand()
	if err != nil {
		return false, err
	}
	return string(b) == desktopFile(exe, args...), nil
}
