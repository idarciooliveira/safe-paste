// Package autostart manages the OS login item for the tray daemon.
package autostart

import (
	"os"
	"strings"
)

// EnvExe overrides the daemon executable path (tests).
const EnvExe = "SAFE_PAST_EXE"

// ExePath resolves the daemon binary for login-item entries.
func ExePath() (string, error) {
	if p := strings.TrimSpace(os.Getenv(EnvExe)); p != "" {
		return p, nil
	}
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	return p, nil
}
