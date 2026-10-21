package backend

import (
	"strings"
	"testing"
)

func TestRedactReplace(t *testing.T) {
	app := New()
	res, err := app.Redact(RedactRequest{Text: "db 192.168.1.10 user bob@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res.Text, "192.168.1.10") || !strings.Contains(res.Text, "[IP_1]") {
		t.Fatalf("not redacted: %q", res.Text)
	}
	if res.Replaces != 2 {
		t.Fatalf("want 2 replaces, got %d (%s)", res.Replaces, res.Stats)
	}
}

func TestRedactDisabled(t *testing.T) {
	app := New()
	res, err := app.Redact(RedactRequest{Text: "db 192.168.1.10 user bob@example.com", Disabled: []string{"IP"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Text, "192.168.1.10") || strings.Contains(res.Text, "[IP_") {
		t.Fatalf("IP should be disabled: %q", res.Text)
	}
	if !strings.Contains(res.Text, "[EMAIL_1]") {
		t.Fatalf("EMAIL should still redact: %q", res.Text)
	}
}

func TestRedactBadMode(t *testing.T) {
	app := New()
	if _, err := app.Redact(RedactRequest{Text: "x", Mode: "nope"}); err == nil {
		t.Fatal("want error for unknown mode")
	}
}

func TestListRules(t *testing.T) {
	if got := len(New().ListRules()); got == 0 {
		t.Fatal("no rules listed")
	}
}
