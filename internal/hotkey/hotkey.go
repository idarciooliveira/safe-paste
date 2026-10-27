// Package hotkey parses human hotkey strings into a canonical form.
//
// Canonical modifiers: Ctrl, Alt, Shift, Super. Aliases (Cmd, Command,
// Meta, Win, Windows -> Super; Option -> Alt; Control -> Ctrl) keep one
// portable spelling in the config file; the daemon maps them per-OS onto
// golang.design/x/hotkey modifiers (ModCmd on darwin, ModWin on windows,
// Mod4 on X11). The parser itself is pure Go so GUI code and tests run
// headless — no display or cgo required.
package hotkey

import (
	"fmt"
	"sort"
	"strings"
)

// Canonical modifier names in display order.
const (
	ModCtrl  = "Ctrl"
	ModAlt   = "Alt"
	ModShift = "Shift"
	ModSuper = "Super"
)

var modOrder = map[string]int{ModCtrl: 0, ModAlt: 1, ModShift: 2, ModSuper: 3}

var modAliases = map[string]string{
	"CTRL": ModCtrl, "CONTROL": ModCtrl,
	"ALT": ModAlt, "OPTION": ModAlt, "OPT": ModAlt,
	"SHIFT": ModShift, "SHFT": ModShift,
	"SUPER": ModSuper, "CMD": ModSuper, "COMMAND": ModSuper,
	"META": ModSuper, "WIN": ModSuper, "WINDOWS": ModSuper,
}

// Hotkey is a parsed combination: at least one modifier plus one key.
type Hotkey struct {
	Mods []string // canonical, sorted in display order
	Key  string   // canonical: "V", "F5", "Space", ...
}

// String renders canonical form, e.g. "Ctrl+Shift+V".
func (h Hotkey) String() string {
	return strings.Join(append(append([]string{}, h.Mods...), h.Key), "+")
}

// Display renders with OS idioms: Super shows as Cmd on darwin, Win elsewhere.
func (h Hotkey) Display(macOS bool) string {
	mods := append([]string{}, h.Mods...)
	for i, m := range mods {
		if m == ModSuper {
			if macOS {
				mods[i] = "Cmd"
			} else {
				mods[i] = "Win"
			}
		}
	}
	return strings.Join(append(mods, h.Key), "+")
}

var namedKeys = map[string]string{
	"SPACE": "Space", "TAB": "Tab",
	"ENTER": "Enter", "RETURN": "Enter",
	"ESC": "Esc", "ESCAPE": "Esc",
	"DELETE": "Delete", "DEL": "Delete", "BACKSPACE": "Backspace",
	"UP": "Up", "DOWN": "Down", "LEFT": "Left", "RIGHT": "Right",
	"HOME": "Home", "END": "End", "PAGEUP": "PageUp", "PAGEDOWN": "PageDown",
	"INSERT": "Insert", "INS": "Insert",
	"PLUS": "Plus", "MINUS": "Minus", "COMMA": "Comma", "PERIOD": "Period", "DOT": "Period",
	"SLASH": "Slash", "SEMICOLON": "Semicolon",
}

// Parse turns "Ctrl+Shift+V" (also "ctrl + shift + v", "Cmd-Shift-V") into
// a canonical Hotkey. Exactly one non-modifier key is required.
func Parse(s string) (Hotkey, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Hotkey{}, fmt.Errorf("empty hotkey (example: Ctrl+Shift+V)")
	}
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '+' || r == '-' })
	var mods []string
	var key string
	seenMod := map[string]bool{}
	for _, p := range parts {
		up := strings.ToUpper(strings.TrimSpace(p))
		if up == "" {
			continue
		}
		if m, ok := modAliases[up]; ok {
			if !seenMod[m] {
				seenMod[m] = true
				mods = append(mods, m)
			}
			continue
		}
		if key != "" {
			return Hotkey{}, fmt.Errorf("only one key allowed, got %q and %q", key, p)
		}
		k, err := canonKey(up)
		if err != nil {
			return Hotkey{}, fmt.Errorf("bad hotkey %q: %w", s, err)
		}
		key = k
	}
	if len(mods) == 0 {
		return Hotkey{}, fmt.Errorf("hotkey %q needs at least one modifier (Ctrl/Alt/Shift/Super)", s)
	}
	if key == "" {
		return Hotkey{}, fmt.Errorf("hotkey %q needs a key (example: Ctrl+Shift+V)", s)
	}
	sort.Slice(mods, func(i, j int) bool { return modOrder[mods[i]] < modOrder[mods[j]] })
	return Hotkey{Mods: mods, Key: key}, nil
}

// Normalize parses and re-renders s in canonical form.
func Normalize(s string) (string, error) {
	h, err := Parse(s)
	if err != nil {
		return "", err
	}
	return h.String(), nil
}

func canonKey(up string) (string, error) {
	if len(up) == 1 {
		c := up[0]
		if c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' {
			return up, nil
		}
		return "", fmt.Errorf("unsupported key %q", up)
	}
	if n, ok := namedKeys[up]; ok {
		return n, nil
	}
	if len(up) > 1 && (up[0] == 'F' || up[0] == 'f') {
		n := 0
		for _, c := range up[1:] {
			if c < '0' || c > '9' {
				return "", fmt.Errorf("unsupported key %q", up)
			}
			n = n*10 + int(c-'0')
		}
		if n >= 1 && n <= 24 {
			return fmt.Sprintf("F%d", n), nil
		}
	}
	return "", fmt.Errorf("unsupported key %q", up)
}
