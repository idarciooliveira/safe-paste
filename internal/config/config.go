// Package config loads and stores safe-past daemon settings.
//
// The file lives at os.UserConfigDir()/safe-past/config.json, overridable
// with SAFE_PAST_CONFIG_FILE (used by tests and --config). Missing files
// yield defaults; unknown mode/hotkey strings are rejected on load so a
// typo never silently disables the hotkey.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	// EnvFile overrides the config path (tests, --config flag).
	EnvFile = "SAFE_PAST_CONFIG_FILE"

	appDir      = "safe-past"
	fileName    = "config.json"
	filePerm    = 0o600
	dirPerm     = 0o755
	defaultMode = "replace"
)

// Config is the daemon + future GUI settings file.
type Config struct {
	// Hotkey is canonical "Ctrl+Shift+V" (see internal/hotkey).
	Hotkey string `json:"hotkey"`
	// Mode is a redact mode: replace|mask|hash|presidio.
	Mode string `json:"mode"`
	// MaskKeep/MaskChar apply to mask mode.
	MaskKeep int    `json:"maskKeep"`
	MaskChar string `json:"maskChar"`
	// Autostart keeps the OS login item in sync when true.
	Autostart bool `json:"autostart"`
	// Disabled lists detector placeholder kinds to skip (upper-cased).
	Disabled []string `json:"disabled,omitempty"`
	// Extra holds custom regexes, one per entry (# unsupported here).
	Extra []string `json:"extra,omitempty"`
}

// DefaultHotkey is per-OS: Cmd on macOS, Ctrl elsewhere.
func DefaultHotkey() string {
	if runtime.GOOS == "darwin" {
		return "Cmd+Shift+V"
	}
	return "Ctrl+Shift+V"
}

// Default returns usable settings when no file exists.
func Default() Config {
	return Config{
		Hotkey:    DefaultHotkey(),
		Mode:      defaultMode,
		MaskKeep:  4,
		MaskChar:  "*",
		Autostart: true,
	}
}

// Path resolves the config file location.
func Path() (string, error) {
	if p := strings.TrimSpace(os.Getenv(EnvFile)); p != "" {
		return p, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config dir: %w", err)
	}
	return filepath.Join(dir, appDir, fileName), nil
}

// Validate normalizes (upper-cases Disabled) and rejects bad values.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.Hotkey) == "" {
		return fmt.Errorf("hotkey is empty")
	}
	switch c.Mode {
	case "replace", "mask", "hash", "presidio", "":
		if c.Mode == "" {
			c.Mode = defaultMode
		}
	default:
		return fmt.Errorf("unknown mode %q (want replace|mask|hash|presidio)", c.Mode)
	}
	if c.MaskKeep <= 0 {
		c.MaskKeep = 4
	}
	if c.MaskChar == "" {
		c.MaskChar = "*"
	}
	seen := map[string]bool{}
	out := c.Disabled[:0]
	for _, d := range c.Disabled {
		d = strings.ToUpper(strings.TrimSpace(d))
		if d == "" || seen[d] {
			continue
		}
		seen[d] = true
		out = append(out, d)
	}
	c.Disabled = out
	return nil
}

// Load reads Path, returning defaults when the file does not exist.
func Load() (Config, error) {
	p, err := Path()
	if err != nil {
		return Config{}, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return Config{}, fmt.Errorf("read %s: %w", p, err)
	}
	var c Config
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", p, err)
	}
	if err := c.Validate(); err != nil {
		return Config{}, fmt.Errorf("invalid %s: %w", p, err)
	}
	return c, nil
}

// Save writes c to Path, creating parent dirs.
func (c Config) Save() error {
	if err := c.Validate(); err != nil {
		return err
	}
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), dirPerm); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), filePerm); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, p); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}
