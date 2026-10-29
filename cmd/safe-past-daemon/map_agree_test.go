package main

import (
	"testing"

	ihotkey "github.com/idarciooliveira/safe-past/internal/hotkey"
)

// TestParserAndOSMapsAgree ensures every key the pure parser accepts also
// registers on the current OS. A parser-only key bricks startup after a
// restart, so the two must stay in lockstep.
func TestParserAndOSMapsAgree(t *testing.T) {
	keys := []string{
		"A", "Z", "V", "0", "9",
		"Space", "Tab", "Enter", "Esc", "Delete", "Backspace",
		"Up", "Down", "Left", "Right",
		"Home", "End", "PageUp", "PageDown", "Insert",
		"Plus", "Minus", "Comma", "Period", "Slash", "Semicolon",
		"F1", "F5", "F12", "F20",
	}
	for _, k := range keys {
		spec := "Ctrl+" + k
		h, err := ihotkey.Parse(spec)
		if err != nil {
			t.Errorf("parser rejects %q: %v", spec, err)
			continue
		}
		if _, err := mapMods(h.Mods); err != nil {
			t.Errorf("mapMods rejects %q mods %v: %v", spec, h.Mods, err)
		}
		if _, err := mapKey(h.Key); err != nil {
			t.Errorf("mapKey rejects %q (canonical %q): %v", spec, h.Key, err)
		}
	}
	// Beyond the portable F1-F20 subset the parser must reject early.
	for _, k := range []string{"F21", "F24"} {
		if _, err := ihotkey.Parse("Ctrl+" + k); err == nil {
			t.Errorf("parser should reject Ctrl+%s", k)
		}
	}
}
