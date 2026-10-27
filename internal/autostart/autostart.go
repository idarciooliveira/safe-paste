// Package autostart manages the OS login item for the tray daemon.
package autostart

import (
	"bytes"
	"encoding/xml"
	"os"
	"strings"
)

// EnvExe overrides the daemon executable path (tests).
const EnvExe = "SAFE_PAST_EXE"

// EnvConfigFile mirrors config.EnvFile without importing config
// (avoids a dependency; both must stay "SAFE_PAST_CONFIG_FILE").
const EnvConfigFile = "SAFE_PAST_CONFIG_FILE"

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

// DaemonArgs returns extra login-command args needed to preserve the
// current settings. When a custom config file is in use
// (--config / SAFE_PAST_CONFIG_FILE), the login entry must carry it,
// otherwise the next login reads the default config instead.
func DaemonArgs() []string {
	if p := strings.TrimSpace(os.Getenv(EnvConfigFile)); p != "" {
		return []string{"--config", p}
	}
	return nil
}

// LoginCommand resolves the current executable + preserved args.
func LoginCommand() (string, []string, error) {
	exe, err := ExePath()
	if err != nil {
		return "", nil, err
	}
	return exe, DaemonArgs(), nil
}

// quoteDesktopArg quotes one Exec token per the Desktop Entry spec:
// bare tokens pass through, otherwise double-quote and escape \ " ` $.
func quoteDesktopArg(s string) string {
	if s == "" {
		return `""`
	}
	if !strings.ContainsAny(s, " \t\n\"'\\><~|&;$*?#()`") {
		return s
	}
	r := strings.ReplaceAll(s, `\`, `\\`)
	r = strings.ReplaceAll(r, `"`, `\"`)
	r = strings.ReplaceAll(r, "`", "\\`")
	r = strings.ReplaceAll(r, `$`, `\$`)
	return `"` + r + `"`
}

// xmlEscape escapes a string for embedding in plist XML.
func xmlEscape(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

// quoteWindowsArg quotes one argv token for the Run-key command line.
func quoteWindowsArg(s string) string {
	if s == "" {
		return `""`
	}
	if !strings.ContainsAny(s, " \t\n\"") {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}
