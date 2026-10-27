//go:build darwin

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
			out = append(out, hotkey.ModOption)
		case "Shift":
			out = append(out, hotkey.ModShift)
		case "Super":
			out = append(out, hotkey.ModCmd)
		default:
			return nil, fmt.Errorf("unsupported modifier %q", m)
		}
	}
	return out, nil
}

// darwinKeycodes maps canonical keys to Carbon virtual keycodes.
var darwinKeycodes = map[string]hotkey.Key{
	"A": hotkey.KeyA, "B": hotkey.KeyB, "C": hotkey.KeyC,
	"D": hotkey.KeyD, "E": hotkey.KeyE, "F": hotkey.KeyF,
	"G": hotkey.KeyG, "H": hotkey.KeyH, "I": hotkey.KeyI,
	"J": hotkey.KeyJ, "K": hotkey.KeyK, "L": hotkey.KeyL,
	"M": hotkey.KeyM, "N": hotkey.KeyN, "O": hotkey.KeyO,
	"P": hotkey.KeyP, "Q": hotkey.KeyQ, "R": hotkey.KeyR,
	"S": hotkey.KeyS, "T": hotkey.KeyT, "U": hotkey.KeyU,
	"V": hotkey.KeyV, "W": hotkey.KeyW, "X": hotkey.KeyX,
	"Y": hotkey.KeyY, "Z": hotkey.KeyZ,
	"0": hotkey.Key0, "1": hotkey.Key1, "2": hotkey.Key2,
	"3": hotkey.Key3, "4": hotkey.Key4, "5": hotkey.Key5,
	"6": hotkey.Key6, "7": hotkey.Key7, "8": hotkey.Key8,
	"9":     hotkey.Key9,
	"Space": hotkey.KeySpace, "Enter": hotkey.KeyReturn,
	"Esc": hotkey.KeyEscape, "Delete": hotkey.KeyDelete,
	"Tab": hotkey.KeyTab, "Up": hotkey.KeyUp, "Down": hotkey.KeyDown,
	"Left": hotkey.KeyLeft, "Right": hotkey.KeyRight,
	"F1": hotkey.KeyF1, "F2": hotkey.KeyF2, "F3": hotkey.KeyF3,
	"F4": hotkey.KeyF4, "F5": hotkey.KeyF5, "F6": hotkey.KeyF6,
	"F7": hotkey.KeyF7, "F8": hotkey.KeyF8, "F9": hotkey.KeyF9,
	"F10": hotkey.KeyF10, "F11": hotkey.KeyF11, "F12": hotkey.KeyF12,
	"F13": hotkey.KeyF13, "F14": hotkey.KeyF14, "F15": hotkey.KeyF15,
	"F16": hotkey.KeyF16, "F17": hotkey.KeyF17, "F18": hotkey.KeyF18,
	"F19": hotkey.KeyF19, "F20": hotkey.KeyF20,
}

func mapKey(key string) (hotkey.Key, error) {
	if k, ok := darwinKeycodes[key]; ok {
		return k, nil
	}
	return 0, fmt.Errorf("unsupported key %q on macOS", key)
}
