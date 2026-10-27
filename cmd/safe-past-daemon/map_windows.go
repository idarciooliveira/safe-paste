//go:build windows

package main

import (
	"fmt"

	"golang.design/x/hotkey"
)

func mapMods(mods []string) ([]hotkey.Modifier, error) {
	var out []hotkey.Modifier
	for _, m := range mods {
		switch m {
		case "Ctrl":
			out = append(out, hotkey.ModCtrl)
		case "Alt":
			out = append(out, hotkey.ModAlt)
		case "Shift":
			out = append(out, hotkey.ModShift)
		case "Super":
			out = append(out, hotkey.ModWin)
		default:
			return nil, fmt.Errorf("unsupported modifier %q", m)
		}
	}
	return out, nil
}

func mapKey(key string) (hotkey.Key, error) {
	if len(key) == 1 {
		c := key[0]
		if c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' {
			return hotkey.Key(c), nil // VK codes match ASCII here
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
	case "Backspace":
		return hotkey.Key(0x08), nil
	case "Insert":
		return hotkey.Key(0x2D), nil
	case "Home":
		return hotkey.Key(0x24), nil
	case "End":
		return hotkey.Key(0x23), nil
	case "PageUp":
		return hotkey.Key(0x21), nil
	case "PageDown":
		return hotkey.Key(0x22), nil
	}
	var n int
	if _, err := fmt.Sscanf(key, "F%d", &n); err == nil && n >= 1 && n <= 20 {
		return hotkey.Key(0x70 + n - 1), nil
	}
	return 0, fmt.Errorf("unsupported key %q on Windows", key)
}
