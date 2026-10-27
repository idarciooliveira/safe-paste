package hotkey

import "testing"

func TestParseCanonical(t *testing.T) {
	h, err := Parse("Ctrl+Shift+V")
	if err != nil {
		t.Fatal(err)
	}
	if h.String() != "Ctrl+Shift+V" {
		t.Fatalf("got %q", h.String())
	}
}

func TestParseAliases(t *testing.T) {
	for in, want := range map[string]string{
		"ctrl + shift + v": "Ctrl+Shift+V",
		"Cmd-Shift-V":      "Shift+Super+V",
		"control-alt-f5":   "Ctrl+Alt+F5",
		"Win+Space":        "Super+Space",
		"option-shift-esc": "Alt+Shift+Esc",
	} {
		h, err := Parse(in)
		if err != nil {
			t.Errorf("%q: %v", in, err)
			continue
		}
		if h.String() != want {
			t.Errorf("%q = %q, want %q", in, h.String(), want)
		}
	}
}

func TestParseRejects(t *testing.T) {
	for _, in := range []string{"", "V", "Ctrl", "Ctrl+Shift", "Ctrl+V+X", "Ctrl+Nope", "Shift++"} {
		if _, err := Parse(in); err == nil {
			t.Errorf("%q should fail", in)
		}
	}
}

func TestParseFunctionKeysPortableSubset(t *testing.T) {
	// F1-F20 is the portable subset all OS backends support.
	for _, k := range []string{"F1", "F12", "F20"} {
		if _, err := Parse("Ctrl+" + k); err != nil {
			t.Errorf("Ctrl+%s should parse: %v", k, err)
		}
	}
	// F21+ has no common backend (notably macOS caps at F20): reject
	// early instead of bricking startup after a restart.
	for _, k := range []string{"F21", "F24", "F25"} {
		if _, err := Parse("Ctrl+" + k); err == nil {
			t.Errorf("Ctrl+%s should fail (beyond F20)", k)
		}
	}
}

func TestDisplay(t *testing.T) {
	h, _ := Parse("Cmd+Shift+V")
	if h.Display(true) != "Shift+Cmd+V" {
		t.Fatalf("mac display: %q", h.Display(true))
	}
	if h.Display(false) != "Shift+Win+V" {
		t.Fatalf("other display: %q", h.Display(false))
	}
}

func TestNormalize(t *testing.T) {
	s, err := Normalize("shift+ctrl+v")
	if err != nil || s != "Ctrl+Shift+V" {
		t.Fatalf("got %q, %v", s, err)
	}
}
