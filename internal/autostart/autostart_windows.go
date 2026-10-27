//go:build windows

package autostart

import (
	"fmt"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// Label is the Run-key value name.
const Label = "safe-past"

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

// Path reports where the login entry lives (for Settings UI display).
func Path() string {
	return `HKCU\` + runKey + `\` + Label
}

// Enabled reports whether the Run value exists.
func Enabled() (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false, err
	}
	defer k.Close()
	_, _, err = k.GetStringValue(Label)
	if err == nil {
		return true, nil
	}
	if err == registry.ErrNotExist {
		return false, nil
	}
	return false, err
}

// SetEnabled creates or removes the Run value.
func SetEnabled(enable bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !enable {
		_ = k.DeleteValue(Label) // idempotent
		return nil
	}
	v, err := loginValue()
	if err != nil {
		return err
	}
	if err := k.SetStringValue(Label, v); err != nil {
		return fmt.Errorf("set Run value: %w", err)
	}
	return nil
}

// loginValue builds the Run-key command: quoted exe + preserved args.
func loginValue() (string, error) {
	exe, args, err := LoginCommand()
	if err != nil {
		return "", err
	}
	parts := []string{quoteWindowsArg(exe)}
	for _, a := range args {
		parts = append(parts, quoteWindowsArg(a))
	}
	return strings.Join(parts, " "), nil
}

// IsCurrent reports whether the Run value matches the current executable
// + preserved args.
func IsCurrent() (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false, err
	}
	defer k.Close()
	v, _, err := k.GetStringValue(Label)
	if err != nil {
		if err == registry.ErrNotExist {
			return false, nil
		}
		return false, err
	}
	want, err := loginValue()
	if err != nil {
		return false, err
	}
	return v == want, nil
}
