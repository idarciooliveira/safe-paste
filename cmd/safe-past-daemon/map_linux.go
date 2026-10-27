//go:build linux

package main

import (
	"fmt"
	"strings"

	"golang.design/x/hotkey"
)

func mapMods(mods []string) ([]hotkey.Modifier, error) {
	var out []hotkey.Modifier
	for _, m := range mods {
		switch m {
		case "Ctrl":
			out = append(out, hotkey.ModCtrl)
		case "Alt":
			out = append(out, hotkey.Mod1)
		case "Shift":
			out = append(out, hotkey.ModShift)
		case "Super":
			out = append(out, hotkey.Mod4)
		default:
			return nil, fmt.Errorf("unsupported modifier %q", m)
		}
	}
	return out, nil
}

func mapKey(key string) (hotkey.Key, error) {
	if len(key) == 1 {
		lower := strings.ToLower(key)
		c := lower[0]
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' {
			return hotkey.Key(c), nil // X11 keysyms match ASCII here
		}
		return 0, fmt.Errorf("unsupported key %q", key)
	}
	switch key {
	case "Space":
		return hotkey.KeySpace, nil
	case "Enter":
		return hotkey.KeyReturn, nil
	case "Esc":
		return hotkey.KeyEscape, nil
	case "Delete":
		return hotkey.KeyDelete, nil
	case "Tab":
		return hotkey.KeyTab, nil
	case "Up":
		return hotkey.KeyUp, nil
	case "Down":
		return hotkey.KeyDown, nil
	case "Left":
		return hotkey.KeyLeft, nil
	case "Right":
		return hotkey.KeyRight, nil
	}
	var n int
	if _, err := fmt.Sscanf(key, "F%d", &n); err == nil && n >= 1 && n <= 20 {
		return hotkey.Key(0xffbe + n - 1), nil
	}
	return 0, fmt.Errorf("unsupported key %q on Linux", key)
}
